package handler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/firmware"
	"github.com/olivere/flipper/internal/screen"
)

// firmwareFixture wires up the minimal Handler needed for the
// firmware-dispatch and binary-serving tests. Each test gets its own
// XDG-rooted tmpdir so devices.json / manifest.json / pending.json
// stay isolated.
type firmwareFixture struct {
	t        *testing.T
	dir      string
	devices  *device.Registry
	store    *firmware.Store
	pending  *firmware.Pending
	handler  *Handler
	apiKey   string
	mac      string
	model    string
	binary   firmware.Binary
	binBytes []byte
}

func newFirmwareFixture(t *testing.T) *firmwareFixture {
	t.Helper()
	dir := t.TempDir()
	devReg, err := device.NewRegistry("secret", device.WithPath(filepath.Join(dir, "devices.json")))
	if err != nil {
		t.Fatal(err)
	}
	d, err := devReg.Register("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatal(err)
	}
	devReg.Touch(d.MAC, device.Telemetry{FirmwareVersion: "1.7.4", Model: "TRMNL_X"})

	store, err := firmware.NewStore(firmware.WithStoreDir(filepath.Join(dir, "firmware")))
	if err != nil {
		t.Fatal(err)
	}
	binSrc := filepath.Join(t.TempDir(), "fw.bin")
	binBytes := []byte("firmware-bytes-XYZ")
	if err := os.WriteFile(binSrc, binBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	bin, err := store.Import(context.Background(), binSrc, "1.8.2", "TRMNL_X")
	if err != nil {
		t.Fatal(err)
	}

	pending, err := firmware.NewPending(firmware.WithPendingPath(filepath.Join(dir, "pending.json")))
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Server.SecretKey = "secret"
	cfg.Device.Width = 800
	cfg.Device.Height = 480
	cfg.Device.RefreshRate = 900

	h := &Handler{
		Config:   cfg,
		Devices:  devReg,
		Screens:  screen.NewRegistry(),
		Pipeline: display.NewPipeline(),
		Cache:    NewImageCache(),
		Firmware: &Firmware{Store: store, Pending: pending},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	return &firmwareFixture{
		t:        t,
		dir:      dir,
		devices:  devReg,
		store:    store,
		pending:  pending,
		handler:  h,
		apiKey:   d.APIKey,
		mac:      d.MAC,
		model:    "TRMNL_X",
		binary:   bin,
		binBytes: binBytes,
	}
}

func TestDisplayDispatchesFirmwareWhenArmed(t *testing.T) {
	fx := newFirmwareFixture(t)
	arm := firmware.Arm{
		Filename: fx.binary.Filename,
		Version:  fx.binary.Version,
		Model:    fx.binary.Model,
		ArmedAt:  time.Now().UTC(),
	}
	if err := fx.pending.Arm(fx.mac, arm); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/display", nil)
	req.Header.Set("ID", fx.mac)
	req.Header.Set("Access-Token", fx.apiKey)
	req.Header.Set("Model", fx.model)
	req.Host = "flipper.local:3443"
	rr := httptest.NewRecorder()

	fx.handler.Display(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Status         int     `json:"status"`
		ImageURL       string  `json:"image_url"`
		Filename       string  `json:"filename"`
		UpdateFirmware bool    `json:"update_firmware"`
		FirmwareURL    *string `json:"firmware_url"`
		RefreshRate    int     `json:"refresh_rate"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v\n%s", err, rr.Body.String())
	}
	if !resp.UpdateFirmware {
		t.Errorf("update_firmware = false, want true; body=%s", rr.Body.String())
	}
	if resp.FirmwareURL == nil || *resp.FirmwareURL == "" {
		t.Errorf("firmware_url should be a non-empty URL, got %v", resp.FirmwareURL)
	}
	expected := "http://flipper.local:3443/firmware/" + fx.binary.Filename
	if resp.FirmwareURL != nil && *resp.FirmwareURL != expected {
		t.Errorf("firmware_url = %q, want %q", *resp.FirmwareURL, expected)
	}
	if resp.ImageURL != "" || resp.Filename != "" {
		t.Errorf("image_url/filename must be empty on firmware dispatch, got %+v", resp)
	}

	// Arm must be consumed — a second call returns the normal display
	// flow (which here errors with no-screen, proving we no longer
	// dispatch firmware).
	rr2 := httptest.NewRecorder()
	fx.handler.Display(rr2, req)
	if rr2.Code == http.StatusOK {
		var second struct {
			UpdateFirmware bool `json:"update_firmware"`
		}
		_ = json.Unmarshal(rr2.Body.Bytes(), &second)
		if second.UpdateFirmware {
			t.Errorf("second Display call still dispatched firmware — Take is not one-shot")
		}
	}
}

func TestDisplaySuppressesDispatchOnModelMismatch(t *testing.T) {
	fx := newFirmwareFixture(t)
	arm := firmware.Arm{
		Filename: fx.binary.Filename,
		Version:  fx.binary.Version,
		Model:    "TRMNL_X", // armed for X
		ArmedAt:  time.Now().UTC(),
	}
	if err := fx.pending.Arm(fx.mac, arm); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/display", nil)
	req.Header.Set("ID", fx.mac)
	req.Header.Set("Access-Token", fx.apiKey)
	req.Header.Set("Model", "og") // device actually reports OG
	rr := httptest.NewRecorder()
	fx.handler.Display(rr, req)

	// Dispatch must not happen — but the arm is consumed (dropped) so
	// the loop can't keep dispatching wrong firmware. The fall-through
	// will hit the no-screens-configured path because the fixture has
	// no screens. That's fine; we only care that update_firmware is
	// not in the response.
	if rr.Code == http.StatusOK {
		var resp struct {
			UpdateFirmware bool `json:"update_firmware"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp.UpdateFirmware {
			t.Errorf("dispatch should be suppressed on model mismatch")
		}
	}
	if _, ok := fx.pending.Take(fx.mac); ok {
		t.Errorf("arm should be consumed even on mismatch (no auto-retry loop)")
	}
}

func TestDisplayFallsThroughWhenNotArmed(t *testing.T) {
	fx := newFirmwareFixture(t)
	req := httptest.NewRequest(http.MethodGet, "/api/display", nil)
	req.Header.Set("ID", fx.mac)
	req.Header.Set("Access-Token", fx.apiKey)
	req.Header.Set("Model", fx.model)
	rr := httptest.NewRecorder()
	fx.handler.Display(rr, req)
	// No firmware arm, no screens configured → 503. The important
	// invariant is that we did NOT emit a firmware dispatch.
	if rr.Code == http.StatusOK {
		var resp struct {
			UpdateFirmware bool `json:"update_firmware"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp.UpdateFirmware {
			t.Errorf("no arm yet update_firmware=true: %s", rr.Body.String())
		}
	}
}

func TestServeFirmwareDoesNotRequireAuth(t *testing.T) {
	// TRMNL OG firmware 1.7.4 does NOT send ID / Access-Token headers
	// when fetching firmware_url, so requiring them blocks every real
	// OTA. The real security boundary is /api/display (which decides
	// which binary gets dispatched to which MAC); this endpoint only
	// serves bytes whose filenames are already in our store.
	fx := newFirmwareFixture(t)
	r := chi.NewRouter()
	r.Get("/firmware/{filename}", fx.handler.ServeFirmware)

	req := httptest.NewRequest(http.MethodGet, "/firmware/"+fx.binary.Filename, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("no-auth fetch: status = %d, want 200", rr.Code)
	}
	if rr.Body.String() != string(fx.binBytes) {
		t.Errorf("no-auth fetch returned wrong bytes")
	}
}

func TestServeFirmwareReturnsBinaryWithContentLength(t *testing.T) {
	fx := newFirmwareFixture(t)
	r := chi.NewRouter()
	r.Get("/firmware/{filename}", fx.handler.ServeFirmware)

	req := httptest.NewRequest(http.MethodGet, "/firmware/"+fx.binary.Filename, nil)
	req.Header.Set("ID", fx.mac)
	req.Header.Set("Access-Token", fx.apiKey)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	cl := rr.Header().Get("Content-Length")
	if cl == "" {
		t.Errorf("Content-Length header missing (TRMNL bootloader needs it; chunked is not supported)")
	} else if n, _ := strconv.Atoi(cl); n != len(fx.binBytes) {
		t.Errorf("Content-Length = %s, want %d", cl, len(fx.binBytes))
	}
	if rr.Header().Get("Content-Type") != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", rr.Header().Get("Content-Type"))
	}
	if rr.Body.String() != string(fx.binBytes) {
		t.Errorf("body mismatch: got %q want %q", rr.Body.String(), fx.binBytes)
	}
}

func TestServeFirmwareUnknownFile(t *testing.T) {
	fx := newFirmwareFixture(t)
	r := chi.NewRouter()
	r.Get("/firmware/{filename}", fx.handler.ServeFirmware)
	req := httptest.NewRequest(http.MethodGet, "/firmware/does-not-exist.bin", nil)
	req.Header.Set("ID", fx.mac)
	req.Header.Set("Access-Token", fx.apiKey)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}
