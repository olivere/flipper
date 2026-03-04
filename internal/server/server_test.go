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

	// Override device data path
	dataDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", dataDir)
	t.Cleanup(func() { os.Unsetenv("XDG_DATA_HOME") })

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

	registry, err := device.NewRegistry(cfg.Server.SecretKey)
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

	var setupResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&setupResp); err != nil {
		t.Fatal(err)
	}
	apiKey := setupResp["api_key"]
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

	// Verify refresh_rate is a string
	refreshRate, ok := displayResp["refresh_rate"].(string)
	if !ok {
		t.Fatal("display: refresh_rate should be a string")
	}
	if refreshRate != "900" {
		t.Errorf("display: expected refresh_rate 900, got %s", refreshRate)
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

	// Step 4: Fetch display again — same filename (no content change)
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
