package firmware

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/olivere/flipper/internal/xdg"
)

// Arm records that the next /api/display poll from a specific device
// should dispatch a one-shot firmware OTA update.
type Arm struct {
	Filename string    `json:"filename"`
	Version  string    `json:"version"`
	Model    string    `json:"model"`
	ArmedAt  time.Time `json:"armed_at"`
}

// Pending coordinates one-shot firmware arms between the operator
// (CLI process) and the running server. The state is persisted to a
// single JSON file keyed by device MAC; every mutation acquires an
// exclusive flock on the file to keep concurrent CLI/server access
// from losing arms via a lost-update race.
type Pending struct {
	path string
}

// PendingOption configures a Pending.
type PendingOption func(*Pending)

// WithPendingPath overrides the default state file location
// ($XDG_DATA_HOME/flipper/firmware-pending.json).
func WithPendingPath(path string) PendingOption {
	return func(p *Pending) { p.path = path }
}

// NewPending returns a Pending tracker backed by a JSON file under
// $XDG_DATA_HOME/flipper (or the path supplied via WithPendingPath).
func NewPending(opts ...PendingOption) (*Pending, error) {
	p := &Pending{path: filepath.Join(xdg.DataHome(), "flipper", "firmware-pending.json")}
	for _, o := range opts {
		o(p)
	}
	return p, nil
}

// Path returns the state file path used by this Pending instance.
func (p *Pending) Path() string { return p.path }

// Arm records a pending update for mac. Re-arming the same MAC
// replaces the previous arm.
func (p *Pending) Arm(mac string, a Arm) error {
	mac = normMAC(mac)
	return p.update(func(m map[string]Arm) {
		m[mac] = a
	})
}

// Take returns the pending arm for mac and clears it atomically. The
// arm-and-clear happens under the same file lock, so a successful
// Take is the only way an arm gets consumed.
func (p *Pending) Take(mac string) (Arm, bool) {
	mac = normMAC(mac)
	var got Arm
	var ok bool
	_ = p.update(func(m map[string]Arm) {
		got, ok = m[mac]
		if ok {
			delete(m, mac)
		}
	})
	return got, ok
}

// Disarm removes a pending arm for mac and returns whether one was
// present.
func (p *Pending) Disarm(mac string) bool {
	mac = normMAC(mac)
	var existed bool
	_ = p.update(func(m map[string]Arm) {
		_, existed = m[mac]
		delete(m, mac)
	})
	return existed
}

// List returns all currently armed updates keyed by MAC.
func (p *Pending) List() map[string]Arm {
	f, err := p.openLocked()
	if err != nil {
		return map[string]Arm{}
	}
	defer p.closeLocked(f)
	m, _ := readPendingMap(f)
	return m
}

func (p *Pending) update(fn func(map[string]Arm)) error {
	f, err := p.openLocked()
	if err != nil {
		return err
	}
	defer p.closeLocked(f)

	m, err := readPendingMap(f)
	if err != nil {
		return err
	}
	fn(m)

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func (p *Pending) openLocked() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
		return nil, fmt.Errorf("create pending dir: %w", err)
	}
	f, err := os.OpenFile(p.path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open pending file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock pending file: %w", err)
	}
	return f, nil
}

func (p *Pending) closeLocked(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

func readPendingMap(f *os.File) (map[string]Arm, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	m := make(map[string]Arm)
	if len(data) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse pending: %w", err)
	}
	return m, nil
}

func normMAC(mac string) string {
	return strings.ToUpper(strings.TrimSpace(mac))
}
