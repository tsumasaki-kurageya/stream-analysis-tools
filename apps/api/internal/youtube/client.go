package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

const (
	DefaultBaseURL  = "https://www.googleapis.com/youtube/v3"
	maxResponseSize = 1 << 20
)

var iso8601DurationPattern = regexp.MustCompile(
	`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?)?$`,
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	now        func() time.Time
}

var _ streams.MetadataProvider = (*Client)(nil)

func NewClient(apiKey string, baseURL string, httpClient *http.Client) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil ||
		(parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return nil, errors.New("invalid YouTube API base URL")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    baseURL,
		httpClient: httpClient,
		now:        time.Now,
	}, nil
}

func (client *Client) Fetch(ctx context.Context, videoID string) (streams.Metadata, error) {
	if client.apiKey == "" {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}
	requestURL, err := url.Parse(client.baseURL + "/videos")
	if err != nil {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}
	query := requestURL.Query()
	query.Set("part", "snippet,contentDetails,liveStreamingDetails")
	query.Set("id", videoID)
	query.Set("key", client.apiKey)
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}
	request.Header.Set("Accept", "application/json")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}

	var payload videoListResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize))
	if err := decoder.Decode(&payload); err != nil {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}
	if len(payload.Items) == 0 {
		return streams.Metadata{}, streams.ErrVideoUnavailable
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != videoID {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}

	return client.metadataFromVideo(payload.Items[0])
}

type videoListResponse struct {
	Items []videoResource `json:"items"`
}

type videoResource struct {
	ID      string `json:"id"`
	Snippet struct {
		Title        string               `json:"title"`
		ChannelID    string               `json:"channelId"`
		ChannelTitle string               `json:"channelTitle"`
		Thumbnails   map[string]thumbnail `json:"thumbnails"`
	} `json:"snippet"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
	LiveStreamingDetails *struct {
		ScheduledStartTime *time.Time `json:"scheduledStartTime"`
		ActualStartTime    *time.Time `json:"actualStartTime"`
		ActualEndTime      *time.Time `json:"actualEndTime"`
	} `json:"liveStreamingDetails"`
}

type thumbnail struct {
	URL string `json:"url"`
}

func (client *Client) metadataFromVideo(video videoResource) (streams.Metadata, error) {
	if video.LiveStreamingDetails == nil {
		return streams.Metadata{}, streams.ErrVideoUnavailable
	}
	if strings.TrimSpace(video.Snippet.Title) == "" ||
		strings.TrimSpace(video.Snippet.ChannelID) == "" ||
		strings.TrimSpace(video.Snippet.ChannelTitle) == "" {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}

	duration, err := parseDuration(video.ContentDetails.Duration)
	if err != nil {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}
	thumbnailURL, err := preferredThumbnail(video.Snippet.Thumbnails)
	if err != nil {
		return streams.Metadata{}, streams.ErrMetadataProviderUnavailable
	}

	scheduledStart := utcTime(video.LiveStreamingDetails.ScheduledStartTime)
	actualStart := utcTime(video.LiveStreamingDetails.ActualStartTime)
	actualEnd := utcTime(video.LiveStreamingDetails.ActualEndTime)

	return streams.Metadata{
		Title:             strings.TrimSpace(video.Snippet.Title),
		ChannelID:         strings.TrimSpace(video.Snippet.ChannelID),
		ChannelTitle:      strings.TrimSpace(video.Snippet.ChannelTitle),
		ThumbnailURL:      thumbnailURL,
		ScheduledStartAt:  scheduledStart,
		ActualStartAt:     actualStart,
		ActualEndAt:       actualEnd,
		Duration:          duration,
		LifecycleStatus:   lifecycleStatus(scheduledStart, actualStart, actualEnd),
		MetadataFetchedAt: client.now().UTC(),
	}, nil
}

func preferredThumbnail(thumbnails map[string]thumbnail) (*string, error) {
	for _, name := range []string{"maxres", "standard", "high", "medium", "default"} {
		candidate := strings.TrimSpace(thumbnails[name].URL)
		if candidate == "" {
			continue
		}
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return nil, errors.New("invalid thumbnail URL")
		}
		return &candidate, nil
	}
	return nil, nil
}

func lifecycleStatus(scheduledStart, actualStart, actualEnd *time.Time) streams.LifecycleStatus {
	switch {
	case actualEnd != nil:
		return streams.LifecycleEnded
	case actualStart != nil:
		return streams.LifecycleLive
	case scheduledStart != nil:
		return streams.LifecycleScheduled
	default:
		return streams.LifecycleUnknown
	}
}

func utcTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}

func parseDuration(value string) (*time.Duration, error) {
	if value == "" {
		return nil, nil
	}
	matches := iso8601DurationPattern.FindStringSubmatch(value)
	if matches == nil || strings.Join(matches[1:], "") == "" {
		return nil, fmt.Errorf("invalid ISO 8601 duration")
	}

	units := []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second}
	var total time.Duration
	for index, raw := range matches[1:] {
		if raw == "" {
			continue
		}
		amount, err := strconv.ParseFloat(raw, 64)
		if err != nil || amount < 0 {
			return nil, fmt.Errorf("invalid ISO 8601 duration")
		}
		component := time.Duration(amount * float64(units[index]))
		if component < 0 || total > time.Duration(1<<63-1)-component {
			return nil, fmt.Errorf("ISO 8601 duration overflows")
		}
		total += component
	}
	return &total, nil
}
