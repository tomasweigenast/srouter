package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const githubRepo = "tomasweigenast/srouter"
const updateAssetName = "srouter-linux"

type UpdateInfo struct {
	Version      string
	PublishedAt  time.Time
	ReleaseNotes string
	AssetURL     string
	HTMLURL      string
}

type UpdateStatus struct {
	Checked   time.Time
	Available bool
	Info      *UpdateInfo
	Error     string
}

type UpdateChecker struct {
	devMode bool
	mu      sync.RWMutex
	status  UpdateStatus
}

func NewUpdateChecker(devMode bool) *UpdateChecker {
	return &UpdateChecker{devMode: devMode}
}

// Status returns the last cached update status without fetching.
func (uc *UpdateChecker) Status() UpdateStatus {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return uc.status
}

// Check fetches the latest GitHub release and returns the update status.
// It caches the result; subsequent calls within 5 minutes return the cache.
func (uc *UpdateChecker) Check(ctx context.Context) UpdateStatus {
	if uc.devMode {
		status := UpdateStatus{
			Checked:   time.Now(),
			Available: true,
			Info: &UpdateInfo{
				Version:      "v99.0.0",
				PublishedAt:  time.Now().Add(-48 * time.Hour),
				ReleaseNotes: "Mock release for development testing.",
				AssetURL:     "",
				HTMLURL:      "https://github.com/" + githubRepo + "/releases/tag/v99.0.0",
			},
		}
		uc.mu.Lock()
		uc.status = status
		uc.mu.Unlock()
		return status
	}

	uc.mu.RLock()
	cached := uc.status
	uc.mu.RUnlock()
	if !cached.Checked.IsZero() && time.Since(cached.Checked) < 5*time.Minute {
		return cached
	}

	status := uc.fetchLatestRelease(ctx)
	uc.mu.Lock()
	uc.status = status
	uc.mu.Unlock()
	return status
}

// Install downloads the latest release asset and atomically replaces the running binary,
// then signals the service to restart.
func (uc *UpdateChecker) Install(ctx context.Context) error {
	if uc.devMode {
		time.Sleep(2 * time.Second)
		return nil
	}

	uc.mu.RLock()
	info := uc.status.Info
	uc.mu.RUnlock()

	if info == nil || info.AssetURL == "" {
		return fmt.Errorf("no update available or asset URL missing")
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	// Download to a temp file in the same directory to ensure same filesystem for rename.
	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, "srouter-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up temp file if something goes wrong after creation.
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.AssetURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download update: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod update: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	// Restart service. rc-service sends SIGTERM; OpenRC starts the new binary.
	if err := exec.Command("rc-service", "srouter", "restart").Start(); err != nil {
		return fmt.Errorf("restart service: %w", err)
	}
	return nil
}

// UpdateCheckLoop runs Check every interval, starting after an initial 30 s delay.
func UpdateCheckLoop(uc *UpdateChecker, interval time.Duration) {
	time.Sleep(30 * time.Second)
	for {
		status := uc.Check(context.Background())
		if status.Error != "" {
			slog.Warn("update check failed", "err", status.Error)
		} else if status.Available && status.Info != nil {
			slog.Info("update available", "version", status.Info.Version)
		}
		time.Sleep(interval)
	}
}

type githubRelease struct {
	TagName     string         `json:"tag_name"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Body        string         `json:"body"`
	Assets      []githubAsset  `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (uc *UpdateChecker) fetchLatestRelease(ctx context.Context) UpdateStatus {
	url := "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return UpdateStatus{Checked: time.Now(), Error: err.Error()}
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "srouter/"+AppVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UpdateStatus{Checked: time.Now(), Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases published yet.
		return UpdateStatus{Checked: time.Now()}
	}
	if resp.StatusCode != http.StatusOK {
		return UpdateStatus{Checked: time.Now(), Error: fmt.Sprintf("GitHub API: HTTP %d", resp.StatusCode)}
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateStatus{Checked: time.Now(), Error: fmt.Sprintf("parse response: %s", err)}
	}

	var assetURL string
	for _, a := range release.Assets {
		if a.Name == updateAssetName {
			assetURL = a.BrowserDownloadURL
			break
		}
	}

	info := &UpdateInfo{
		Version:      release.TagName,
		PublishedAt:  release.PublishedAt,
		ReleaseNotes: release.Body,
		AssetURL:     assetURL,
		HTMLURL:      release.HTMLURL,
	}
	available := AppVersion != "dev" && AppVersion != "" && release.TagName != AppVersion

	return UpdateStatus{
		Checked:   time.Now(),
		Available: available,
		Info:      info,
	}
}
