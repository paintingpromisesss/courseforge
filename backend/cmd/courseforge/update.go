package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const updateRepo = "paintingpromisesss/courseforge"

var (
	updateAPIClient  = &http.Client{Timeout: 30 * time.Second}
	updateDownClient = &http.Client{Timeout: 5 * time.Minute}
)

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	tag := fs.String("tag", "latest", "release tag to install")
	force := fs.Bool("force", false, "reinstall even if already up to date")
	fs.Parse(args)

	release, err := fetchRelease(*tag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "courseforge update: %v\n", err)
		os.Exit(1)
	}

	if !*force && release.TagName == version {
		fmt.Printf("courseforge %s is already up to date.\n", version)
		return
	}

	assetName, err := updateAssetName()
	if err != nil {
		fmt.Fprintf(os.Stderr, "courseforge update: %v\n", err)
		os.Exit(1)
	}

	var downloadURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			downloadURL = a.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		fmt.Fprintf(os.Stderr, "courseforge update: no %s asset in release %s\n", assetName, release.TagName)
		os.Exit(1)
	}

	exePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "courseforge update: %v\n", err)
		os.Exit(1)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	fmt.Printf("Updating courseforge %s -> %s...\n", version, release.TagName)
	if err := replaceExecutable(exePath, downloadURL); err != nil {
		fmt.Fprintf(os.Stderr, "courseforge update: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated to %s. Run: courseforge version\n", release.TagName)
}

func fetchRelease(tag string) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", updateRepo)
	if tag != "" && tag != "latest" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", updateRepo, tag)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "courseforge-update")

	resp, err := updateAPIClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch release info: unexpected status %s", resp.Status)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release info: %w", err)
	}
	return &rel, nil
}

// updateAssetName maps the running binary's OS/arch to the release asset
// name produced by .github/workflows/release.yml.
func updateAssetName() (string, error) {
	var osName string
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
	case "darwin":
		osName = "macos"
	case "linux":
		osName = "linux"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	name := fmt.Sprintf("courseforge-%s-%s", osName, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name, nil
}

// replaceExecutable downloads url into exePath's directory and swaps it in
// for the running binary. The old file is moved aside rather than deleted
// in place: on Windows a running .exe can be renamed but not overwritten in
// place, and moving first (instead of write-then-rename over the original)
// means a failed download never leaves the install without a binary.
func replaceExecutable(exePath, url string) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, "courseforge-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the temp file is renamed into place below

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		tmp.Close()
		return err
	}
	req.Header.Set("User-Agent", "courseforge-update")

	resp, err := updateDownClient.Do(req)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmp.Close()
		return fmt.Errorf("download: unexpected status %s", resp.Status)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}

	old := exePath + ".old"
	os.Remove(old) // best-effort cleanup from a previous update
	if err := os.Rename(exePath, old); err != nil {
		return fmt.Errorf("move current binary aside: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		os.Rename(old, exePath) // restore so the install isn't left without a binary
		return fmt.Errorf("install new binary: %w", err)
	}
	os.Remove(old) // best-effort; may still be locked on Windows until this process exits
	return nil
}
