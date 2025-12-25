package browser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// Manager handles Lightpanda browser lifecycle
type Manager struct {
	host       string
	port       int
	cmd        *exec.Cmd
	browser    *rod.Browser
	mu         sync.Mutex
	restartMu  sync.Mutex
	isRunning  bool
	binaryPath string
}

// NewManager creates a new browser manager
func NewManager(host string, port int) (*Manager, error) {
	binaryPath, err := findBrowserBinaryLegacy()
	if err != nil {
		return nil, fmt.Errorf("failed to find browser binary: %w", err)
	}

	return &Manager{
		host:       host,
		port:       port,
		binaryPath: binaryPath,
	}, nil
}

// NewManagerWithPath creates a new browser manager with a specific binary path
func NewManagerWithPath(binaryPath string, host string, port int) (*Manager, error) {
	return &Manager{
		host:       host,
		port:       port,
		binaryPath: binaryPath,
	}, nil
}

// findBrowserBinaryLegacy finds the Lightpanda browser binary (legacy)
func findBrowserBinaryLegacy() (string, error) {
	// Get the executable directory
	execPath, err := os.Executable()
	if err != nil {
		return "", err
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

	for _, searchPath := range searchPaths {
		for _, binaryName := range binaryNames {
			fullPath := filepath.Join(searchPath, binaryName)
			if _, err := os.Stat(fullPath); err == nil {
				return fullPath, nil
			}
		}
	}

	return "", fmt.Errorf("lightpanda browser binary not found in search paths")
}

// Start starts the Lightpanda browser CDP server
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return nil
	}

	// Check if running on Linux
	if runtime.GOOS != "linux" {
		return fmt.Errorf("Lightpanda browser only supports Linux, current OS: %s", runtime.GOOS)
	}

	// Start Lightpanda browser
	m.cmd = exec.Command(m.binaryPath, "serve", "--host", m.host, "--port", fmt.Sprintf("%d", m.port))
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Lightpanda browser: %w", err)
	}

	// Wait for browser to be ready
	time.Sleep(2 * time.Second)

	// Connect to browser via CDP
	wsURL := fmt.Sprintf("ws://%s:%d", m.host, m.port)
	browser := rod.New().ControlURL(launcher.MustResolveURL(wsURL))

	if err := browser.Connect(); err != nil {
		if killErr := m.cmd.Process.Kill(); killErr != nil {
			log.Printf("Warning: failed to kill browser process after connect error: %v", killErr)
		}
		if waitErr := m.cmd.Wait(); waitErr != nil {
			log.Printf("Warning: failed to wait for browser process after connect error: %v", waitErr)
		}
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	m.browser = browser
	m.isRunning = true

	log.Printf("Lightpanda browser started on %s:%d", m.host, m.port)
	return nil
}

// Stop stops the Lightpanda browser
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	if m.browser != nil {
		if err := m.browser.Close(); err != nil {
			log.Printf("Warning: failed to close browser: %v", err)
		}
	}

	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Kill(); err != nil {
			log.Printf("Warning: failed to kill browser process: %v", err)
		}
		if err := m.cmd.Wait(); err != nil {
			log.Printf("Warning: failed to wait for browser process: %v", err)
		}
	}

	m.browser = nil
	m.cmd = nil
	m.isRunning = false
	log.Println("Lightpanda browser stopped")
	return nil
}

// GetBrowser returns the rod browser instance
func (m *Manager) GetBrowser() *rod.Browser {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.browser
}

// IsRunning returns true if the browser is running
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning
}

// GetEndpoint returns the WebSocket endpoint URL
func (m *Manager) GetEndpoint() string {
	return fmt.Sprintf("ws://%s:%d", m.host, m.port)
}

// NewPage creates a new browser page
func (m *Manager) NewPage(ctx context.Context) (*rod.Page, error) {
	if err := m.ensureStarted(); err != nil {
		return nil, fmt.Errorf("failed to start browser: %w", err)
	}

	page, err := m.browser.Context(ctx).Page(proto.TargetCreateTarget{})
	if err != nil {
		if !isConnectionError(err) {
			return nil, fmt.Errorf("failed to create new page: %w", err)
		}

		if restartErr := m.restart(); restartErr != nil {
			return nil, fmt.Errorf("failed to restart browser after connection error: %w", restartErr)
		}

		page, err = m.browser.Context(ctx).Page(proto.TargetCreateTarget{})
		if err != nil {
			return nil, fmt.Errorf("failed to create new page: %w", err)
		}
		return page, nil
	}

	return page, nil
}

// OpenPage creates a page, applies options, and navigates to the URL.
func (m *Manager) OpenPage(ctx context.Context, url string, opts PageOptions) (*rod.Page, func(), error) {
	if opts.Proxy != "" {
		return nil, noopCleanup, fmt.Errorf("proxy is only supported on chrome endpoints")
	}

	page, err := m.NewPage(ctx)
	if err != nil {
		return nil, noopCleanup, err
	}

	if err := applyPageOptions(page, url, opts); err != nil {
		page.Close()
		return nil, noopCleanup, err
	}

	if err := page.Navigate(url); err != nil {
		page.Close()
		return nil, noopCleanup, fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	if opts.WaitForLoad {
		if err := page.WaitLoad(); err != nil {
			page.Close()
			return nil, noopCleanup, fmt.Errorf("failed to wait for page load: %w", err)
		}
	}

	return page, noopCleanup, nil
}

func (m *Manager) ensureStarted() error {
	if m.IsRunning() {
		return nil
	}

	m.restartMu.Lock()
	defer m.restartMu.Unlock()

	if m.IsRunning() {
		return nil
	}

	return m.Start()
}

func (m *Manager) restart() error {
	m.restartMu.Lock()
	defer m.restartMu.Unlock()

	if err := m.Stop(); err != nil {
		log.Printf("Warning: failed to stop browser before restart: %v", err)
	}

	return m.Start()
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "eof")
}

// Navigate navigates to a URL and returns the page
func (m *Manager) Navigate(ctx context.Context, url string) (*rod.Page, error) {
	page, _, err := m.OpenPage(ctx, url, DefaultPageOptions())
	return page, err
}
