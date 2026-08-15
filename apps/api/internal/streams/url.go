package streams

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var youTubeVideoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

const maxYouTubeURLLength = 2048

type YouTubeURL struct {
	VideoID      string
	CanonicalURL string
}

func ParseYouTubeURL(raw string) (YouTubeURL, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) == 0 || len(raw) > maxYouTubeURLLength {
		return YouTubeURL{}, ErrInvalidYouTubeURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return YouTubeURL{}, ErrInvalidYouTubeURL
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return YouTubeURL{}, ErrInvalidYouTubeURL
	}
	if parsed.User != nil || parsed.Port() != "" {
		return YouTubeURL{}, ErrInvalidYouTubeURL
	}

	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	var videoID string
	switch host {
	case "youtu.be":
		videoID = singlePathSegment(parsed.Path)
	case "youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com":
		videoID = videoIDFromYouTubePath(parsed)
	case "youtube-nocookie.com", "www.youtube-nocookie.com":
		videoID = videoIDFromEmbedPath(parsed.Path)
	default:
		return YouTubeURL{}, ErrInvalidYouTubeURL
	}

	if !youTubeVideoIDPattern.MatchString(videoID) {
		return YouTubeURL{}, ErrInvalidYouTubeURL
	}
	return YouTubeURL{
		VideoID:      videoID,
		CanonicalURL: fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
	}, nil
}

func videoIDFromYouTubePath(parsed *url.URL) string {
	path := strings.Trim(parsed.EscapedPath(), "/")
	if path == "watch" {
		values := parsed.Query()["v"]
		if len(values) == 1 {
			return values[0]
		}
		return ""
	}

	parts := strings.Split(path, "/")
	if len(parts) == 2 {
		switch parts[0] {
		case "live", "shorts", "embed":
			videoID, err := url.PathUnescape(parts[1])
			if err == nil {
				return videoID
			}
		}
	}
	return ""
}

func videoIDFromEmbedPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[0] != "embed" {
		return ""
	}
	videoID, err := url.PathUnescape(parts[1])
	if err != nil {
		return ""
	}
	return videoID
}

func singlePathSegment(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 1 {
		return ""
	}
	videoID, err := url.PathUnescape(parts[0])
	if err != nil {
		return ""
	}
	return videoID
}
