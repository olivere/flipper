package handler

import (
	"encoding/json"
	"net/http"
)

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
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"api_key": dev.APIKey,
	})
}
