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
	Status         int     `json:"status"`
	ImageURL       string  `json:"image_url"`
	Filename       string  `json:"filename"`
	UpdateFirmware bool    `json:"update_firmware"`
	FirmwareURL    *string `json:"firmware_url"`
	RefreshRate    string  `json:"refresh_rate"`
	ResetFirmware  bool    `json:"reset_firmware"`
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

	// Firmware OTA dispatch (one-shot). If an arm is present, consume
	// it now and return a firmware-dispatch response instead of
	// rendering a screen. The arm has already been cleared by Take, so
	// even if the device fails to flash, we will not auto-retry —
	// the operator must re-arm explicitly.
	if h.tryDispatchFirmware(w, r, mac) {
		return
	}

	// Detect device profile from headers
	width := headerInt(r, "WIDTH", h.Config.Device.Width)
	height := headerInt(r, "HEIGHT", h.Config.Device.Height)
	profile := display.DetectProfile(width, height)

	// Get current screen: playlist takes priority, then registry rotation.
	var scr screen.Screen
	refreshRate := h.Config.Device.RefreshRate

	if h.Playlist != nil && h.Playlist.Len() > 0 {
		s, dur := h.Playlist.Next()
		scr = s
		if dur > 0 {
			refreshRate = int(dur.Seconds())
		}
	} else if h.Config.Screens.Rotate {
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
		Width:     profile.Width,
		Height:    profile.Height,
		Scaling:   "fit",
		ColorMode: profile.ColorMode,
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

	writeDisplayResponse(w, r, result, refreshRate)
}

// tryDispatchFirmware checks for a pending OTA arm for mac and, if
// present, writes the firmware-dispatch response. Returns true when
// the response has been written (caller must return). When the arm's
// model does not match the device's reported Model header, the arm is
// dropped (already consumed by Take) and the function returns false
// so the normal display flow continues; the mismatch is logged so the
// operator notices.
func (h *Handler) tryDispatchFirmware(w http.ResponseWriter, r *http.Request, mac string) bool {
	if h.Firmware == nil || h.Firmware.Pending == nil || h.Firmware.Store == nil {
		return false
	}
	arm, ok := h.Firmware.Pending.Take(mac)
	if !ok {
		return false
	}

	deviceModel := r.Header.Get("Model")
	if arm.Model != "" && arm.Model != deviceModel {
		h.Logger.Error("firmware arm model mismatch — refusing to dispatch",
			"mac", mac,
			"want_model", arm.Model,
			"got_model", deviceModel,
			"version", arm.Version,
			"file", arm.Filename,
		)
		return false
	}

	if _, exists := h.Firmware.Store.FindByFilename(arm.Filename); !exists {
		h.Logger.Error("firmware arm references missing binary — refusing to dispatch",
			"mac", mac, "file", arm.Filename, "version", arm.Version,
		)
		return false
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/firmware/%s", scheme, r.Host, arm.Filename)

	writeFirmwareResponse(w, h.Config.Device.RefreshRate, url)
	h.Logger.Info("firmware update dispatched",
		"mac", mac,
		"version", arm.Version,
		"model", arm.Model,
		"file", arm.Filename,
		"url", url,
	)
	return true
}

func writeFirmwareResponse(w http.ResponseWriter, refreshRate int, firmwareURL string) {
	resp := displayResponse{
		Status:         0,
		UpdateFirmware: true,
		FirmwareURL:    &firmwareURL,
		RefreshRate:    strconv.Itoa(refreshRate),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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
