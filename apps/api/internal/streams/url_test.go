package streams

import (
	"errors"
	"strings"
	"testing"
)

func TestParseYouTubeURL(t *testing.T) {
	const videoID = "dQw4w9WgXcQ"
	valid := []string{
		"https://www.youtube.com/watch?v=" + videoID,
		"http://youtube.com/watch?feature=share&v=" + videoID,
		"https://m.youtube.com/live/" + videoID + "?si=token",
		"https://music.youtube.com/shorts/" + videoID,
		"https://www.youtube.com/embed/" + videoID,
		"https://www.youtube-nocookie.com/embed/" + videoID,
		"https://youtu.be/" + videoID + "?t=12",
	}
	for _, input := range valid {
		t.Run(input, func(t *testing.T) {
			parsed, err := ParseYouTubeURL(input)
			if err != nil {
				t.Fatalf("parse URL: %v", err)
			}
			if parsed.VideoID != videoID {
				t.Fatalf("expected video ID %q, got %q", videoID, parsed.VideoID)
			}
			if parsed.CanonicalURL != "https://www.youtube.com/watch?v="+videoID {
				t.Fatalf("unexpected canonical URL %q", parsed.CanonicalURL)
			}
		})
	}
}

func TestParseYouTubeURLRejectsMalformedOrUntrustedInputs(t *testing.T) {
	invalid := []string{
		"",
		"dQw4w9WgXcQ",
		"ftp://youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com.evil.example/watch?v=dQw4w9WgXcQ",
		"https://user@youtube.com/watch?v=dQw4w9WgXcQ",
		"https://youtube.com:443/watch?v=dQw4w9WgXcQ",
		"https://youtube.com/watch?v=too-short",
		"https://youtube.com/watch?v=dQw4w9WgXcQ&v=aaaaaaaaaaa",
		"https://youtu.be/dQw4w9WgXcQ/extra",
		"https://youtube.com/channel/dQw4w9WgXcQ",
		"https://youtube.com/watch?v=dQw4w9WgXcQ&padding=" + strings.Repeat("a", 2048),
	}
	for _, input := range invalid {
		t.Run(input, func(t *testing.T) {
			_, err := ParseYouTubeURL(input)
			if !errors.Is(err, ErrInvalidYouTubeURL) {
				t.Fatalf("expected ErrInvalidYouTubeURL, got %v", err)
			}
		})
	}
}
