package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ServeFirmware streams a firmware binary to an authenticated device.
//
// Auth uses the same MAC + Access-Token pair as /api/display; tokens
// are never adopted on this endpoint, so a device must already be
// fully registered before it can pull a binary. The response sets an
// explicit Content-Length (TRMNL's bootloader needs the size up
// front; chunked transfer is rejected) and never redirects.
func (h *Handler) ServeFirmware(w http.ResponseWriter, r *http.Request) {
	if h.Firmware == nil || h.Firmware.Store == nil {
		http.NotFound(w, r)
		return
	}

	mac := r.Header.Get("ID")
	token := r.Header.Get("Access-Token")
	if mac == "" || token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.Devices.Authenticate(mac, token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	name := chi.URLParam(r, "filename")
	if _, ok := h.Firmware.Store.FindByFilename(name); !ok {
		http.NotFound(w, r)
		return
	}

	rc, size, err := h.Firmware.Store.Open(name)
	if err != nil {
		h.Logger.Error("firmware open failed", "file", name, "mac", mac, "err", err)
		http.Error(w, "open failed", http.StatusInternalServerError)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, rc); err != nil {
		h.Logger.Error("firmware stream failed", "file", name, "mac", mac, "err", err)
		return
	}
	h.Logger.Info("firmware served", "mac", mac, "file", name, "size", size)
}
