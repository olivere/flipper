package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// setupResponse is the wire shape TRMNL firmware expects from
// /api/setup. Status MUST be an integer 200 — the firmware parser
// (lib/trmnl/src/parse_response_api_setup.cpp) does
// `doc["status"].as<int>()` and bails out before extracting api_key
// when status != 200. ArduinoJson v7 returns 0 for string-to-int, so
// emitting `"status": "ok"` looks fine over the wire but causes the
// device to never persist the api_key and then 400 on every
// subsequent /api/display call with a missing Access-Token header.
type setupResponse struct {
	Status     int    `json:"status"`
	APIKey     string `json:"api_key"`
	FriendlyID string `json:"friendly_id,omitempty"`
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	mac := r.Header.Get("ID")
	if mac == "" {
		http.Error(w, "missing ID header", http.StatusBadRequest)
		return
	}

	if !h.Config.Server.SetupMode {
		http.Error(w, "setup mode disabled", http.StatusForbidden)
		return
	}

	dev, err := h.Devices.Register(mac)
	if err != nil {
		h.Logger.Error("register device", "mac", mac, "err", err)
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	h.Logger.Info("device registered", "mac", mac)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(setupResponse{
		Status:     200,
		APIKey:     dev.APIKey,
		FriendlyID: friendlyIDFromMAC(mac),
	})
}

// friendlyIDFromMAC mirrors TRMNL's friendly-ID convention: the
// last three octets of the MAC, hex-uppercased, no separators.
// Example: 94:A9:90:8C:6C:2C -> "8C6C2C". Used as a stable
// human-readable identifier in firmware logs and UI.
func friendlyIDFromMAC(mac string) string {
	clean := strings.ToUpper(strings.ReplaceAll(mac, ":", ""))
	if len(clean) < 6 {
		return clean
	}
	return clean[len(clean)-6:]
}
