package youtube

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tsumasaki-kurageya/stream-analysis-tools/apps/api/internal/streams"
)

func TestClientFetchesEndedLiveStreamMetadata(t *testing.T) {
	const videoID = "dQw4w9WgXcQ"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/videos" {
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
		if request.URL.Query().Get("id") != videoID || request.URL.Query().Get("key") != "test-key" {
			t.Fatalf("unexpected query %q", request.URL.RawQuery)
		}
		if request.URL.Query().Get("part") != "snippet,contentDetails,liveStreamingDetails" {
			t.Fatalf("unexpected parts %q", request.URL.Query().Get("part"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(writer, `{
          "items": [{
            "id": %q,
            "snippet": {
              "title": "Archived stream",
              "channelId": "UC123",
              "channelTitle": "Channel",
              "thumbnails": {"high": {"url": "https://i.ytimg.com/high.jpg"}}
            },
            "contentDetails": {"duration": "PT1H2M3.5S"},
            "liveStreamingDetails": {
              "scheduledStartTime": "2026-08-10T08:55:00Z",
              "actualStartTime": "2026-08-10T09:00:00Z",
              "actualEndTime": "2026-08-10T10:02:03Z"
            }
          }]
        }`, videoID)
	}))
	defer server.Close()

	client, err := NewClient("test-key", server.URL, server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	fetchedAt := time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return fetchedAt }

	metadata, err := client.Fetch(context.Background(), videoID)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if metadata.Title != "Archived stream" || metadata.ChannelID != "UC123" {
		t.Fatalf("unexpected metadata: %+v", metadata)
	}
	if metadata.LifecycleStatus != streams.LifecycleEnded {
		t.Fatalf("expected ended lifecycle, got %q", metadata.LifecycleStatus)
	}
	if metadata.Duration == nil || *metadata.Duration != time.Hour+2*time.Minute+3500*time.Millisecond {
		t.Fatalf("unexpected duration: %v", metadata.Duration)
	}
	if metadata.ThumbnailURL == nil || *metadata.ThumbnailURL != "https://i.ytimg.com/high.jpg" {
		t.Fatalf("unexpected thumbnail: %v", metadata.ThumbnailURL)
	}
	if metadata.MetadataFetchedAt != fetchedAt {
		t.Fatalf("unexpected fetched time: %v", metadata.MetadataFetchedAt)
	}
}

func TestClientClassifiesUnavailableAndProviderFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		expected   error
	}{
		{name: "missing item", statusCode: http.StatusOK, body: `{"items":[]}`, expected: streams.ErrVideoUnavailable},
		{
			name:       "not a live stream",
			statusCode: http.StatusOK,
			body:       `{"items":[{"id":"dQw4w9WgXcQ","snippet":{"title":"Video","channelId":"UC1","channelTitle":"Channel"},"contentDetails":{"duration":"PT1M"}}]}`,
			expected:   streams.ErrVideoUnavailable,
		},
		{name: "quota or upstream failure", statusCode: http.StatusForbidden, body: `{}`, expected: streams.ErrMetadataProviderUnavailable},
		{name: "invalid response", statusCode: http.StatusOK, body: `{`, expected: streams.ErrMetadataProviderUnavailable},
		{
			name:       "invalid duration",
			statusCode: http.StatusOK,
			body:       `{"items":[{"id":"dQw4w9WgXcQ","snippet":{"title":"Stream","channelId":"UC1","channelTitle":"Channel"},"contentDetails":{"duration":"one minute"},"liveStreamingDetails":{}}]}`,
			expected:   streams.ErrMetadataProviderUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.statusCode)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			client, err := NewClient("test-key", server.URL, server.Client())
			if err != nil {
				t.Fatalf("new client: %v", err)
			}

			_, err = client.Fetch(context.Background(), "dQw4w9WgXcQ")
			if !errors.Is(err, test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, err)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected *time.Duration
		wantErr  bool
	}{
		{input: "", expected: nil},
		{input: "PT0S", expected: durationPointer(0)},
		{input: "PT1H2M3S", expected: durationPointer(time.Hour + 2*time.Minute + 3*time.Second)},
		{input: "P1DT2H", expected: durationPointer(26 * time.Hour)},
		{input: "PT0.25S", expected: durationPointer(250 * time.Millisecond)},
		{input: "P", wantErr: true},
		{input: "1H", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actual, err := parseDuration(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parse duration: %v", err)
			}
			if test.expected == nil {
				if actual != nil {
					t.Fatalf("expected nil, got %v", *actual)
				}
				return
			}
			if actual == nil || *actual != *test.expected {
				t.Fatalf("expected %v, got %v", *test.expected, actual)
			}
		})
	}
}

func durationPointer(value time.Duration) *time.Duration {
	return &value
}
