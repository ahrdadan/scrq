package nats

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// NATSVersion is the version of NATS server to download
	NATSVersion = "2.10.24"
)

// GetDownloadURL returns the download URL for NATS server based on OS/arch
func GetDownloadURL() (string, error) {
	var osName, arch string

	switch runtime.GOOS {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	switch runtime.GOARCH {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	ext := "zip"
	if runtime.GOOS == "linux" {
		ext = "zip"
	}

	return fmt.Sprintf(
		"https://github.com/nats-io/nats-server/releases/download/v%s/nats-server-v%s-%s-%s.%s",
		NATSVersion, NATSVersion, osName, arch, ext,
	), nil
}

// EnsureNATSBinary ensures the NATS server binary is available
func EnsureNATSBinary(binPath string, autoDL bool) (string, error) {
	// Check if binary already exists
	if _, err := os.Stat(binPath); err == nil {
		log.Printf("NATS server binary found at %s", binPath)
		return binPath, nil
	}

	if !autoDL {
		return "", fmt.Errorf("NATS server binary not found at %s and auto-download is disabled", binPath)
	}

	log.Printf("NATS server binary not found, downloading...")

	downloadURL, err := GetDownloadURL()
	if err != nil {
		return "", fmt.Errorf("failed to get download URL: %w", err)
	}

	// Create directory for binary
	binDir := filepath.Dir(binPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", binDir, err)
	}

	// Download to temp file
	tmpFile, err := os.CreateTemp("", "nats-server-*.zip")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	log.Printf("Downloading NATS server from %s", downloadURL)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download NATS server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download NATS server: HTTP %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save NATS server: %w", err)
	}

	tmpFile.Close()

	// Extract the binary
	if err := extractNATSBinary(tmpFile.Name(), binPath); err != nil {
		return "", fmt.Errorf("failed to extract NATS server: %w", err)
	}

	// Make executable
	if err := os.Chmod(binPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make NATS server executable: %w", err)
	}

	log.Printf("NATS server downloaded and installed at %s", binPath)
	return binPath, nil
}

// extractNATSBinary extracts the nats-server binary from a zip file
func extractNATSBinary(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	binaryName := "nats-server"
	if runtime.GOOS == "windows" {
		binaryName = "nats-server.exe"
	}

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, binaryName) {
			rc, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open file in zip: %w", err)
			}
			defer rc.Close()

			out, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %w", err)
			}
			defer out.Close()

			if _, err := io.Copy(out, rc); err != nil {
				return fmt.Errorf("failed to copy binary: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("nats-server binary not found in zip")
}
