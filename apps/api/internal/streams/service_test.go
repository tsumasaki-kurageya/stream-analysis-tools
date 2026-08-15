package streams

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRegisterRevalidatesMetadataAfterPreview(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	provider := &sequenceProvider{results: []Metadata{
		validMetadata("Preview title", now),
		validMetadata("Current title", now.Add(time.Minute)),
	}}
	repository := &recordingRepository{}
	service := NewService(repository, provider)

	preview, err := service.Preview(context.Background(), "https://youtu.be/dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Title != "Preview title" || repository.createCalls != 0 {
		t.Fatalf("preview should not persist metadata: %+v", preview)
	}

	registered, err := service.Register(
		context.Background(),
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if registered.Title != "Current title" {
		t.Fatalf("expected revalidated title, got %q", registered.Title)
	}
	if repository.created.MetadataFetchedAt != now.Add(time.Minute) {
		t.Fatalf("expected current metadata to be persisted: %+v", repository.created)
	}
	if provider.calls != 2 || repository.createCalls != 1 {
		t.Fatalf("expected two fetches and one create, got fetches=%d creates=%d", provider.calls, repository.createCalls)
	}
}

func TestRegisterPreservesDuplicateError(t *testing.T) {
	provider := &sequenceProvider{results: []Metadata{validMetadata("Title", time.Now())}}
	repository := &recordingRepository{createErr: ErrYouTubeVideoIDExists}
	service := NewService(repository, provider)

	_, err := service.Register(context.Background(), "https://youtu.be/dQw4w9WgXcQ")
	if !errors.Is(err, ErrYouTubeVideoIDExists) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

type sequenceProvider struct {
	results []Metadata
	err     error
	calls   int
}

func (provider *sequenceProvider) Fetch(context.Context, string) (Metadata, error) {
	if provider.err != nil {
		return Metadata{}, provider.err
	}
	index := provider.calls
	provider.calls++
	if index >= len(provider.results) {
		index = len(provider.results) - 1
	}
	return provider.results[index], nil
}

type recordingRepository struct {
	created     Metadata
	createCalls int
	createErr   error
}

func (repository *recordingRepository) Create(_ context.Context, metadata Metadata) (Stream, error) {
	repository.created = metadata
	repository.createCalls++
	if repository.createErr != nil {
		return Stream{}, repository.createErr
	}
	return Stream{
		ID:                uuid.New(),
		YouTubeVideoID:    metadata.YouTubeVideoID,
		CanonicalURL:      metadata.CanonicalURL,
		Title:             metadata.Title,
		ChannelID:         metadata.ChannelID,
		ChannelTitle:      metadata.ChannelTitle,
		LifecycleStatus:   metadata.LifecycleStatus,
		MetadataFetchedAt: metadata.MetadataFetchedAt,
		CreatedAt:         metadata.MetadataFetchedAt,
		UpdatedAt:         metadata.MetadataFetchedAt,
	}, nil
}

func (*recordingRepository) Upsert(context.Context, Metadata) (Stream, error) {
	panic("unexpected Upsert call")
}

func (*recordingRepository) Get(context.Context, uuid.UUID) (Stream, error) {
	return Stream{}, ErrNotFound
}

func (*recordingRepository) GetByYouTubeVideoID(context.Context, string) (Stream, error) {
	return Stream{}, ErrNotFound
}

func (*recordingRepository) List(context.Context, ListOptions) ([]Stream, error) {
	return nil, nil
}

func validMetadata(title string, fetchedAt time.Time) Metadata {
	return Metadata{
		Title:             title,
		ChannelID:         "UC123",
		ChannelTitle:      "Channel",
		LifecycleStatus:   LifecycleEnded,
		MetadataFetchedAt: fetchedAt,
	}
}
