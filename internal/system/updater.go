package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/tomasweigenast/srouter/internal/logging"
)

const installPath = "/usr/local/bin/srouter"

var logger = logging.GetLogger("updater")

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
// then triggers a clean restart via a detached shell script.
func (uc *UpdateChecker) Install(ctx context.Context) error {
	uc.mu.RLock()
	info := uc.status.Info
	uc.mu.RUnlock()

	if info == nil || info.AssetURL == "" {
		return fmt.Errorf("no update available or asset URL missing")
	}

	logger.Info("update install: starting", "version", info.Version)

	if uc.devMode {
		logger.Info("update install: dev mode — simulating download")
		time.Sleep(2 * time.Second)
		logger.Info("update install: dev mode — simulating restart")
		return nil
	}

	// Download to a temp file in the same directory to ensure same filesystem for rename.
	dir := filepath.Dir(installPath)
	tmp, err := os.CreateTemp(dir, "srouter-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if _, statErr := os.Stat(tmpPath); statErr == nil {
			os.Remove(tmpPath)
		}
	}()

	logger.Info("update install: downloading asset", "url", info.AssetURL)

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

	n, err := io.Copy(tmp, resp.Body)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("write update: %w", err)
	}
	tmp.Close()
	logger.Info("update install: download complete", "bytes", n)

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod update: %w", err)
	}

	if err := os.Rename(tmpPath, installPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	logger.Info("update install: binary replaced", "path", installPath)

	// Spawn a fully-detached shell script (new session via Setsid) that:
	//   1. Sends SIGTERM to the current process.
	//   2. Waits until the process is gone (polls kill -0).
	//   3. Clears OpenRC ghost state and stale PID file.
	//   4. Starts the new binary via OpenRC.
	//
	// Setsid detaches the script from srouter's process group so it survives
	// after srouter exits and is not killed by the same signal delivery.
	pid := os.Getpid()
	script := fmt.Sprintf(
		"kill %d 2>/dev/null; "+
			"i=0; while kill -0 %d 2>/dev/null && [ $i -lt 30 ]; do sleep 1; i=$((i+1)); done; "+
			"rc-service srouter zap 2>/dev/null || true; "+
			"rm -f /run/srouter.pid; "+
			"rc-service srouter start",
		pid, pid,
	)
	cmd := exec.Command("sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn restart script: %w", err)
	}
	logger.Info("update install: restart script spawned", "pid", pid)
	return nil
}

// UpdateCheckLoop runs Check every interval, starting after an initial 30 s delay.
func UpdateCheckLoop(uc *UpdateChecker, interval time.Duration) {
	time.Sleep(30 * time.Second)
	for {
		status := uc.Check(context.Background())
		if status.Error != "" {
			logger.Warn("update check failed", "err", status.Error)
		} else if status.Available && status.Info != nil {
			logger.Info("update available", "version", status.Info.Version)
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
