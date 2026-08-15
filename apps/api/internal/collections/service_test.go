package collections

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestListMessagesUsesStableOpaqueCursor(t *testing.T) {
	streamID := uuid.New()
	firstID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	secondID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	repository := &repositoryStub{messages: []ChatMessage{
		{ID: firstID, OffsetMilliseconds: 1000},
		{ID: secondID, OffsetMilliseconds: 1000},
	}}
	service := NewService(repository)

	page, err := service.ListMessages(context.Background(), streamID, 1, "")
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != firstID || page.NextCursor == nil {
		t.Fatalf("unexpected first page: %+v", page)
	}

	cursor, err := DecodeCursor(*page.NextCursor)
	if err != nil {
		t.Fatalf("decode next cursor: %v", err)
	}
	if cursor.OffsetMilliseconds != 1000 || cursor.ID != firstID {
		t.Fatalf("unexpected cursor: %+v", cursor)
	}
}

func TestDecodeCursorRejectsMalformedInput(t *testing.T) {
	_, err := DecodeCursor("not-a-cursor")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestSearchMessagesRejectsInvalidQueryAndLimit(t *testing.T) {
	service := NewService(&repositoryStub{})
	streamID := uuid.New()

	tests := []struct {
		name  string
		query string
		limit int
	}{
		{name: "blank query", query: "   ", limit: DefaultMessageLimit},
		{name: "short query", query: "ab", limit: DefaultMessageLimit},
		{name: "long query", query: strings.Repeat("a", 101), limit: DefaultMessageLimit},
		{name: "zero limit", query: "music", limit: 0},
		{name: "excessive limit", query: "music", limit: MaxMessageLimit + 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.SearchMessages(context.Background(), streamID, test.query, test.limit, "")
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("expected invalid request, got %v", err)
			}
		})
	}
}

func TestSafeErrorsIgnoreStoredMessagesAndUseStableRetryability(t *testing.T) {
	tests := []struct {
		code      string
		retryable bool
	}{
		{code: "SOURCE_NOT_READY", retryable: true},
		{code: "YOUTUBE_RATE_LIMITED", retryable: true},
		{code: "YTDLP_TIMEOUT", retryable: true},
		{code: "YTDLP_PROCESS_FAILED", retryable: true},
		{code: "CHAT_IMPORT_FAILED", retryable: true},
		{code: "CHAT_REPLAY_NOT_AVAILABLE"},
		{code: "YOUTUBE_ACCESS_DENIED"},
		{code: "YTDLP_OUTPUT_CHANGED"},
		{code: "UNKNOWN_WITH_/tmp/secret-cookie.txt"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			safe := safeErrorFor(test.code)
			if safe.Retryable != test.retryable {
				t.Fatalf("retryable=%t, want %t", safe.Retryable, test.retryable)
			}
			if test.code != "UNKNOWN_WITH_/tmp/secret-cookie.txt" && safe.Code != test.code {
				t.Fatalf("code=%q, want %q", safe.Code, test.code)
			}
			if test.code == "UNKNOWN_WITH_/tmp/secret-cookie.txt" && safe.Code != "COLLECTION_FAILED" {
				t.Fatalf("unknown code was exposed: %q", safe.Code)
			}
		})
	}
}

type repositoryStub struct {
	job      Job
	messages []ChatMessage
	err      error
	cursor   *Cursor
	limit    int
}

func (repository *repositoryStub) Start(context.Context, uuid.UUID) (Job, error) {
	return repository.job, repository.err
}

func (repository *repositoryStub) Latest(context.Context, uuid.UUID) (Job, error) {
	return repository.job, repository.err
}

func (repository *repositoryStub) Retry(context.Context, uuid.UUID) (Job, error) {
	return repository.job, repository.err
}

func (repository *repositoryStub) ListMessages(
	_ context.Context,
	_ uuid.UUID,
	limit int,
	cursor *Cursor,
) ([]ChatMessage, error) {
	repository.limit = limit
	repository.cursor = cursor
	return repository.messages, repository.err
}

func (repository *repositoryStub) SearchMessages(
	_ context.Context,
	_ uuid.UUID,
	_ string,
	limit int,
	cursor *Cursor,
) ([]ChatMessage, error) {
	repository.limit = limit
	repository.cursor = cursor
	return repository.messages, repository.err
}

func jobFixture(streamID uuid.UUID) Job {
	now := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	return Job{
		ID:          uuid.New(),
		StreamID:    streamID,
		Kind:        "chat_replay",
		Status:      StatusQueued,
		RequestedAt: now,
		UpdatedAt:   now,
	}
}
