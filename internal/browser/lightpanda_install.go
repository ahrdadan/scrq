package browser

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const (
	// LightpandaDownloadURL is the URL to download Lightpanda browser
	LightpandaDownloadURL = "https://github.com/lightpanda-io/browser/releases/download/nightly/lightpanda-x86_64-linux"
)

// EnsureLightpandaBinary ensures the Lightpanda browser binary is available
// Returns the path to the binary and whether Lightpanda is available
func EnsureLightpandaBinary() (string, bool, error) {
	// Only supported on Linux
	if runtime.GOOS != "linux" {
		log.Printf("⚠️  Warning: Lightpanda browser only supports Linux, current OS: %s", runtime.GOOS)
		log.Printf("⚠️  Lightpanda-related APIs will not be available")
		return "", false, nil
	}

	// Get the executable directory
	execPath, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	execDir := filepath.Dir(execPath)

	// Possible binary names based on OS
	binaryNames := []string{
		"lightpanda-x86_64-linux",
		"lightpanda",
	}

	// Search paths
	searchPaths := []string{
		execDir,
		filepath.Join(execDir, "browser"),
		"./browser",
		".",
	}

	// Try to find existing binary
	for _, searchPath := range searchPaths {
		for _, binaryName := range binaryNames {
			fullPath := filepath.Join(searchPath, binaryName)
			if info, err := os.Stat(fullPath); err == nil {
				// Ensure file is executable
				if err := ensureExecutable(fullPath, info); err != nil {
					log.Printf("Warning: Failed to ensure executable permissions: %v", err)
				}
				log.Printf("Lightpanda browser found at %s", fullPath)
				return fullPath, true, nil
			}
		}
	}

	// Binary not found, try to download
	log.Printf("Lightpanda browser not found, attempting to download...")

	browserDir := filepath.Join(execDir, "browser")
	if err := os.MkdirAll(browserDir, 0755); err != nil {
		// Try current directory instead
		browserDir = "./browser"
		if err := os.MkdirAll(browserDir, 0755); err != nil {
			log.Printf("⚠️  Warning: Failed to create browser directory: %v", err)
			log.Printf("⚠️  Lightpanda-related APIs will not be available")
			return "", false, nil
		}
	}

	binaryPath := filepath.Join(browserDir, "lightpanda-x86_64-linux")
	if err := downloadLightpanda(binaryPath); err != nil {
		log.Printf("⚠️  Warning: Failed to download Lightpanda browser: %v", err)
		log.Printf("⚠️  Lightpanda-related APIs will not be available")
		return "", false, nil
	}

	return binaryPath, true, nil
}

// downloadLightpanda downloads the Lightpanda browser binary
func downloadLightpanda(destPath string) error {
	log.Printf("Downloading Lightpanda browser from %s", LightpandaDownloadURL)

	resp, err := http.Get(LightpandaDownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("failed to save file: %w", err)
	}

	// Make executable
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to make executable: %w", err)
	}

	log.Printf("Lightpanda browser downloaded and installed at %s", destPath)
	return nil
}

// ensureExecutable ensures the file has executable permissions
func ensureExecutable(path string, info os.FileInfo) error {
	// Check if file is already executable
	mode := info.Mode()
	if mode&0111 != 0 {
		// Already has execute permission
		return nil
	}

	// Add execute permission for owner, group, and others
	newMode := mode | 0755
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", path, err)
	}
	log.Printf("Set executable permissions on %s", path)
	return nil
}
