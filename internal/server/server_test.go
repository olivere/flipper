package server_test

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/handler"
	"github.com/olivere/flipper/internal/screen"
	"github.com/olivere/flipper/internal/screen/static"
)

func setupTestServer(t *testing.T) (*httptest.Server, *config.Config) {
	t.Helper()

	// Create temp dir with a test image
	dir := t.TempDir()
	createTestImage(t, filepath.Join(dir, "test.png"), 100, 100)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Addr:      ":0",
			SecretKey: "test-secret",
			SetupMode: true,
		},
		Device: config.DeviceConfig{
			Width:       800,
			Height:      480,
			Format:      "bmp",
			RefreshRate: 900,
		},
		Screens: config.ScreensConfig{
			Static: config.StaticScreenConfig{Dir: dir},
		},
	}

	devicesPath := filepath.Join(t.TempDir(), "devices.json")
	registry, err := device.NewRegistry(cfg.Server.SecretKey, device.WithPath(devicesPath))
	if err != nil {
		t.Fatal(err)
	}

	pipeline := display.NewPipeline()
	cache := handler.NewImageCache()
	screens := screen.NewRegistry()

	s, err := static.New(cfg.Screens.Static.Dir)
	if err != nil {
		t.Fatal(err)
	}
	screens.Add(s)

	h := &handler.Handler{
		Config:   cfg,
		Devices:  registry,
		Screens:  screens,
		Pipeline: pipeline,
		Cache:    cache,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	r := chi.NewRouter()
	r.Get("/api/setup", h.Setup)
	r.Get("/api/display", h.Display)
	r.Post("/api/log", h.Log)
	r.Get("/images/{filename}", h.ServeImage)

	return httptest.NewServer(r), cfg
}

func createTestImage(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.Gray{Y: uint8((x + y) % 256)})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration(t *testing.T) {
	ts, _ := setupTestServer(t)
	defer ts.Close()

	mac := "AA:BB:CC:DD:EE:FF"

	// Step 1: Setup (provision device)
	req, _ := http.NewRequest("GET", ts.URL+"/api/setup", nil)
	req.Header.Set("ID", mac)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup: expected 200, got %d", resp.StatusCode)
	}

	// The TRMNL firmware parses `status` as int with ArduinoJson and
	// bails out before extracting api_key when status != 200. Make
	// sure we decode into typed fields so the test catches any future
	// regression to a string status.
	var setupResp struct {
		Status     int    `json:"status"`
		APIKey     string `json:"api_key"`
		FriendlyID string `json:"friendly_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&setupResp); err != nil {
		t.Fatal(err)
	}
	if setupResp.Status != 200 {
		t.Fatalf("setup: expected status 200 (int), got %d", setupResp.Status)
	}
	apiKey := setupResp.APIKey
	if apiKey == "" {
		t.Fatal("setup: expected non-empty api_key")
	}

	// Step 2: Fetch display
	req, _ = http.NewRequest("GET", ts.URL+"/api/display", nil)
	req.Header.Set("ID", mac)
	req.Header.Set("Access-Token", apiKey)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("display: expected 200, got %d: %s", resp.StatusCode, body)
	}

	var displayResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&displayResp); err != nil {
		t.Fatal(err)
	}

	imageURL, ok := displayResp["image_url"].(string)
	if !ok || imageURL == "" {
		t.Fatal("display: expected non-empty image_url")
	}
	filename := displayResp["filename"].(string)
	if filename == "" {
		t.Fatal("display: expected non-empty filename")
	}

	// Verify refresh_rate is a JSON number, not a string. TRMNL
	// firmware ≥ 1.8.2 parses this field as uint64 with no
	// string-to-int fallback, so emitting it as a string yields 0 on
	// the device and triggers a ~10s polling spin. encoding/json
	// decodes JSON numbers into float64 when the target is any.
	refreshRate, ok := displayResp["refresh_rate"].(float64)
	if !ok {
		t.Fatalf("display: refresh_rate should be a JSON number, got %T: %v",
			displayResp["refresh_rate"], displayResp["refresh_rate"])
	}
	if refreshRate != 900 {
		t.Errorf("display: expected refresh_rate 900, got %v", refreshRate)
	}

	// Step 3: Download the image
	resp, err = http.Get(imageURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("image download: expected 200, got %d", resp.StatusCode)
	}

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// Verify BMP magic bytes
	if imgData[0] != 'B' || imgData[1] != 'M' {
		t.Errorf("expected BMP magic bytes, got %x %x", imgData[0], imgData[1])
	}

	// Step 4: Fetch display again — still renders (no caching), but with
	// only one image the static screen returns the same file each time.
	req, _ = http.NewRequest("GET", ts.URL+"/api/display", nil)
	req.Header.Set("ID", mac)
	req.Header.Set("Access-Token", apiKey)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var displayResp2 map[string]any
	json.NewDecoder(resp.Body).Decode(&displayResp2)
	if displayResp2["filename"] != filename {
		t.Errorf("expected same filename %s, got %s", filename, displayResp2["filename"])
	}
}

func TestSlideshowRotation(t *testing.T) {
	// With multiple images, each /api/display call should return a different
	// image because the static screen rotates on every Render() and the
	// display handler no longer caches by key.
	ts, cfg := setupTestServer(t)
	defer ts.Close()

	// Add a second image to the static dir (the screen re-scans on each Render).
	createTestImage(t, filepath.Join(cfg.Screens.Static.Dir, "a.png"), 200, 100)

	mac := "AA:BB:CC:DD:EE:01"

	// Provision
	req, _ := http.NewRequest("GET", ts.URL+"/api/setup", nil)
	req.Header.Set("ID", mac)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var setupResp struct {
		Status     int    `json:"status"`
		APIKey     string `json:"api_key"`
		FriendlyID string `json:"friendly_id"`
	}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	resp.Body.Close()
	apiKey := setupResp.APIKey

	// First display call
	fn1 := fetchDisplayFilename(t, ts.URL, mac, apiKey)
	// Second display call — should be a different image
	fn2 := fetchDisplayFilename(t, ts.URL, mac, apiKey)

	if fn1 == fn2 {
		t.Errorf("expected different filenames on consecutive calls, got %s both times", fn1)
	}

	// Third call should cycle back to the first image
	fn3 := fetchDisplayFilename(t, ts.URL, mac, apiKey)
	if fn3 != fn1 {
		t.Errorf("expected rotation to cycle back: got %s, want %s", fn3, fn1)
	}
}

func fetchDisplayFilename(t *testing.T, baseURL, mac, apiKey string) string {
	t.Helper()
	req, _ := http.NewRequest("GET", baseURL+"/api/display", nil)
	req.Header.Set("ID", mac)
	req.Header.Set("Access-Token", apiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("display: expected 200, got %d: %s", resp.StatusCode, body)
	}
	var dr map[string]any
	json.NewDecoder(resp.Body).Decode(&dr)
	fn, _ := dr["filename"].(string)
	if fn == "" {
		t.Fatal("display: expected non-empty filename")
	}
	return fn
}

func TestSetupModeDisabled(t *testing.T) {
	ts, cfg := setupTestServer(t)
	defer ts.Close()

	cfg.Server.SetupMode = false

	req, _ := http.NewRequest("GET", ts.URL+"/api/setup", nil)
	req.Header.Set("ID", "AA:BB:CC:DD:EE:FF")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestDisplayUnauthorized(t *testing.T) {
	// Unregistered device should get 401.
	ts, _ := setupTestServer(t)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/api/display", nil)
	req.Header.Set("ID", "AA:BB:CC:DD:EE:FF")
	req.Header.Set("Access-Token", "wrong-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestDisplayTokenAdoption(t *testing.T) {
	// A registered device sending a different token should be accepted
	// while setup_mode is enabled (migration window).
	ts, _ := setupTestServer(t)
	defer ts.Close()

	mac := "AA:BB:CC:DD:EE:FF"

	// Register the device first.
	req, _ := http.NewRequest("GET", ts.URL+"/api/setup", nil)
	req.Header.Set("ID", mac)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Use a different token (simulating a device migrating from another server).
	req, _ = http.NewRequest("GET", ts.URL+"/api/display", nil)
	req.Header.Set("ID", mac)
	req.Header.Set("Access-Token", "foreign-server-key")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (token adopted), got %d", resp.StatusCode)
	}
}

func TestDisplayTokenAdoptionDisabledOutsideSetupMode(t *testing.T) {
	// Once setup_mode is off, token adoption should not happen.
	ts, cfg := setupTestServer(t)
	defer ts.Close()

	mac := "AA:BB:CC:DD:EE:FF"

	// Register the device while setup_mode is on.
	req, _ := http.NewRequest("GET", ts.URL+"/api/setup", nil)
	req.Header.Set("ID", mac)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Disable setup_mode.
	cfg.Server.SetupMode = false

	// A different token should now be rejected.
	req, _ = http.NewRequest("GET", ts.URL+"/api/display", nil)
	req.Header.Set("ID", mac)
	req.Header.Set("Access-Token", "foreign-server-key")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 (adoption disabled), got %d", resp.StatusCode)
	}
}
