package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRepo = "paintingpromisesss/courseforge"
	cacheTTL    = 1 * time.Hour
)

// ReleaseInfo holds metadata for a GitHub release asset.
type ReleaseInfo struct {
	TagName            string    `json:"tag_name"`
	Name               string    `json:"name"`
	Body               string    `json:"body"`
	PublishedAt        time.Time `json:"published_at"`
	HTMLURL            string    `json:"html_url"`
	AssetName          string    `json:"asset_name"`
	BrowserDownloadURL string    `json:"browser_download_url"`
	AssetSize          int64     `json:"asset_size"`
}

// CheckResult represents the outcome of checking for updates.
type CheckResult struct {
	CurrentVersion  string       `json:"current_version"`
	LatestVersion   string       `json:"latest_version"`
	UpdateAvailable bool         `json:"update_available"`
	IsDev           bool         `json:"is_dev"`
	CheckedAt       time.Time    `json:"checked_at"`
	Release         *ReleaseInfo `json:"release,omitempty"`
}

// Status describes the current state of an ongoing or completed update.
type Status struct {
	State    string `json:"state"` // idle | checking | downloading | ready_restart | error
	Progress int    `json:"progress"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

type cachedCheck struct {
	ETag   string       `json:"etag"`
	Result *CheckResult `json:"result"`
}

// Updater manages checking and applying single-binary updates.
type Updater struct {
	mu             sync.RWMutex
	currentVersion string
	dataDir        string
	repo           string
	cacheFile      string
	lastCheck      *CheckResult
	lastETag       string
	status         Status
	apiClient      *http.Client
	downClient     *http.Client
}

// New creates an Updater instance.
func New(currentVersion, dataDir string) *Updater {
	if currentVersion == "" {
		currentVersion = "dev"
	}
	cacheFile := ""
	if dataDir != "" {
		cacheFile = filepath.Join(dataDir, "update_cache.json")
	}

	u := &Updater{
		currentVersion: currentVersion,
		dataDir:        dataDir,
		repo:           defaultRepo,
		cacheFile:      cacheFile,
		status:         Status{State: "idle"},
		apiClient:      &http.Client{Timeout: 30 * time.Second},
		downClient:     &http.Client{Timeout: 10 * time.Minute},
	}
	u.loadCache()
	return u
}

// CurrentVersion returns the active application version.
func (u *Updater) CurrentVersion() string {
	return u.currentVersion
}

// GetStatus returns a snapshot of the current update state.
func (u *Updater) GetStatus() Status {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.status
}

// LastCheck returns the most recent check result, if any.
func (u *Updater) LastCheck() *CheckResult {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.lastCheck
}

// Check queries GitHub Releases for the latest update, respecting ETag and TTL caching.
func (u *Updater) Check(ctx context.Context, force bool) (*CheckResult, error) {
	return u.CheckTag(ctx, "latest", force)
}

// CheckTag queries GitHub Releases for a specific tag or "latest".
func (u *Updater) CheckTag(ctx context.Context, tag string, force bool) (*CheckResult, error) {
	if tag == "" {
		tag = "latest"
	}
	isLatest := tag == "latest"

	u.mu.Lock()
	now := time.Now()
	if isLatest && !force && u.lastCheck != nil && now.Sub(u.lastCheck.CheckedAt) < cacheTTL {
		res := u.lastCheck
		u.mu.Unlock()
		return res, nil
	}
	etag := u.lastETag
	u.mu.Unlock()

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.repo)
	if !isLatest {
		url = fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", u.repo, tag)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "courseforge-update")
	if isLatest && etag != "" && !force {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := u.apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check release: %w", err)
	}
	defer resp.Body.Close()

	u.mu.Lock()
	defer u.mu.Unlock()

	if resp.StatusCode == http.StatusNotModified && u.lastCheck != nil {
		u.lastCheck.CheckedAt = now
		u.saveCacheLocked()
		return u.lastCheck, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %s", resp.Status)
	}

	var ghRel struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		Body        string    `json:"body"`
		PublishedAt time.Time `json:"published_at"`
		HTMLURL     string    `json:"html_url"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghRel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	assetName, err := AssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}

	var matchedAssetURL string
	var matchedAssetSize int64
	for _, a := range ghRel.Assets {
		if a.Name == assetName {
			matchedAssetURL = a.BrowserDownloadURL
			matchedAssetSize = a.Size
			break
		}
	}

	isDev := u.currentVersion == "dev" || strings.HasPrefix(u.currentVersion, "dev-")
	updateAvailable := false
	if matchedAssetURL != "" {
		if isDev {
			// In dev mode, any published release can be offered as an update
			updateAvailable = ghRel.TagName != ""
		} else {
			updateAvailable = IsNewer(u.currentVersion, ghRel.TagName)
		}
	}

	result := &CheckResult{
		CurrentVersion:  u.currentVersion,
		LatestVersion:   ghRel.TagName,
		UpdateAvailable: updateAvailable,
		IsDev:           isDev,
		CheckedAt:       now,
		Release: &ReleaseInfo{
			TagName:            ghRel.TagName,
			Name:               ghRel.Name,
			Body:               ghRel.Body,
			PublishedAt:        ghRel.PublishedAt,
			HTMLURL:            ghRel.HTMLURL,
			AssetName:          assetName,
			BrowserDownloadURL: matchedAssetURL,
			AssetSize:          matchedAssetSize,
		},
	}

	u.lastCheck = result
	if newETag := resp.Header.Get("ETag"); newETag != "" {
		u.lastETag = newETag
	}
	u.saveCacheLocked()

	return result, nil
}

// StartUpdate triggers the background download and atomic binary replacement.
func (u *Updater) StartUpdate(ctx context.Context) error {
	u.mu.Lock()
	if u.status.State == "downloading" {
		u.mu.Unlock()
		return errors.New("update is already in progress")
	}

	check := u.lastCheck
	if check == nil || check.Release == nil || check.Release.BrowserDownloadURL == "" {
		u.mu.Unlock()
		return errors.New("no update available or release asset not found")
	}

	downloadURL := check.Release.BrowserDownloadURL
	expectedSize := check.Release.AssetSize
	targetTag := check.Release.TagName

	u.status = Status{
		State:    "downloading",
		Progress: 0,
		Message:  fmt.Sprintf("Загрузка обновления %s...", targetTag),
	}
	u.mu.Unlock()

	go func() {
		err := u.performUpdate(ctx, downloadURL, expectedSize)
		u.mu.Lock()
		defer u.mu.Unlock()
		if err != nil {
			u.status = Status{
				State: "error",
				Error: err.Error(),
			}
		} else {
			u.status = Status{
				State:    "ready_restart",
				Progress: 100,
				Message:  fmt.Sprintf("Обновление до %s установлено. Требуется перезапуск.", targetTag),
			}
		}
	}()

	return nil
}

func (u *Updater) performUpdate(ctx context.Context, downloadURL string, expectedSize int64) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, "courseforge-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		tmp.Close()
		return err
	}
	req.Header.Set("User-Agent", "courseforge-update")

	resp, err := u.downClient.Do(req)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmp.Close()
		return fmt.Errorf("download returned status %s", resp.Status)
	}

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		totalSize = expectedSize
	}

	buf := make([]byte, 32*1024)
	var downloaded int64
	lastReport := time.Now()

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			if _, wErr := tmp.Write(buf[:n]); wErr != nil {
				tmp.Close()
				return fmt.Errorf("write temp binary: %w", wErr)
			}
			downloaded += int64(n)
			if totalSize > 0 && (time.Since(lastReport) > 200*time.Millisecond || rErr != nil) {
				lastReport = time.Now()
				percent := int((downloaded * 100) / totalSize)
				if percent > 99 && rErr == nil {
					percent = 99
				}
				u.mu.Lock()
				if u.status.State == "downloading" {
					u.status.Progress = percent
					u.status.Message = fmt.Sprintf("Скачивание: %d%% (%s / %s)",
						percent, formatBytes(downloaded), formatBytes(totalSize))
				}
				u.mu.Unlock()
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			tmp.Close()
			return fmt.Errorf("read body: %w", rErr)
		}
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod binary: %w", err)
	}

	// Windows locked-executable replacement technique:
	// Running executables cannot be written to or deleted, but NTFS allows renaming.
	old := exePath + ".old"
	_ = os.Remove(old) // Best-effort cleanup
	if err := os.Rename(exePath, old); err != nil {
		return fmt.Errorf("move current binary aside: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		_ = os.Rename(old, exePath) // rollback
		return fmt.Errorf("install new binary: %w", err)
	}
	_ = os.Remove(old) // Best-effort cleanup (may still be held until exit on Windows)

	return nil
}

// Restart spawns a new process of the current binary and terminates this process.
func (u *Updater) Restart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}

	cmd := exec.Command(exePath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	setupDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn new process: %w", err)
	}

	// Exit current process shortly after
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}

// UpdateCLI performs synchronous CLI update, printing progress to stdout/stderr.
func (u *Updater) UpdateCLI(stdout, stderr io.Writer, tag string, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	check, err := u.CheckTag(ctx, tag, true)
	if err != nil {
		return err
	}

	if !force && check.LatestVersion == u.currentVersion {
		fmt.Fprintf(stdout, "courseforge %s is already up to date.\n", u.currentVersion)
		return nil
	}

	if check.Release == nil || check.Release.BrowserDownloadURL == "" {
		assetName := "matching"
		if check.Release != nil && check.Release.AssetName != "" {
			assetName = check.Release.AssetName
		}
		return fmt.Errorf("no %s asset in release %s", assetName, check.LatestVersion)
	}

	fmt.Fprintf(stdout, "Updating courseforge %s -> %s...\n", u.currentVersion, check.LatestVersion)
	if err := u.performUpdate(ctx, check.Release.BrowserDownloadURL, check.Release.AssetSize); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Updated to %s. Run: courseforge version\n", check.LatestVersion)
	return nil
}

// CleanupOldBinary removes leftover `<exe>.old` files from past updates.
func CleanupOldBinary() {
	if exePath, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
			exePath = resolved
		}
		_ = os.Remove(exePath + ".old")
	}
}

// AssetName returns the expected release binary name for a given OS and architecture.
func AssetName(goos, goarch string) (string, error) {
	var osName string
	switch goos {
	case "windows":
		osName = "windows"
	case "darwin":
		osName = "macos"
	case "linux":
		osName = "linux"
	default:
		return "", fmt.Errorf("unsupported OS: %s", goos)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}

	name := fmt.Sprintf("courseforge-%s-%s", osName, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name, nil
}

// IsNewer compares two version strings (e.g. "v0.4.1", "v0.4.2" or "0.4.1").
// Returns true if latest is strictly newer than current.
func IsNewer(current, latest string) bool {
	cParts := parseSemver(current)
	lParts := parseSemver(latest)

	for i := range 3 {
		if lParts[i] > cParts[i] {
			return true
		}
		if lParts[i] < cParts[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	var res [3]int
	v = strings.TrimPrefix(v, "v")
	// Strip anything after '-' or '+' (prerelease/build metadata)
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	for i, part := range parts {
		if i >= 3 {
			break
		}
		if n, err := strconv.Atoi(part); err == nil {
			res[i] = n
		}
	}
	return res
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (u *Updater) loadCache() {
	if u.cacheFile == "" {
		return
	}
	data, err := os.ReadFile(u.cacheFile)
	if err != nil {
		return
	}
	var cached cachedCheck
	if err := json.Unmarshal(data, &cached); err == nil && cached.Result != nil {
		u.lastETag = cached.ETag
		u.lastCheck = cached.Result
	}
}

func (u *Updater) saveCacheLocked() {
	if u.cacheFile == "" || u.lastCheck == nil {
		return
	}
	cached := cachedCheck{
		ETag:   u.lastETag,
		Result: u.lastCheck,
	}
	data, err := json.MarshalIndent(cached, "", "  ")
	if err == nil {
		_ = os.WriteFile(u.cacheFile, data, 0644)
	}
}
