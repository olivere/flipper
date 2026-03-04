package server

import (
	"context"
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
)

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

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("server starting", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutCtx)
}
