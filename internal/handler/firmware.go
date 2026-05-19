package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ServeFirmware streams a firmware binary to a TRMNL device.
//
// The endpoint is intentionally unauthenticated. TRMNL devices do not
// send the ID / Access-Token headers when fetching the firmware_url
// returned by /api/display (verified empirically against OG firmware
// 1.7.4 — they send only standard HTTP headers), so requiring those
// headers would block every real OTA. The real security gate is
// /api/display, which decides which binary gets dispatched to which
// device; this endpoint only serves bytes that Flipper has already
// imported and that /api/display chose to advertise.
//
// We still enforce: the firmware feature must be enabled, the
// filename must match a binary in the store (Store.Open also rejects
// path traversal), the response sets an explicit Content-Length
// (TRMNL's bootloader needs the size up front; chunked transfer is
// rejected), and every request is logged with the source address so
// downloads remain auditable.
func (h *Handler) ServeFirmware(w http.ResponseWriter, r *http.Request) {
	if h.Firmware == nil || h.Firmware.Store == nil {
		http.NotFound(w, r)
		return
	}

	name := chi.URLParam(r, "filename")
	if _, ok := h.Firmware.Store.FindByFilename(name); !ok {
		http.NotFound(w, r)
		return
	}

	rc, size, err := h.Firmware.Store.Open(name)
	if err != nil {
		h.Logger.Error("firmware open failed", "file", name, "err", err)
		http.Error(w, "open failed", http.StatusInternalServerError)
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, rc); err != nil {
		h.Logger.Error("firmware stream failed", "file", name, "err", err)
		return
	}
	h.Logger.Info("firmware served", "file", name, "size", size, "from", r.RemoteAddr)
}
