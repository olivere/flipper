package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/firmware"
	"github.com/olivere/flipper/internal/handler"
	"github.com/olivere/flipper/internal/screen"
	"github.com/olivere/flipper/internal/selfcert"
)

// Run wires up the device registry, screen registry, image pipeline,
// and HTTP routes, then starts the server. It blocks until ctx is
// cancelled or an OS signal (SIGINT, SIGTERM) is received, then shuts
// down gracefully with a 5-second timeout.
func Run(ctx context.Context, cfg *config.Config) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	registry, err := device.NewRegistry(cfg.Server.SecretKey)
	if err != nil {
		return err
	}

	pipeline := display.NewPipeline()
	cache := handler.NewImageCache()

	screens := screen.NewRegistry()
	if cfg.Screens.Static.Dir != "" {
		s, err := screen.Build("static", cfg, nil)
		if err != nil {
			logger.Warn("static screen disabled", "err", err)
		} else {
			screens.Add(s)
			logger.Info("static screen enabled", "dir", cfg.Screens.Static.Dir)
		}
	}
	if cfg.Screens.Demo.Enabled {
		s, err := screen.Build("demo", cfg, nil)
		if err != nil {
			logger.Warn("demo screen disabled", "err", err)
		} else {
			screens.Add(s)
			logger.Info("demo screen enabled")
		}
	}

	playlists, err := buildPlaylists(cfg, screens, logger)
	if err != nil {
		return err
	}

	var fw *handler.Firmware
	if cfg.Firmware.Enabled {
		store, err := firmware.NewStore()
		if err != nil {
			return fmt.Errorf("init firmware store: %w", err)
		}
		pending, err := firmware.NewPending()
		if err != nil {
			return fmt.Errorf("init firmware pending: %w", err)
		}
		fw = &handler.Firmware{Store: store, Pending: pending}
		logger.Info("firmware support enabled", "dir", store.Dir())
	}

	h := &handler.Handler{
		Config:    cfg,
		Devices:   registry,
		Screens:   screens,
		Playlists: playlists,
		Pipeline:  pipeline,
		Cache:     cache,
		Firmware:  fw,
		Logger:    logger,
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(logger))

	r.Get("/api/setup", h.Setup)
	r.Get("/api/display", h.Display)
	r.Post("/api/log", h.Log)
	r.Get("/images/{filename}", h.ServeImage)
	if fw != nil {
		r.Get("/firmware/{filename}", h.ServeFirmware)
	}

	srv := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: r,
	}

	tlsCfg := cfg.Server.TLS
	useTLS := !tlsCfg.Disabled

	if useTLS && tlsCfg.CertFile == "" && tlsCfg.KeyFile == "" {
		cert, err := selfcert.Generate()
		if err != nil {
			return err
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	serve := srv.ListenAndServe
	if useTLS {
		serve = func() error { return srv.ListenAndServeTLS(tlsCfg.CertFile, tlsCfg.KeyFile) }
	}

	logger.Info("server starting", "addr", cfg.Server.Addr, "tls", useTLS)
	go func() {
		if err := serve(); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
		}
		logger.Info("stopped serving new connections")
		stop()
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path),
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration", time.Since(start).String(),
				"from", r.RemoteAddr,
				"id", r.Header.Get("ID"),
			)
		})
	}
}

// buildPlaylists assembles the default and named playlists and the
// per-device assignments. Devices without an assignment use the
// top-level [[playlist]]; when that is empty and exactly one named
// playlist exists, it becomes the default. Returns nil when no
// playlists are configured at all. An assignment referencing an
// unknown playlist name is a startup error.
func buildPlaylists(cfg *config.Config, screens *screen.Registry, logger *slog.Logger) (*screen.Playlists, error) {
	if len(cfg.Playlist) == 0 && len(cfg.Playlists) == 0 {
		return nil, nil
	}

	// Reuse the static screen instance across all playlist entries so
	// "static" entries share the same directory scanner and index.
	var staticScreen screen.Screen
	for _, s := range screens.All() {
		if s.Name() == "static" {
			staticScreen = s
			break
		}
	}

	build := func(name string, entries []config.PlaylistEntry) *screen.Playlist {
		pl := screen.NewPlaylist()
		for _, entry := range entries {
			dur, err := parseDuration(entry.Duration)
			if err != nil {
				logger.Warn("invalid playlist duration, skipping entry",
					"playlist", name, "screen", entry.Screen, "duration", entry.Duration, "err", err)
				continue
			}

			// For static, reuse the shared instance instead of creating a new one.
			if entry.Screen == "static" && staticScreen != nil {
				pl.Add(staticScreen, dur)
				continue
			}

			scr, err := screen.Build(entry.Screen, cfg, entry.Params)
			if err != nil {
				logger.Warn("playlist screen skipped",
					"playlist", name, "screen", entry.Screen, "err", err)
				continue
			}
			pl.Add(scr, dur)
		}
		logger.Info("playlist enabled", "playlist", name, "entries", pl.Len())
		return pl
	}

	playlists := &screen.Playlists{ByDevice: make(map[string]*screen.Playlist)}

	if len(cfg.Playlist) > 0 {
		playlists.Default = build("default", cfg.Playlist)
	}

	named := make(map[string]*screen.Playlist, len(cfg.Playlists))
	for name, entries := range cfg.Playlists {
		named[name] = build(name, entries)
	}

	// A single named playlist with no explicit default serves everyone.
	if playlists.Default == nil && len(named) == 1 {
		for _, pl := range named {
			playlists.Default = pl
		}
	}

	for mac, override := range cfg.Devices {
		if override.Playlist == "" {
			continue
		}
		pl, ok := named[override.Playlist]
		if !ok {
			return nil, fmt.Errorf("device %q references unknown playlist %q", mac, override.Playlist)
		}
		playlists.ByDevice[device.NormalizeMAC(mac)] = pl
	}

	return playlists, nil
}

// parseDuration parses a duration string like "2m", "60s", or "".
// An empty string returns (0, nil) meaning "use default refresh rate".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
