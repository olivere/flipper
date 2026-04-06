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

	// Build playlist if configured.
	var playlist *screen.Playlist
	if len(cfg.Playlist) > 0 {
		playlist = screen.NewPlaylist()

		// Reuse the static screen instance across playlist entries so all
		// "static" entries share the same directory scanner and index.
		var staticScreen screen.Screen
		for _, s := range screens.All() {
			if s.Name() == "static" {
				staticScreen = s
				break
			}
		}

		for _, entry := range cfg.Playlist {
			dur, err := parseDuration(entry.Duration)
			if err != nil {
				logger.Warn("invalid playlist duration, skipping entry",
					"screen", entry.Screen, "duration", entry.Duration, "err", err)
				continue
			}

			// For static, reuse the shared instance instead of creating a new one.
			if entry.Screen == "static" && staticScreen != nil {
				playlist.Add(staticScreen, dur)
				continue
			}

			scr, err := screen.Build(entry.Screen, cfg, entry.Params)
			if err != nil {
				logger.Warn("playlist screen skipped", "screen", entry.Screen, "err", err)
				continue
			}
			playlist.Add(scr, dur)
		}
		logger.Info("playlist enabled", "entries", playlist.Len())
	}

	h := &handler.Handler{
		Config:   cfg,
		Devices:  registry,
		Screens:  screens,
		Playlist: playlist,
		Pipeline: pipeline,
		Cache:    cache,
		Logger:   logger,
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(logger))

	r.Get("/api/setup", h.Setup)
	r.Get("/api/display", h.Display)
	r.Post("/api/log", h.Log)
	r.Get("/images/{filename}", h.ServeImage)

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

// parseDuration parses a duration string like "2m", "60s", or "".
// An empty string returns (0, nil) meaning "use default refresh rate".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
