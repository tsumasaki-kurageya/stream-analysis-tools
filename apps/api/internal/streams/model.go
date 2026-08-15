package streams

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidStream        = errors.New("invalid stream")
	ErrNotFound             = errors.New("stream not found")
	ErrYouTubeVideoIDExists = errors.New("YouTube video ID already exists")
)

type LifecycleStatus string

const (
	LifecycleUnknown     LifecycleStatus = "unknown"
	LifecycleScheduled   LifecycleStatus = "scheduled"
	LifecycleLive        LifecycleStatus = "live"
	LifecycleEnded       LifecycleStatus = "ended"
	LifecycleUnavailable LifecycleStatus = "unavailable"
)

type Stream struct {
	ID                uuid.UUID
	YouTubeVideoID    string
	CanonicalURL      string
	Title             string
	ChannelID         string
	ChannelTitle      string
	ThumbnailURL      *string
	ScheduledStartAt  *time.Time
	ActualStartAt     *time.Time
	ActualEndAt       *time.Time
	Duration          *time.Duration
	LifecycleStatus   LifecycleStatus
	MetadataFetchedAt time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Metadata struct {
	YouTubeVideoID    string
	CanonicalURL      string
	Title             string
	ChannelID         string
	ChannelTitle      string
	ThumbnailURL      *string
	ScheduledStartAt  *time.Time
	ActualStartAt     *time.Time
	ActualEndAt       *time.Time
	Duration          *time.Duration
	LifecycleStatus   LifecycleStatus
	MetadataFetchedAt time.Time
}

type ListOptions struct {
	Limit  int
	Offset int
}
