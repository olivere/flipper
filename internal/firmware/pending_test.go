package firmware

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPendingArmTakeIsOneShot(t *testing.T) {
	p := newTestPending(t)
	mac := "AA:BB:CC:DD:EE:FF"
	arm := Arm{Filename: "FW-1.8.2.TRMNL_X.bin", Version: "1.8.2", Model: "TRMNL_X", ArmedAt: time.Now().UTC()}
	if err := p.Arm(mac, arm); err != nil {
		t.Fatalf("arm: %v", err)
	}

	got, ok := p.Take(mac)
	if !ok {
		t.Fatal("first Take should succeed")
	}
	if got.Filename != arm.Filename || got.Version != arm.Version || got.Model != arm.Model {
		t.Errorf("Take returned wrong arm: %+v", got)
	}

	if _, ok := p.Take(mac); ok {
		t.Fatal("second Take must return ok=false (one-shot)")
	}
}

func TestPendingMACNormalization(t *testing.T) {
	p := newTestPending(t)
	arm := Arm{Filename: "x", Version: "1.0.0", Model: "og"}
	if err := p.Arm("aa:bb:cc:dd:ee:ff", arm); err != nil {
		t.Fatalf("arm: %v", err)
	}
	if _, ok := p.Take("AA:BB:CC:DD:EE:FF"); !ok {
		t.Error("Take must normalise MAC case so lowercase arm matches uppercase Take")
	}
}

func TestPendingDisarm(t *testing.T) {
	p := newTestPending(t)
	mac := "AA:BB:CC:DD:EE:FF"
	if p.Disarm(mac) {
		t.Error("Disarm on empty must return false")
	}
	if err := p.Arm(mac, Arm{Filename: "f", Version: "1", Model: "m"}); err != nil {
		t.Fatal(err)
	}
	if !p.Disarm(mac) {
		t.Error("Disarm should report true when an arm existed")
	}
	if _, ok := p.Take(mac); ok {
		t.Error("Disarm should clear the arm")
	}
}

func TestPendingPersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending.json")
	p1, err := NewPending(WithPendingPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := p1.Arm("AA:BB:CC:DD:EE:FF", Arm{Filename: "F", Version: "1.8.2", Model: "TRMNL_X"}); err != nil {
		t.Fatal(err)
	}
	// Fresh instance pointed at the same file must see the arm.
	p2, err := NewPending(WithPendingPath(path))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := p2.Take("AA:BB:CC:DD:EE:FF")
	if !ok {
		t.Fatal("second instance should see persisted arm")
	}
	if got.Version != "1.8.2" {
		t.Errorf("Version = %q, want 1.8.2", got.Version)
	}
}

func TestPendingList(t *testing.T) {
	p := newTestPending(t)
	_ = p.Arm("AA:BB:CC:DD:EE:01", Arm{Filename: "a", Version: "1.0.0", Model: "og"})
	_ = p.Arm("AA:BB:CC:DD:EE:02", Arm{Filename: "b", Version: "2.0.0", Model: "TRMNL_X"})
	got := p.List()
	if len(got) != 2 {
		t.Fatalf("List len = %d, want 2", len(got))
	}
	if got["AA:BB:CC:DD:EE:01"].Filename != "a" || got["AA:BB:CC:DD:EE:02"].Filename != "b" {
		t.Errorf("unexpected List payload: %+v", got)
	}
}

func TestPendingArmReplacesExisting(t *testing.T) {
	p := newTestPending(t)
	mac := "AA:BB:CC:DD:EE:FF"
	_ = p.Arm(mac, Arm{Filename: "old", Version: "1.0.0", Model: "og"})
	_ = p.Arm(mac, Arm{Filename: "new", Version: "2.0.0", Model: "og"})
	got, _ := p.Take(mac)
	if got.Filename != "new" {
		t.Errorf("Filename = %q, want 'new' (re-arm should replace)", got.Filename)
	}
}

func newTestPending(t *testing.T) *Pending {
	t.Helper()
	p, err := NewPending(WithPendingPath(filepath.Join(t.TempDir(), "pending.json")))
	if err != nil {
		t.Fatal(err)
	}
	return p
}
