package collections

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidRequest = errors.New("invalid collection request")
	ErrStreamNotFound = errors.New("stream not found")
	ErrJobNotFound    = errors.New("collection job not found")
	ErrNotRetryable   = errors.New("collection job is not retryable")
	ErrActiveJob      = errors.New("an active collection job already exists")
)

type JobStatus string

const (
	StatusQueued    JobStatus = "queued"
	StatusRunning   JobStatus = "running"
	StatusSucceeded JobStatus = "succeeded"
	StatusNoData    JobStatus = "no_data"
	StatusFailed    JobStatus = "failed"
)

type SafeError struct {
	Code      string
	Message   string
	Retryable bool
}

type Job struct {
	ID             uuid.UUID
	StreamID       uuid.UUID
	Kind           string
	Status         JobStatus
	Attempt        int
	ProcessedCount int64
	SkippedCount   int64
	RequestedAt    time.Time
	StartedAt      *time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
	Error          *SafeError
}

func (job Job) PublicStatus() JobStatus {
	if job.Status == StatusSucceeded && job.ProcessedCount == 0 && job.SkippedCount == 0 {
		return StatusNoData
	}
	return job.Status
}

func (job Job) CurrentStep() *string {
	if job.Status != StatusQueued && job.Status != StatusRunning {
		return nil
	}
	step := "chat_replay"
	return &step
}

type ChatMessage struct {
	ID                 uuid.UUID
	AuthorChannelID    *string
	AuthorDisplayName  string
	MessageText        string
	PublishedAt        time.Time
	OffsetMilliseconds int64
	MessageType        string
}

type Cursor struct {
	OffsetMilliseconds int64
	ID                 uuid.UUID
}

func EncodeCursor(cursor Cursor) string {
	payload := strconv.FormatInt(cursor.OffsetMilliseconds, 10) + ":" + cursor.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func DecodeCursor(encoded string) (*Cursor, error) {
	if encoded == "" {
		return nil, nil
	}
	if len(encoded) > 256 {
		return nil, fmt.Errorf("%w: cursor is too long", ErrInvalidRequest)
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed cursor", ErrInvalidRequest)
	}
	parts := strings.Split(string(payload), ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("%w: malformed cursor", ErrInvalidRequest)
	}
	offset, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed cursor offset", ErrInvalidRequest)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: malformed cursor ID", ErrInvalidRequest)
	}
	return &Cursor{OffsetMilliseconds: offset, ID: id}, nil
}

type MessagePage struct {
	Items      []ChatMessage
	NextCursor *string
}

func safeErrorFor(code string) *SafeError {
	switch code {
	case "CHAT_REPLAY_NOT_AVAILABLE":
		return &SafeError{Code: code, Message: "Chat replay is not available for this stream."}
	case "SOURCE_NOT_READY":
		return &SafeError{Code: code, Message: "The stream archive is not ready yet.", Retryable: true}
	case "YOUTUBE_ACCESS_DENIED":
		return &SafeError{Code: code, Message: "YouTube denied access to this stream."}
	case "YOUTUBE_RATE_LIMITED":
		return &SafeError{Code: code, Message: "YouTube temporarily limited collection requests.", Retryable: true}
	case "YTDLP_TIMEOUT":
		return &SafeError{Code: code, Message: "Chat replay collection timed out.", Retryable: true}
	case "YTDLP_PROCESS_FAILED":
		return &SafeError{Code: code, Message: "Chat replay collection failed.", Retryable: true}
	case "YTDLP_OUTPUT_CHANGED":
		return &SafeError{Code: code, Message: "The chat replay format is currently unsupported."}
	case "CHAT_IMPORT_FAILED":
		return &SafeError{Code: code, Message: "Chat messages could not be saved.", Retryable: true}
	default:
		return &SafeError{Code: "COLLECTION_FAILED", Message: "Collection failed."}
	}
}
