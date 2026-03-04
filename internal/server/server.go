package server

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/device"
	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/handler"
	"github.com/olivere/flipper/internal/screen"
	"github.com/olivere/flipper/internal/screen/static"
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
		s, err := static.New(cfg.Screens.Static.Dir)
		if err != nil {
			logger.Warn("static screen disabled", "err", err)
		} else {
			screens.Add(s)
			logger.Info("static screen enabled", "dir", cfg.Screens.Static.Dir)
		}
	}

	h := &handler.Handler{
		Config:   cfg,
		Devices:  registry,
		Screens:  screens,
		Pipeline: pipeline,
		Cache:    cache,
		Logger:   logger,
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

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
