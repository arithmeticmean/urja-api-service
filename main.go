package main

import (
	"context"
	_ "embed"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"urja-api-service/client"
	"urja-api-service/config"
	"urja-api-service/controllers"
	"urja-api-service/repositories"
	"urja-api-service/routes"
	"urja-api-service/services"
	"urja-api-service/workers"
)

// openapi.json is compiled into the binary and served at /openapi.json and /docs.
//
//go:embed openapi.json
var openapiSpec []byte

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Stops everything on Ctrl+C or `docker stop`.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	pc := client.New(cfg.PortalBaseURL, cfg.PortalEmail, cfg.PortalPassword, cfg.PortalRPS)
	repo := repositories.New(db)

	// Background work: one scheduler, one pool of workers.
	scheduler := workers.NewScheduler(repo, cfg.MeterSyncEvery, cfg.EnergySyncEvery, cfg.EnergyCheckEvery)
	workers.StartPool(ctx, cfg.Workers, scheduler.Jobs(), workers.New(pc, repo))
	go scheduler.Run(ctx)

	// HTTP API.
	meters := controllers.NewMeterController(services.NewMeterService(repo))
	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: routes.New(meters, controllers.NewDocs(openapiSpec)),
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server", "err", err)
			stop()
		}
	}()

	slog.Info("started", "addr", cfg.HTTPAddr, "workers", cfg.Workers)
	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)
}
