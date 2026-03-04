package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/screen"
)

type displayResponse struct {
	Status          int     `json:"status"`
	ImageURL        string  `json:"image_url"`
	Filename        string  `json:"filename"`
	UpdateFirmware  bool    `json:"update_firmware"`
	FirmwareURL     *string `json:"firmware_url"`
	RefreshRate     string  `json:"refresh_rate"`
	ResetFirmware   bool    `json:"reset_firmware"`
}

func (h *Handler) Display(w http.ResponseWriter, r *http.Request) {
	mac := r.Header.Get("ID")
	token := r.Header.Get("Access-Token")

	if mac == "" || token == "" {
		http.Error(w, "missing ID or Access-Token header", http.StatusBadRequest)
		return
	}

	if !h.Devices.Authenticate(mac, token) {
		// Device may be sending a token from a previous server.
		// Only adopt tokens while setup_mode is active (migration window).
		if !h.Config.Server.SetupMode || !h.Devices.AdoptToken(mac, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	h.Devices.Touch(mac, device.Telemetry{
		FirmwareVersion: r.Header.Get("FW-Version"),
		BatteryVoltage:  r.Header.Get("Battery-Voltage"),
		WifiRSSI:        r.Header.Get("RSSI"),
		Model:           r.Header.Get("Model"),
	})

	// Detect device profile from headers
	width := headerInt(r, "WIDTH", h.Config.Device.Width)
	height := headerInt(r, "HEIGHT", h.Config.Device.Height)
	profile := display.DetectProfile(width, height)

	// Get current screen (advance if rotation is enabled)
	var scr screen.Screen
	if h.Config.Screens.Rotate {
		scr = h.Screens.Next()
	} else {
		scr = h.Screens.Current()
	}
	if scr == nil {
		http.Error(w, "no screens configured", http.StatusServiceUnavailable)
		return
	}

	// Render
	opts := screen.RenderOpts{
		Width:   profile.Width,
		Height:  profile.Height,
		Scaling: "fit",
	}
	img, err := scr.Render(r.Context(), opts)
	if err != nil {
		h.Logger.Error("render screen", "screen", scr.Name(), "err", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}

	// Process through pipeline
	result, err := h.Pipeline.Process(img, profile, opts.Scaling)
	if err != nil {
		h.Logger.Error("process image", "err", err)
		http.Error(w, "processing failed", http.StatusInternalServerError)
		return
	}

	h.Cache.Set(result)

	writeDisplayResponse(w, r, result, h.Config.Device.RefreshRate)
}

func writeDisplayResponse(w http.ResponseWriter, r *http.Request, result *display.Result, refreshRate int) {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host

	resp := displayResponse{
		Status:      0,
		ImageURL:    fmt.Sprintf("%s://%s/images/%s", scheme, host, result.Filename),
		Filename:    result.Filename,
		RefreshRate: strconv.Itoa(refreshRate),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func headerInt(r *http.Request, key string, fallback int) int {
	v := r.Header.Get(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
