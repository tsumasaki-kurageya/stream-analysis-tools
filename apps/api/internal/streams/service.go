package streams

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	ErrInvalidYouTubeURL           = errors.New("invalid YouTube URL")
	ErrVideoUnavailable            = errors.New("YouTube video unavailable")
	ErrMetadataProviderUnavailable = errors.New("metadata provider unavailable")
)

type MetadataProvider interface {
	Fetch(context.Context, string) (Metadata, error)
}

type Service struct {
	repository Repository
	provider   MetadataProvider
}

func NewService(repository Repository, provider MetadataProvider) *Service {
	return &Service{repository: repository, provider: provider}
}

func (service *Service) Preview(ctx context.Context, rawURL string) (Metadata, error) {
	return service.fetchMetadata(ctx, rawURL)
}

func (service *Service) Register(ctx context.Context, rawURL string) (Stream, error) {
	metadata, err := service.fetchMetadata(ctx, rawURL)
	if err != nil {
		return Stream{}, err
	}
	stream, err := service.repository.Create(ctx, metadata)
	if err != nil {
		return Stream{}, fmt.Errorf("register stream: %w", err)
	}
	return stream, nil
}

func (service *Service) List(ctx context.Context, options ListOptions) ([]Stream, error) {
	return service.repository.List(ctx, options)
}

func (service *Service) ListItems(ctx context.Context, options ListOptions) ([]ListItem, error) {
	return service.repository.ListItems(ctx, options)
}

func (service *Service) Get(ctx context.Context, id uuid.UUID) (Stream, error) {
	return service.repository.Get(ctx, id)
}

func (service *Service) fetchMetadata(ctx context.Context, rawURL string) (Metadata, error) {
	identity, err := ParseYouTubeURL(rawURL)
	if err != nil {
		return Metadata{}, err
	}
	if service.provider == nil {
		return Metadata{}, ErrMetadataProviderUnavailable
	}

	metadata, err := service.provider.Fetch(ctx, identity.VideoID)
	if err != nil {
		return Metadata{}, fmt.Errorf("fetch YouTube metadata: %w", err)
	}
	metadata.YouTubeVideoID = identity.VideoID
	metadata.CanonicalURL = identity.CanonicalURL
	metadata, err = normalizeMetadata(metadata)
	if err != nil {
		return Metadata{}, fmt.Errorf("validate YouTube metadata: %w", ErrMetadataProviderUnavailable)
	}
	return metadata, nil
}
