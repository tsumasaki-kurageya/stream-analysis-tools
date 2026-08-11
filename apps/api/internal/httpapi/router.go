package httpapi

import (
	"encoding/json"
	"net/http"
)

type healthResponse struct {
	Component string `json:"component"`
	Status    string `json:"status"`
}

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(healthResponse{
			Component: "main-api",
			Status:    "ok",
		})
	})
	return mux
}
