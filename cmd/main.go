package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/sikozonpc/cinema/internal/features/catalog"
	"github.com/sikozonpc/cinema/internal/features/reservations"
	"github.com/sikozonpc/cinema/internal/platform/cache"
	"github.com/sikozonpc/cinema/internal/platform/config"
	"github.com/sikozonpc/cinema/internal/platform/web"
)

func main() {
	settings := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: settings.LogLevel,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	redisStore, err := cache.OpenRedis(ctx, settings.Redis)
	if err != nil {
		logger.Error("redis unavailable", "error", err)
		os.Exit(1)
	}
	defer redisStore.Close()

	movies := catalog.NewLibrary()
	claimBook := reservations.NewCacheClaimBook(redisStore, settings.Reservation.HoldTTL)
	reservationDesk := reservations.NewDesk(claimBook, settings.Reservation.HoldTTL)

	mux := http.NewServeMux()
	catalog.NewHTTPHandler(movies).Mount(mux)
	reservations.NewHTTPHandler(reservationDesk, logger).Mount(mux)
	mountOperationalRoutes(mux, redisStore)
	mountStaticFiles(mux, settings.StaticDir)

	handler := web.Chain(
		mux,
		web.RecoverPanic(logger),
		web.AttachRequestID(),
		web.LogRequests(logger),
	)

	server := &http.Server{
		Addr:              settings.HTTP.Addr,
		Handler:           handler,
		ReadHeaderTimeout: settings.HTTP.ReadHeaderTimeout,
		ReadTimeout:       settings.HTTP.ReadTimeout,
		WriteTimeout:      settings.HTTP.WriteTimeout,
		IdleTimeout:       settings.HTTP.IdleTimeout,
	}

	logger.Info("cinema reservation api listening", "addr", settings.HTTP.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func mountOperationalRoutes(mux *http.ServeMux, health cache.HealthChecker) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		web.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 800*time.Millisecond)
		defer cancel()
		if err := health.Ping(ctx); err != nil {
			web.Problem(w, r, http.StatusServiceUnavailable, "redis_unavailable", "Redis is not reachable")
			return
		}
		web.JSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
}

func mountStaticFiles(mux *http.ServeMux, dir string) {
	mux.Handle("GET /", http.FileServer(http.Dir(dir)))
}
