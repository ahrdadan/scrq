package browser

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// ChromeManager manages a Chromium/Chrome instance launched by rod.
type ChromeManager struct {
	binPath   string
	mu        sync.Mutex
	restartMu sync.Mutex
	launcher  *launcher.Launcher
	browser   *rod.Browser
	wsURL     string
	running   bool
}

// NewChromeManager creates a new Chrome manager.
func NewChromeManager(binPath string) *ChromeManager {
	return &ChromeManager{
		binPath: binPath,
	}
}

// Start launches Chrome and connects via CDP.
func (m *ChromeManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	l := launcher.New()
	if m.binPath != "" {
		l.Bin(m.binPath)
	}

	wsURL, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch chrome: %w", err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		l.Kill()
		return fmt.Errorf("failed to connect to chrome: %w", err)
	}

	m.launcher = l
	m.browser = browser
	m.wsURL = wsURL
	m.running = true

	log.Printf("Chrome started with endpoint %s", wsURL)
	return nil
}

// Stop stops Chrome.
func (m *ChromeManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.browser != nil {
		if err := m.browser.Close(); err != nil {
			log.Printf("Warning: failed to close chrome: %v", err)
		}
	}

	if m.launcher != nil {
		m.launcher.Kill()
		m.launcher.Cleanup()
	}

	m.launcher = nil
	m.browser = nil
	m.wsURL = ""
	m.running = false

	log.Println("Chrome stopped")
	return nil
}

// IsRunning reports whether Chrome is running.
func (m *ChromeManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// GetEndpoint returns the Chrome DevTools endpoint.
func (m *ChromeManager) GetEndpoint() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.wsURL
}

// NewPage creates a new browser page.
func (m *ChromeManager) NewPage(ctx context.Context) (*rod.Page, error) {
	if err := m.ensureStarted(); err != nil {
		return nil, fmt.Errorf("failed to start chrome: %w", err)
	}

	page, err := m.browser.Context(ctx).Page(proto.TargetCreateTarget{})
	if err != nil {
		if !isConnectionError(err) {
			return nil, fmt.Errorf("failed to create new page: %w", err)
		}

		if restartErr := m.restartBrowser(); restartErr != nil {
			return nil, fmt.Errorf("failed to restart chrome after connection error: %w", restartErr)
		}

		page, err = m.browser.Context(ctx).Page(proto.TargetCreateTarget{})
		if err != nil {
			return nil, fmt.Errorf("failed to create new page: %w", err)
		}
	}

	return page, nil
}

// OpenPage creates a page, applies options, and navigates to the URL.
func (m *ChromeManager) OpenPage(ctx context.Context, url string, opts PageOptions) (*rod.Page, func(), error) {
	if opts.Proxy != "" {
		return m.openPageWithProxy(ctx, url, opts)
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

// Navigate navigates to a URL and returns the page.
func (m *ChromeManager) Navigate(ctx context.Context, url string) (*rod.Page, error) {
	page, _, err := m.OpenPage(ctx, url, DefaultPageOptions())
	return page, err
}

// FetchPage fetches a page and returns its content.
func (m *ChromeManager) FetchPage(ctx context.Context, url string, opts PageOptions) (*PageResult, error) {
	return fetchPage(m, ctx, url, opts)
}

// EvaluateScript evaluates JavaScript on a page.
func (m *ChromeManager) EvaluateScript(ctx context.Context, url string, script string, opts PageOptions) (interface{}, error) {
	return evaluateScript(m, ctx, url, script, opts)
}

// ClickElement clicks an element on the page.
func (m *ChromeManager) ClickElement(ctx context.Context, url string, selector string, opts PageOptions) error {
	return clickElement(m, ctx, url, selector, opts)
}

// FillForm fills form inputs on a page.
func (m *ChromeManager) FillForm(ctx context.Context, url string, inputs map[string]string, opts PageOptions) error {
	return fillForm(m, ctx, url, inputs, opts)
}

// TakeScreenshot takes a screenshot of a page.
func (m *ChromeManager) TakeScreenshot(ctx context.Context, url string, fullPage bool, opts PageOptions) ([]byte, error) {
	return takeScreenshot(m, ctx, url, fullPage, opts)
}

// GetPageInfo returns basic page information.
func (m *ChromeManager) GetPageInfo(ctx context.Context, url string, opts PageOptions) (*PageResult, error) {
	return getPageInfo(m, ctx, url, opts)
}

func (m *ChromeManager) ensureStarted() error {
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

func (m *ChromeManager) restartBrowser() error {
	m.restartMu.Lock()
	defer m.restartMu.Unlock()

	if err := m.Stop(); err != nil {
		log.Printf("Warning: failed to stop chrome before restart: %v", err)
	}

	return m.Start()
}

func (m *ChromeManager) openPageWithProxy(ctx context.Context, url string, opts PageOptions) (*rod.Page, func(), error) {
	l := launcher.New().Proxy(opts.Proxy)
	if m.binPath != "" {
		l.Bin(m.binPath)
	}

	wsURL, err := l.Launch()
	if err != nil {
		return nil, noopCleanup, fmt.Errorf("failed to launch chrome with proxy: %w", err)
	}

	browser := rod.New().ControlURL(wsURL)
	if err := browser.Connect(); err != nil {
		l.Kill()
		l.Cleanup()
		return nil, noopCleanup, fmt.Errorf("failed to connect to chrome with proxy: %w", err)
	}

	page, err := browser.Context(ctx).Page(proto.TargetCreateTarget{})
	if err != nil {
		_ = browser.Close()
		l.Kill()
		l.Cleanup()
		return nil, noopCleanup, fmt.Errorf("failed to create new page: %w", err)
	}

	cleanup := func() {
		if err := browser.Close(); err != nil {
			log.Printf("Warning: failed to close chrome proxy browser: %v", err)
		}
		l.Kill()
		l.Cleanup()
	}

	if err := applyPageOptions(page, url, opts); err != nil {
		page.Close()
		cleanup()
		return nil, noopCleanup, err
	}

	if err := page.Navigate(url); err != nil {
		page.Close()
		cleanup()
		return nil, noopCleanup, fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	if opts.WaitForLoad {
		if err := page.WaitLoad(); err != nil {
			page.Close()
			cleanup()
			return nil, noopCleanup, fmt.Errorf("failed to wait for page load: %w", err)
		}
	}

	return page, cleanup, nil
}
