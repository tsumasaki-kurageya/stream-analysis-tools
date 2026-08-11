package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/httpapi"
)

func main() {
	address := ":" + envOrDefault("PORT", "8080")
	server := &http.Server{
		Addr:              address,
		Handler:           httpapi.NewHandler(),
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
