package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/database"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/httpapi"
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
		log.Fatalf("configure PostgreSQL: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := database.ApplyMigrations(
		ctx,
		pool,
		os.DirFS(envOrDefault("YSA_MIGRATIONS_DIR", "migrations")),
	); err != nil {
		log.Fatalf("apply database migrations: %v", err)
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
		log.Fatalf("configure YouTube metadata client: %v", err)
	}
	streamService := streams.NewService(streams.NewPostgresRepository(pool), metadataClient)

	address := ":" + envOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.NewHandler(streamService),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("main API listening on %s", address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
