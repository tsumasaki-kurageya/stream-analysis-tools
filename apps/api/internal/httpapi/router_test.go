package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	openapiv1 "github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/generated/openapiv1"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

const validYouTubeURL = "https://youtu.be/dQw4w9WgXcQ"

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	NewHandler(nil).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}

	var body openapiv1.HealthResponse
	decodeJSON(t, response, &body)
	if body.Component != "main-api" || body.Status != openapiv1.Ok {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestStreamAPIRegistrationRevalidatesAndSupportsReads(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	provider := &apiMetadataProvider{results: []streams.Metadata{
		apiMetadata("Preview title", now),
		apiMetadata("Revalidated title", now.Add(time.Minute)),
	}}
	repository := newAPIRepository()
	handler := NewHandler(streams.NewService(repository, provider))

	previewResponse := performJSONRequest(t, handler, http.MethodPost, "/v1/streams/preview", map[string]string{
		"url": validYouTubeURL,
	})
	if previewResponse.Code != http.StatusOK {
		t.Fatalf("expected preview status 200, got %d: %s", previewResponse.Code, previewResponse.Body.String())
	}
	var preview openapiv1.StreamPreview
	decodeJSON(t, previewResponse, &preview)
	if preview.Title != "Preview title" || len(repository.items) != 0 {
		t.Fatalf("preview should return but not persist first metadata: %+v", preview)
	}

	createResponse := performJSONRequest(t, handler, http.MethodPost, "/v1/streams", map[string]string{
		"url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	})
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d: %s", createResponse.Code, createResponse.Body.String())
	}
	var created openapiv1.Stream
	decodeJSON(t, createResponse, &created)
	if created.Title != "Revalidated title" || provider.calls != 2 {
		t.Fatalf("registration did not revalidate metadata: %+v, calls=%d", created, provider.calls)
	}
	if createResponse.Header().Get("Location") != "/v1/streams/"+created.Id {
		t.Fatalf("unexpected Location header %q", createResponse.Header().Get("Location"))
	}

	listResponse := performJSONRequest(t, handler, http.MethodGet, "/v1/streams?limit=10&offset=0", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listResponse.Code)
	}
	var list openapiv1.StreamList
	decodeJSON(t, listResponse, &list)
	if len(list.Items) != 1 || list.Items[0].Id != created.Id || list.Limit != 10 {
		t.Fatalf("unexpected list response: %+v", list)
	}

	detailResponse := performJSONRequest(t, handler, http.MethodGet, "/v1/streams/"+created.Id, nil)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d", detailResponse.Code)
	}
	var detail openapiv1.Stream
	decodeJSON(t, detailResponse, &detail)
	if detail.Id != created.Id || detail.Title != "Revalidated title" {
		t.Fatalf("unexpected detail response: %+v", detail)
	}
}

func TestStreamAPIRejectsMalformedYouTubeURL(t *testing.T) {
	provider := &apiMetadataProvider{results: []streams.Metadata{apiMetadata("unused", time.Now())}}
	handler := NewHandler(streams.NewService(newAPIRepository(), provider))

	response := performJSONRequest(t, handler, http.MethodPost, "/v1/streams/preview", map[string]string{
		"url": "https://youtube.com.evil.example/watch?v=dQw4w9WgXcQ",
	})
	assertProblem(t, response, http.StatusBadRequest, "INVALID_YOUTUBE_URL")
	if provider.calls != 0 {
		t.Fatalf("provider should not be called for an invalid URL, got %d calls", provider.calls)
	}
}

func TestStreamAPIClassifiesUnavailableVideo(t *testing.T) {
	provider := &apiMetadataProvider{err: streams.ErrVideoUnavailable}
	handler := NewHandler(streams.NewService(newAPIRepository(), provider))

	response := performJSONRequest(t, handler, http.MethodPost, "/v1/streams/preview", map[string]string{
		"url": validYouTubeURL,
	})
	assertProblem(t, response, http.StatusUnprocessableEntity, "VIDEO_UNAVAILABLE")
}

func TestStreamAPIRejectsDuplicateRegistration(t *testing.T) {
	provider := &apiMetadataProvider{results: []streams.Metadata{apiMetadata("Stream", time.Now())}}
	handler := NewHandler(streams.NewService(newAPIRepository(), provider))

	first := performJSONRequest(t, handler, http.MethodPost, "/v1/streams", map[string]string{
		"url": validYouTubeURL,
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first registration status 201, got %d: %s", first.Code, first.Body.String())
	}
	duplicate := performJSONRequest(t, handler, http.MethodPost, "/v1/streams", map[string]string{
		"url": validYouTubeURL,
	})
	assertProblem(t, duplicate, http.StatusConflict, "STREAM_ALREADY_REGISTERED")
}

func TestStreamAPIUsesProblemDetailsForMalformedJSONAndIdentifier(t *testing.T) {
	handler := NewHandler(streams.NewService(newAPIRepository(), &apiMetadataProvider{}))

	malformedRequest := httptest.NewRequest(http.MethodPost, "/v1/streams/preview", bytes.NewBufferString("{"))
	malformedRequest.Header.Set("Content-Type", "application/json")
	malformedResponse := httptest.NewRecorder()
	handler.ServeHTTP(malformedResponse, malformedRequest)
	assertProblem(t, malformedResponse, http.StatusBadRequest, "INVALID_REQUEST")

	identifierResponse := performJSONRequest(t, handler, http.MethodGet, "/v1/streams/not-a-uuid", nil)
	assertProblem(t, identifierResponse, http.StatusBadRequest, "INVALID_REQUEST")
}

func performJSONRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	var encoded bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&encoded).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, &encoded)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeJSON(t *testing.T, response *httptest.ResponseRecorder, destination any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
	}
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("expected status %d, got %d: %s", status, response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("unexpected content type %q", response.Header().Get("Content-Type"))
	}
	var problem openapiv1.ProblemDetails
	decodeJSON(t, response, &problem)
	if problem.Code != code || problem.Status != status {
		t.Fatalf("unexpected problem: %+v", problem)
	}
}

type apiMetadataProvider struct {
	results []streams.Metadata
	err     error
	calls   int
}

func (provider *apiMetadataProvider) Fetch(context.Context, string) (streams.Metadata, error) {
	provider.calls++
	if provider.err != nil {
		return streams.Metadata{}, provider.err
	}
	if len(provider.results) == 0 {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}
	index := provider.calls - 1
	if index >= len(provider.results) {
		index = len(provider.results) - 1
	}
	return provider.results[index], nil
}

type apiRepository struct {
	items map[uuid.UUID]streams.Stream
	byID  map[string]uuid.UUID
}

func newAPIRepository() *apiRepository {
	return &apiRepository{items: make(map[uuid.UUID]streams.Stream), byID: make(map[string]uuid.UUID)}
}

func (repository *apiRepository) Create(_ context.Context, metadata streams.Metadata) (streams.Stream, error) {
	if _, exists := repository.byID[metadata.YouTubeVideoID]; exists {
		return streams.Stream{}, streams.ErrYouTubeVideoIDExists
	}
	stream := streamFromMetadata(metadata)
	repository.items[stream.ID] = stream
	repository.byID[stream.YouTubeVideoID] = stream.ID
	return stream, nil
}

func (repository *apiRepository) Upsert(context.Context, streams.Metadata) (streams.Stream, error) {
	return streams.Stream{}, errors.New("unexpected Upsert call")
}

func (repository *apiRepository) Get(_ context.Context, id uuid.UUID) (streams.Stream, error) {
	stream, exists := repository.items[id]
	if !exists {
		return streams.Stream{}, streams.ErrNotFound
	}
	return stream, nil
}

func (repository *apiRepository) GetByYouTubeVideoID(_ context.Context, videoID string) (streams.Stream, error) {
	id, exists := repository.byID[videoID]
	if !exists {
		return streams.Stream{}, streams.ErrNotFound
	}
	return repository.items[id], nil
}

func (repository *apiRepository) List(_ context.Context, options streams.ListOptions) ([]streams.Stream, error) {
	if options.Limit < 1 || options.Limit > 100 || options.Offset < 0 {
		return nil, streams.ErrInvalidStream
	}
	all := make([]streams.Stream, 0, len(repository.items))
	for _, stream := range repository.items {
		all = append(all, stream)
	}
	if options.Offset >= len(all) {
		return []streams.Stream{}, nil
	}
	end := min(options.Offset+options.Limit, len(all))
	return all[options.Offset:end], nil
}

func streamFromMetadata(metadata streams.Metadata) streams.Stream {
	return streams.Stream{
		ID:                uuid.New(),
		YouTubeVideoID:    metadata.YouTubeVideoID,
		CanonicalURL:      metadata.CanonicalURL,
		Title:             metadata.Title,
		ChannelID:         metadata.ChannelID,
		ChannelTitle:      metadata.ChannelTitle,
		ThumbnailURL:      metadata.ThumbnailURL,
		ScheduledStartAt:  metadata.ScheduledStartAt,
		ActualStartAt:     metadata.ActualStartAt,
		ActualEndAt:       metadata.ActualEndAt,
		Duration:          metadata.Duration,
		LifecycleStatus:   metadata.LifecycleStatus,
		MetadataFetchedAt: metadata.MetadataFetchedAt,
		CreatedAt:         metadata.MetadataFetchedAt,
		UpdatedAt:         metadata.MetadataFetchedAt,
	}
}

func apiMetadata(title string, fetchedAt time.Time) streams.Metadata {
	duration := 90 * time.Minute
	thumbnail := "https://i.ytimg.com/example.jpg"
	actualStart := fetchedAt.Add(-duration)
	actualEnd := fetchedAt
	return streams.Metadata{
		Title:             title,
		ChannelID:         "UC123",
		ChannelTitle:      "Channel",
		ThumbnailURL:      &thumbnail,
		ActualStartAt:     &actualStart,
		ActualEndAt:       &actualEnd,
		Duration:          &duration,
		LifecycleStatus:   streams.LifecycleEnded,
		MetadataFetchedAt: fetchedAt,
	}
}
