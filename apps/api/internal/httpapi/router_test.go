package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	openapiv1 "github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/generated/openapiv1"
)

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}

	var body openapiv1.HealthResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Component != "main-api" || body.Status != openapiv1.Ok {
		t.Fatalf("unexpected response: %+v", body)
	}
}
