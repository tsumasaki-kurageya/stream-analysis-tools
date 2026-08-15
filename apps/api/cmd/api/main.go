package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/collections"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/database"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/httpapi"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/reservations"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/youtube"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, envOrDefault(
		"YSA_DATABASE_URL",
		"postgresql://stream_analysis:stream_analysis_local@localhost:5432/stream_analysis?sslmode=disable",
	))
	if err != nil {
		log.Fatal("configure PostgreSQL: DATABASE_CONFIGURATION_ERROR")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal("connect to PostgreSQL: DATABASE_CONNECTION_ERROR")
	}
	if err := database.ApplyMigrations(
		ctx,
		pool,
		os.DirFS(envOrDefault("YSA_MIGRATIONS_DIR", "migrations")),
	); err != nil {
		log.Fatal("apply database migrations: DATABASE_MIGRATION_ERROR")
	}

	metadataClient, err := youtube.NewClient(
		os.Getenv("YSA_YOUTUBE_API_KEY"),
		envOrDefault("YSA_YOUTUBE_API_BASE_URL", youtube.DefaultBaseURL),
		&http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	)
	if err != nil {
		log.Fatal("configure YouTube metadata client: METADATA_CLIENT_CONFIGURATION_ERROR")
	}
	streamService := streams.NewService(streams.NewPostgresRepository(pool), metadataClient)
	collectionService := collections.NewService(collections.NewPostgresRepository(pool))
	reservationRepository := reservations.NewPostgresRepository(pool)
	reservationService := reservations.NewService(reservationRepository, time.Now)
	reservationMonitor := reservations.NewMonitor(
		reservationRepository,
		metadataClient,
		reservationMonitorWorkerID(),
		time.Now,
		2*time.Minute,
		reservations.NewJSONMonitorObserver(os.Stdout),
	)
	go runReservationMonitor(context.Background(), reservationMonitor)

	address := ":" + envOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.NewHandlerWithReservations(streamService, collectionService, reservationService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("main API listening on %s", address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("main API stopped: HTTP_SERVER_ERROR")
	}
}

func runReservationMonitor(ctx context.Context, monitor *reservations.Monitor) {
	const pollInterval = time.Second
	for {
		didWork, err := monitor.RunOnce(ctx)
		if err != nil {
			log.Printf("reservation monitor failed: RESERVATION_MONITOR_RUN_FAILED")
		}
		if didWork && err == nil {
			continue
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func reservationMonitorWorkerID() string {
	if configured := os.Getenv("YSA_RESERVATION_MONITOR_WORKER_ID"); configured != "" {
		return configured
	}
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname
	}
	return "main-api"
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
