package reservations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

var ErrInvalidURL = errors.New("invalid reservation URL")

type Repository interface {
	Create(context.Context, Reservation) (Reservation, error)
	List(context.Context, ListOptions) ([]Reservation, int, error)
	Get(context.Context, uuid.UUID) (Reservation, error)
	Cancel(context.Context, uuid.UUID, time.Time) (Reservation, error)
}

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository, now func() time.Time) *Service {
	return &Service{repository: repository, now: now}
}

func (service *Service) Create(ctx context.Context, rawURL string) (Reservation, error) {
	parsed, err := streams.ParseYouTubeURL(rawURL)
	if err != nil {
		return Reservation{}, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	now := service.now().UTC()
	return service.repository.Create(ctx, Reservation{
		YouTubeVideoID: parsed.VideoID,
		SourceURL:      parsed.CanonicalURL,
		State:          StateScheduled,
		NextCheckAt:    now,
	})
}

func (service *Service) List(ctx context.Context, options ListOptions) ([]Reservation, int, error) {
	return service.repository.List(ctx, options)
}

func (service *Service) Get(ctx context.Context, id uuid.UUID) (Reservation, error) {
	return service.repository.Get(ctx, id)
}

func (service *Service) Cancel(ctx context.Context, id uuid.UUID) (Reservation, error) {
	return service.repository.Cancel(ctx, id, service.now().UTC())
}

var _ Repository = (*PostgresRepository)(nil)
