package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) ServeImage(w http.ResponseWriter, r *http.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		http.NotFound(w, r)
		return
	}

	result := h.Cache.GetByFilename(filename)
	if result == nil {
		http.NotFound(w, r)
		return
	}

	contentType := "image/bmp"
	if result.Format == "png" {
		contentType = "image/png"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(result.Data)
}
