package collections

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	DefaultMessageLimit  = 50
	MaxMessageLimit      = 100
	MinSearchQueryLength = 3
	MaxSearchQueryLength = 100
)

type Repository interface {
	Start(context.Context, uuid.UUID) (Job, error)
	Latest(context.Context, uuid.UUID) (Job, error)
	Retry(context.Context, uuid.UUID) (Job, error)
	ListMessages(context.Context, uuid.UUID, int, *Cursor) ([]ChatMessage, error)
	SearchMessages(context.Context, uuid.UUID, string, int, *Cursor) ([]ChatMessage, error)
}

type ActivityRepository interface {
	ChatActivity(context.Context, uuid.UUID, int) ([]ActivityBucket, error)
}

type Service struct {
	repository Repository
}

var _ Repository = (*PostgresRepository)(nil)
var _ ActivityRepository = (*PostgresRepository)(nil)

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (service *Service) Start(ctx context.Context, streamID uuid.UUID) (Job, error) {
	if streamID == uuid.Nil {
		return Job{}, fmt.Errorf("%w: stream ID is required", ErrInvalidRequest)
	}
	return service.repository.Start(ctx, streamID)
}

func (service *Service) Latest(ctx context.Context, streamID uuid.UUID) (Job, error) {
	if streamID == uuid.Nil {
		return Job{}, fmt.Errorf("%w: stream ID is required", ErrInvalidRequest)
	}
	return service.repository.Latest(ctx, streamID)
}

func (service *Service) Retry(ctx context.Context, jobID uuid.UUID) (Job, error) {
	if jobID == uuid.Nil {
		return Job{}, fmt.Errorf("%w: job ID is required", ErrInvalidRequest)
	}
	return service.repository.Retry(ctx, jobID)
}

func (service *Service) ChatActivity(
	ctx context.Context,
	streamID uuid.UUID,
	bucketSeconds int,
) (Activity, error) {
	if streamID == uuid.Nil {
		return Activity{}, fmt.Errorf("%w: stream ID is required", ErrInvalidRequest)
	}
	if bucketSeconds != 5 && bucketSeconds != 10 && bucketSeconds != 30 {
		return Activity{}, fmt.Errorf("%w: bucket seconds must be 5, 10, or 30", ErrInvalidRequest)
	}
	repository, ok := service.repository.(ActivityRepository)
	if !ok {
		return Activity{}, fmt.Errorf("%w: chat activity is unavailable", ErrInvalidRequest)
	}
	items, err := repository.ChatActivity(ctx, streamID, bucketSeconds)
	if err != nil {
		return Activity{}, err
	}
	return Activity{BucketSeconds: bucketSeconds, Items: items}, nil
}

func (service *Service) ListMessages(
	ctx context.Context,
	streamID uuid.UUID,
	limit int,
	encodedCursor string,
) (MessagePage, error) {
	if streamID == uuid.Nil {
		return MessagePage{}, fmt.Errorf("%w: stream ID is required", ErrInvalidRequest)
	}
	if limit < 1 || limit > MaxMessageLimit {
		return MessagePage{}, fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidRequest, MaxMessageLimit)
	}
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return MessagePage{}, err
	}

	items, err := service.repository.ListMessages(ctx, streamID, limit+1, cursor)
	if err != nil {
		return MessagePage{}, err
	}
	page := MessagePage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		next := EncodeCursor(Cursor{OffsetMilliseconds: last.OffsetMilliseconds, ID: last.ID})
		page.NextCursor = &next
	}
	return page, nil
}

func (service *Service) SearchMessages(
	ctx context.Context,
	streamID uuid.UUID,
	query string,
	limit int,
	encodedCursor string,
) (MessagePage, error) {
	if streamID == uuid.Nil {
		return MessagePage{}, fmt.Errorf("%w: stream ID is required", ErrInvalidRequest)
	}
	query = strings.TrimSpace(query)
	queryLength := utf8.RuneCountInString(query)
	if queryLength < MinSearchQueryLength || queryLength > MaxSearchQueryLength {
		return MessagePage{}, fmt.Errorf(
			"%w: search query must be between %d and %d characters",
			ErrInvalidRequest,
			MinSearchQueryLength,
			MaxSearchQueryLength,
		)
	}
	if limit < 1 || limit > MaxMessageLimit {
		return MessagePage{}, fmt.Errorf("%w: limit must be between 1 and %d", ErrInvalidRequest, MaxMessageLimit)
	}
	cursor, err := DecodeCursor(encodedCursor)
	if err != nil {
		return MessagePage{}, err
	}

	items, err := service.repository.SearchMessages(ctx, streamID, query, limit+1, cursor)
	if err != nil {
		return MessagePage{}, err
	}
	page := MessagePage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[len(page.Items)-1]
		next := EncodeCursor(Cursor{OffsetMilliseconds: last.OffsetMilliseconds, ID: last.ID})
		page.NextCursor = &next
	}
	return page, nil
}
