package updater

import (
	"testing"
)

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch string
		expected     string
		expectErr    bool
	}{
		{"windows", "amd64", "courseforge-windows-amd64.exe", false},
		{"windows", "arm64", "courseforge-windows-arm64.exe", false},
		{"linux", "amd64", "courseforge-linux-amd64", false},
		{"linux", "arm64", "courseforge-linux-arm64", false},
		{"darwin", "amd64", "courseforge-macos-amd64", false},
		{"darwin", "arm64", "courseforge-macos-arm64", false},
		{"freebsd", "amd64", "", true},
		{"linux", "386", "", true},
	}

	for _, tt := range tests {
		got, err := AssetName(tt.goos, tt.goarch)
		if tt.expectErr && err == nil {
			t.Errorf("AssetName(%q, %q) expected error, got nil", tt.goos, tt.goarch)
		}
		if !tt.expectErr && err != nil {
			t.Errorf("AssetName(%q, %q) unexpected error: %v", tt.goos, tt.goarch, err)
		}
		if got != tt.expected {
			t.Errorf("AssetName(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.expected)
		}
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"v0.4.1", "v0.4.2", true},
		{"v0.4.1", "v0.5.0", true},
		{"v0.4.1", "v1.0.0", true},
		{"v0.4.2", "v0.4.1", false},
		{"v0.4.1", "v0.4.1", false},
		{"0.4.1", "0.4.2", true},
		{"v0.4.1-alpha", "v0.4.2", true},
		{"v0.4.1+build1", "v0.4.2", true},
	}

	for _, tt := range tests {
		got := IsNewer(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024 * 39, "39.0 MB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
