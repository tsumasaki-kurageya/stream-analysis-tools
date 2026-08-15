package reservations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestServiceCreatesCanonicalReservationAndCancelsSupportedState(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	repository := &serviceRepositoryStub{}
	service := NewService(repository, func() time.Time { return now })

	created, err := service.Create(context.Background(), "https://youtu.be/dQw4w9WgXcQ")
	if err != nil {
		t.Fatalf("create reservation: %v", err)
	}
	if created.YouTubeVideoID != "dQw4w9WgXcQ" || created.SourceURL != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Fatalf("unexpected canonical reservation: %+v", created)
	}
	if created.State != StateScheduled || !created.NextCheckAt.Equal(now) {
		t.Fatalf("unexpected initial state: %+v", created)
	}

	canceled, err := service.Cancel(context.Background(), created.ID)
	if err != nil || canceled.State != StateCanceled {
		t.Fatalf("cancel reservation: %+v, %v", canceled, err)
	}
}

func TestServiceRejectsInvalidReservationURLBeforePersistence(t *testing.T) {
	repository := &serviceRepositoryStub{}
	service := NewService(repository, time.Now)

	_, err := service.Create(context.Background(), "https://youtube.com.evil.example/watch?v=dQw4w9WgXcQ")
	if !errors.Is(err, ErrInvalidURL) || repository.created.ID != uuid.Nil {
		t.Fatalf("expected invalid URL without persistence, got %v", err)
	}
}

type serviceRepositoryStub struct {
	created Reservation
}

func (repository *serviceRepositoryStub) Create(_ context.Context, reservation Reservation) (Reservation, error) {
	reservation.ID = uuid.New()
	reservation.CreatedAt = reservation.NextCheckAt
	reservation.UpdatedAt = reservation.NextCheckAt
	repository.created = reservation
	return reservation, nil
}

func (*serviceRepositoryStub) List(context.Context, ListOptions) ([]Reservation, int, error) {
	return nil, 0, nil
}

func (repository *serviceRepositoryStub) Get(context.Context, uuid.UUID) (Reservation, error) {
	return repository.created, nil
}

func (repository *serviceRepositoryStub) Cancel(_ context.Context, _ uuid.UUID, canceledAt time.Time) (Reservation, error) {
	repository.created.State = StateCanceled
	repository.created.UpdatedAt = canceledAt
	return repository.created, nil
}
