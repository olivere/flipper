package firmware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/olivere/flipper/internal/xdg"
)

// Binary records a firmware .bin imported into the local store. The
// manifest stores one entry per file. The on-disk filename is derived
// from version and model so the bin name carries its own provenance.
type Binary struct {
	Filename   string    `json:"filename"`
	Version    string    `json:"version"`
	Model      string    `json:"model"`
	SHA256     string    `json:"sha256"`
	Source     string    `json:"source"`
	ImportedAt time.Time `json:"imported_at"`
	Size       int64     `json:"size"`
}

// Store manages firmware binaries on disk together with a manifest
// describing them. Both the CLI (which writes) and the server (which
// reads) instantiate their own Store and reconcile by re-reading the
// manifest on each call; mutations write the manifest atomically via
// a temp file + rename.
type Store struct {
	mu  sync.Mutex
	dir string
}

// StoreOption configures a Store.
type StoreOption func(*Store)

// WithStoreDir overrides the default firmware directory
// ($XDG_DATA_HOME/flipper/firmware).
func WithStoreDir(dir string) StoreOption {
	return func(s *Store) { s.dir = dir }
}

// NewStore returns a Store rooted at $XDG_DATA_HOME/flipper/firmware
// (or the directory given via WithStoreDir). The directory is created
// lazily on the first write.
func NewStore(opts ...StoreOption) (*Store, error) {
	s := &Store{dir: filepath.Join(xdg.DataHome(), "flipper", "firmware")}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// Dir returns the directory under which firmware binaries live.
func (s *Store) Dir() string { return s.dir }

func (s *Store) manifestPath() string {
	return filepath.Join(s.dir, "manifest.json")
}

// importClient downloads firmware binaries. Timeout is intentionally
// zero — large binaries on slow links can take a while; callers pass
// a context for cancellation.
var importClient = &http.Client{}

// Import copies src (a local filesystem path or http(s) URL) into the
// store, computes its sha256, and records it under version/model. The
// destination filename is derived as "FW-<version>.<model>.bin".
//
// Re-importing the same (version, model) is allowed only when the
// bytes match the existing copy. A different payload for the same
// (version, model) returns an error so the operator removes the prior
// binary before swapping it — silent overwrite of a previously armed
// build is exactly the kind of accident this package exists to avoid.
func (s *Store) Import(ctx context.Context, src, version, model string) (Binary, error) {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	model = strings.TrimSpace(model)
	if version == "" {
		return Binary{}, errors.New("version is required")
	}
	if model == "" {
		return Binary{}, errors.New("model is required")
	}
	if src == "" {
		return Binary{}, errors.New("source path or URL is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return Binary{}, fmt.Errorf("create firmware dir: %w", err)
	}

	filename := fmt.Sprintf("FW-%s.%s.bin", sanitizeFilenamePart(version), sanitizeFilenamePart(model))
	dst := filepath.Join(s.dir, filename)

	tmp, err := os.CreateTemp(s.dir, ".import-*.bin")
	if err != nil {
		return Binary{}, fmt.Errorf("open temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // no-op once renamed away

	rc, err := openSource(ctx, src)
	if err != nil {
		tmp.Close()
		return Binary{}, err
	}

	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(tmp, h), rc)
	rc.Close()
	if cerr := tmp.Close(); copyErr == nil {
		copyErr = cerr
	}
	if copyErr != nil {
		return Binary{}, fmt.Errorf("copy firmware bytes: %w", copyErr)
	}
	if n == 0 {
		return Binary{}, fmt.Errorf("source produced zero bytes: %s", src)
	}
	sum := hex.EncodeToString(h.Sum(nil))

	bins, err := s.loadLocked()
	if err != nil {
		return Binary{}, err
	}

	for _, b := range bins {
		if b.Version == version && b.Model == model {
			if b.SHA256 == sum {
				return b, nil
			}
			return Binary{}, fmt.Errorf(
				"firmware %s/%s already imported with different bytes (existing sha256 %s); remove it first",
				version, model, b.SHA256,
			)
		}
	}

	if err := os.Rename(tmpPath, dst); err != nil {
		return Binary{}, fmt.Errorf("rename firmware: %w", err)
	}

	bin := Binary{
		Filename:   filename,
		Version:    version,
		Model:      model,
		SHA256:     sum,
		Source:     src,
		ImportedAt: time.Now().UTC(),
		Size:       n,
	}
	bins = append(bins, bin)
	if err := s.saveLocked(bins); err != nil {
		_ = os.Remove(dst)
		return Binary{}, err
	}
	return bin, nil
}

// List returns all imported binaries from the manifest, or an empty
// slice if nothing has been imported yet.
func (s *Store) List() []Binary {
	s.mu.Lock()
	defer s.mu.Unlock()
	bins, _ := s.loadLocked()
	if bins == nil {
		return []Binary{}
	}
	return bins
}

// Find returns the binary matching (version, model). A leading "v" on
// version is tolerated so operators can paste either form.
func (s *Store) Find(version, model string) (Binary, bool) {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	model = strings.TrimSpace(model)
	s.mu.Lock()
	defer s.mu.Unlock()
	bins, _ := s.loadLocked()
	for _, b := range bins {
		if b.Version == version && b.Model == model {
			return b, true
		}
	}
	return Binary{}, false
}

// FindByFilename returns the binary with the given filename.
func (s *Store) FindByFilename(name string) (Binary, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	bins, _ := s.loadLocked()
	for _, b := range bins {
		if b.Filename == name {
			return b, true
		}
	}
	return Binary{}, false
}

// Open opens the binary file with the given filename and returns a
// reader plus the byte size. The caller must Close the reader. The
// filename is validated to prevent path traversal.
func (s *Store) Open(name string) (io.ReadCloser, int64, error) {
	if name == "" || strings.ContainsAny(name, `/\`) || name == ".." || strings.HasPrefix(name, ".") {
		return nil, 0, fmt.Errorf("invalid firmware filename: %q", name)
	}
	path := filepath.Join(s.dir, name)
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

// Remove deletes a binary from the store, both the file on disk and
// the manifest entry. Returns an error wrapping os.ErrNotExist if no
// such binary is recorded.
func (s *Store) Remove(version, model string) error {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	model = strings.TrimSpace(model)
	s.mu.Lock()
	defer s.mu.Unlock()

	bins, err := s.loadLocked()
	if err != nil {
		return err
	}
	out := make([]Binary, 0, len(bins))
	var found Binary
	var hit bool
	for _, b := range bins {
		if b.Version == version && b.Model == model {
			found = b
			hit = true
			continue
		}
		out = append(out, b)
	}
	if !hit {
		return fmt.Errorf("firmware %s/%s: %w", version, model, os.ErrNotExist)
	}
	if err := os.Remove(filepath.Join(s.dir, found.Filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove firmware file: %w", err)
	}
	return s.saveLocked(out)
}

func (s *Store) loadLocked() ([]Binary, error) {
	data, err := os.ReadFile(s.manifestPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("load firmware manifest: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var bins []Binary
	if err := json.Unmarshal(data, &bins); err != nil {
		return nil, fmt.Errorf("parse firmware manifest: %w", err)
	}
	return bins, nil
}

func (s *Store) saveLocked(bins []Binary) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create firmware dir: %w", err)
	}
	data, err := json.MarshalIndent(bins, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.manifestPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.manifestPath())
}

var safeFilenamePart = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func sanitizeFilenamePart(s string) string {
	return safeFilenamePart.ReplaceAllString(s, "_")
}

func openSource(ctx context.Context, src string) (io.ReadCloser, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		resp, err := importClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", src, err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("fetch %s: status %d", src, resp.StatusCode)
		}
		return resp.Body, nil
	}
	f, err := os.Open(src)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", src, err)
	}
	return f, nil
}
