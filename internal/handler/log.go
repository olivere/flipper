package handler

import (
	"encoding/json"
	"io"
	"net/http"
)

func (h *Handler) Log(w http.ResponseWriter, r *http.Request) {
	mac := r.Header.Get("ID")

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Try to parse as JSON for structured logging
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		h.Logger.Info("device log", "mac", mac, "payload", payload)
	} else {
		h.Logger.Info("device log", "mac", mac, "raw", string(body))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
