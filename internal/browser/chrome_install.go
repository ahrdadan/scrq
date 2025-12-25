package browser

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-rod/rod/lib/launcher"
)

// GetChromeDownloadDir returns the directory where Chrome will be downloaded
func GetChromeDownloadDir() string {
	// Try to get user cache directory first
	cacheDir, err := os.UserCacheDir()
	if err == nil {
		return filepath.Join(cacheDir, "rod", "browser")
	}

	// Fallback to temp directory
	return filepath.Join(os.TempDir(), "rod", "browser")
}

// InstallChrome downloads a Chromium build for the current OS/arch as portable binary.
// No package manager (apt/dnf/yum) is used - pure binary download.
func InstallChrome(ctx context.Context, revision int) (string, error) {
	log.Printf("Installing Chrome browser (OS: %s, Arch: %s)", runtime.GOOS, runtime.GOARCH)

	// Create browser download directory
	downloadDir := GetChromeDownloadDir()
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create browser directory: %w", err)
	}

	// Use go-rod's built-in downloader which downloads portable Chrome/Chromium
	// This does NOT use apt or any package manager - it downloads the binary directly
	downloader := launcher.NewBrowser()
	downloader.Context = ctx

	// Set custom download directory
	downloader.RootDir = downloadDir

	if revision > 0 {
		downloader.Revision = revision
	}

	log.Printf("Downloading Chrome to: %s", downloadDir)

	path, err := downloader.Get()
	if err != nil {
		return "", fmt.Errorf("failed to download chrome: %w", err)
	}

	// Ensure the binary is executable (for Linux/macOS)
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(path); err == nil {
			if err := ensureChromeExecutable(path, info); err != nil {
				log.Printf("Warning: Failed to set executable permissions: %v", err)
			}
		}
	}

	log.Printf("Chrome browser installed at: %s", path)
	return path, nil
}

// ensureChromeExecutable ensures the Chrome binary has executable permissions
func ensureChromeExecutable(path string, info os.FileInfo) error {
	mode := info.Mode()
	if mode&0111 != 0 {
		return nil // Already executable
	}

	newMode := mode | 0755
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", path, err)
	}
	log.Printf("Set executable permissions on %s", path)
	return nil
}

// GetChromePath returns the path to an existing Chrome installation if available
func GetChromePath() (string, bool) {
	downloadDir := GetChromeDownloadDir()

	// Check if Chrome is already downloaded
	downloader := launcher.NewBrowser()
	downloader.RootDir = downloadDir

	// Try to find existing Chrome binary using BinPath
	chromePath := downloader.BinPath()
	if chromePath != "" {
		if _, err := os.Stat(chromePath); err == nil {
			return chromePath, true
		}
	}

	return "", false
}
