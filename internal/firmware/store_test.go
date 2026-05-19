package firmware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreImportFromPath(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "fw.bin")
	if err := os.WriteFile(srcPath, []byte("firmware-bytes-1"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewStore(WithStoreDir(dir))
	if err != nil {
		t.Fatal(err)
	}

	bin, err := s.Import(context.Background(), srcPath, "1.8.2", "TRMNL_X")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if bin.Filename != "FW-1.8.2.TRMNL_X.bin" {
		t.Errorf("Filename = %q, want FW-1.8.2.TRMNL_X.bin", bin.Filename)
	}
	if bin.Version != "1.8.2" || bin.Model != "TRMNL_X" {
		t.Errorf("unexpected version/model: %+v", bin)
	}
	if bin.Size != int64(len("firmware-bytes-1")) {
		t.Errorf("Size = %d, want %d", bin.Size, len("firmware-bytes-1"))
	}
	if bin.SHA256 == "" || len(bin.SHA256) != 64 {
		t.Errorf("SHA256 looks wrong: %q", bin.SHA256)
	}

	if _, err := os.Stat(filepath.Join(dir, bin.Filename)); err != nil {
		t.Errorf("file should exist on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Errorf("manifest should exist: %v", err)
	}
}

func TestStoreImportFromURL(t *testing.T) {
	dir := t.TempDir()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("over-the-wire-bytes"))
	}))
	t.Cleanup(srv.Close)

	s, err := NewStore(WithStoreDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := s.Import(context.Background(), srv.URL+"/fw.bin", "v2.0.0", "og")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if bin.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0 (leading v stripped)", bin.Version)
	}
	if bin.Source != srv.URL+"/fw.bin" {
		t.Errorf("Source = %q, want URL", bin.Source)
	}
}

func TestStoreImportIdempotentSameBytes(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "fw.bin")
	if err := os.WriteFile(srcPath, []byte("identical"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewStore(WithStoreDir(dir))

	first, err := s.Import(context.Background(), srcPath, "1.0.0", "og")
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	second, err := s.Import(context.Background(), srcPath, "1.0.0", "og")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if first.SHA256 != second.SHA256 || first.Filename != second.Filename {
		t.Errorf("re-import returned different binary: %+v vs %+v", first, second)
	}
	bins := s.List()
	if len(bins) != 1 {
		t.Errorf("expected exactly one manifest entry after idempotent re-import, got %d", len(bins))
	}
}

func TestStoreImportRefusesDifferentBytes(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "fw.bin")
	if err := os.WriteFile(srcPath, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewStore(WithStoreDir(dir))
	if _, err := s.Import(context.Background(), srcPath, "1.0.0", "og"); err != nil {
		t.Fatalf("first import: %v", err)
	}

	// Now overwrite the source path with different bytes and re-import.
	if err := os.WriteFile(srcPath, []byte("tampered-different-size"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := s.Import(context.Background(), srcPath, "1.0.0", "og")
	if err == nil {
		t.Fatal("expected refusal on conflicting re-import, got nil")
	}
	if !strings.Contains(err.Error(), "different bytes") {
		t.Errorf("error should mention 'different bytes', got: %v", err)
	}
}

func TestStoreImportRejectsZeroBytes(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "empty.bin")
	if err := os.WriteFile(srcPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewStore(WithStoreDir(dir))
	if _, err := s.Import(context.Background(), srcPath, "1.0.0", "og"); err == nil {
		t.Fatal("expected error on zero-byte source")
	}
}

func TestStoreImportRequiresVersionAndModel(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "fw.bin")
	if err := os.WriteFile(srcPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := NewStore(WithStoreDir(dir))
	if _, err := s.Import(context.Background(), srcPath, "", "og"); err == nil {
		t.Error("expected error when version missing")
	}
	if _, err := s.Import(context.Background(), srcPath, "1.0.0", ""); err == nil {
		t.Error("expected error when model missing")
	}
}

func TestStoreFindAndFindByFilename(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "fw.bin")
	_ = os.WriteFile(srcPath, []byte("bytes"), 0o644)
	s, _ := NewStore(WithStoreDir(dir))
	bin, err := s.Import(context.Background(), srcPath, "1.8.2", "TRMNL_X")
	if err != nil {
		t.Fatal(err)
	}

	got, ok := s.Find("1.8.2", "TRMNL_X")
	if !ok || got.Filename != bin.Filename {
		t.Errorf("Find returned %+v, ok=%v", got, ok)
	}
	got, ok = s.Find("v1.8.2", "TRMNL_X") // leading v tolerated
	if !ok {
		t.Error("Find should tolerate leading v")
	}
	if _, ok := s.Find("1.8.2", "og"); ok {
		t.Error("Find for wrong model must miss")
	}
	got, ok = s.FindByFilename(bin.Filename)
	if !ok || got.SHA256 != bin.SHA256 {
		t.Errorf("FindByFilename returned %+v ok=%v", got, ok)
	}
}

func TestStoreRemoveClearsFileAndManifest(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(t.TempDir(), "fw.bin")
	_ = os.WriteFile(srcPath, []byte("bytes"), 0o644)
	s, _ := NewStore(WithStoreDir(dir))
	bin, err := s.Import(context.Background(), srcPath, "1.0.0", "og")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Remove("1.0.0", "og"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, bin.Filename)); !os.IsNotExist(err) {
		t.Errorf("binary file should be gone, got err=%v", err)
	}
	if len(s.List()) != 0 {
		t.Errorf("manifest should be empty after remove, got %+v", s.List())
	}
	if err := s.Remove("1.0.0", "og"); err == nil {
		t.Error("expected ErrNotExist on second remove")
	}
}

func TestStoreOpenRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewStore(WithStoreDir(dir))
	bads := []string{"", "../etc/passwd", "sub/dir", `windows\path`, ".hidden", ".."}
	for _, name := range bads {
		if _, _, err := s.Open(name); err == nil {
			t.Errorf("Open(%q) should be rejected", name)
		}
	}
}
