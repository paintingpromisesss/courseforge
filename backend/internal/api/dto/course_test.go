package dto

import (
	"testing"
)

func TestBuildVideoSources(t *testing.T) {
	tests := []struct {
		name       string
		primaryURL string
		videos     map[string]string
		wantCount  int
		wantMain   string
		wantTop    int
	}{
		{
			name:       "empty inputs",
			primaryURL: "",
			videos:     nil,
			wantCount:  0,
			wantMain:   "",
			wantTop:    0,
		},
		{
			name:       "only primaryURL (legacy single video)",
			primaryURL: "https://archive.org/video.mp4",
			videos:     nil,
			wantCount:  1,
			wantMain:   "https://archive.org/video.mp4",
			wantTop:    1080,
		},
		{
			name:       "multi qualities with suffixes",
			primaryURL: "",
			videos: map[string]string{
				"480p":  "https://archive.org/video_480.mp4",
				"1080p": "https://archive.org/video_1080.mp4",
				"720":   "https://archive.org/video_720.mp4",
			},
			wantCount: 3,
			wantMain:  "https://archive.org/video_1080.mp4",
			wantTop:   1080,
		},
		{
			name:       "cyrillic suffix 'р'",
			primaryURL: "",
			videos: map[string]string{
				"720р": "https://archive.org/video_720.mp4",
				"360p": "https://archive.org/video_360.mp4",
			},
			wantCount: 2,
			wantMain:  "https://archive.org/video_720.mp4",
			wantTop:   720,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sources, main := BuildVideoSources(tt.primaryURL, tt.videos)
			if len(sources) != tt.wantCount {
				t.Fatalf("expected %d sources, got %d", tt.wantCount, len(sources))
			}
			if main != tt.wantMain {
				t.Errorf("expected main URL %q, got %q", tt.wantMain, main)
			}
			if tt.wantCount > 0 && sources[0].Size != tt.wantTop {
				t.Errorf("expected top source size %d, got %d", tt.wantTop, sources[0].Size)
			}
			// Verify sorted descending
			for i := 1; i < len(sources); i++ {
				if sources[i].Size > sources[i-1].Size {
					t.Errorf("sources not sorted descending: %d before %d", sources[i-1].Size, sources[i].Size)
				}
			}
		})
	}
}
