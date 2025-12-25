package browser

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// PageOptions represents options for page operations
type PageOptions struct {
	Timeout     time.Duration `json:"timeout"`
	WaitForLoad bool          `json:"wait_for_load"`
	Screenshot  bool          `json:"screenshot"`
	UserAgent   string        `json:"user_agent,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Cookies     []CookieParam `json:"cookies,omitempty"`
	Proxy       string        `json:"proxy,omitempty"`
}

// DefaultPageOptions returns default page options
func DefaultPageOptions() PageOptions {
	return PageOptions{
		Timeout:     30 * time.Second,
		WaitForLoad: true,
		Screenshot:  false,
	}
}

// PageResult represents the result of a page operation
type PageResult struct {
	URL        string            `json:"url"`
	Title      string            `json:"title"`
	HTML       string            `json:"html,omitempty"`
	Text       string            `json:"text,omitempty"`
	Links      []string          `json:"links,omitempty"`
	Screenshot []byte            `json:"screenshot,omitempty"`
	Cookies    []CookieInfo      `json:"cookies,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// CookieInfo represents cookie information
type CookieInfo struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires"`
	HTTPOnly bool   `json:"http_only"`
	Secure   bool   `json:"secure"`
}

// CookieParam represents cookie parameters sent in requests.
type CookieParam struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	URL      string `json:"url,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Expires  int64  `json:"expires,omitempty"`
	HTTPOnly bool   `json:"http_only,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
}

// FetchPage fetches a page and returns its content
func (m *Manager) FetchPage(ctx context.Context, url string, opts PageOptions) (*PageResult, error) {
	return fetchPage(m, ctx, url, opts)
}

// EvaluateScript evaluates JavaScript on a page
func (m *Manager) EvaluateScript(ctx context.Context, url string, script string, opts PageOptions) (interface{}, error) {
	return evaluateScript(m, ctx, url, script, opts)
}

// ClickElement clicks an element on the page
func (m *Manager) ClickElement(ctx context.Context, url string, selector string, opts PageOptions) error {
	return clickElement(m, ctx, url, selector, opts)
}

// FillForm fills form inputs on a page
func (m *Manager) FillForm(ctx context.Context, url string, inputs map[string]string, opts PageOptions) error {
	return fillForm(m, ctx, url, inputs, opts)
}

// TakeScreenshot takes a screenshot of a page
func (m *Manager) TakeScreenshot(ctx context.Context, url string, fullPage bool, opts PageOptions) ([]byte, error) {
	return takeScreenshot(m, ctx, url, fullPage, opts)
}

// GetPageInfo returns basic page information
func (m *Manager) GetPageInfo(ctx context.Context, url string, opts PageOptions) (*PageResult, error) {
	return getPageInfo(m, ctx, url, opts)
}

type pageOpener interface {
	OpenPage(ctx context.Context, url string, opts PageOptions) (*rod.Page, func(), error)
}

func fetchPage(opener pageOpener, ctx context.Context, url string, opts PageOptions) (*PageResult, error) {
	ctx, cancel := withTimeout(ctx, opts.Timeout)
	defer cancel()

	page, cleanup, err := opener.OpenPage(ctx, url, opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer page.Close()

	result := &PageResult{
		URL: url,
	}

	title := page.MustInfo().Title
	result.Title = title

	html, err := page.HTML()
	if err == nil {
		result.HTML = html
	}

	text, err := page.Eval(`() => document.body.innerText`)
	if err == nil && text.Value.Str() != "" {
		result.Text = text.Value.Str()
	}

	links, err := extractLinks(page)
	if err == nil {
		result.Links = links
	}

	if opts.Screenshot {
		screenshot, err := page.Screenshot(true, nil)
		if err == nil {
			result.Screenshot = screenshot
		}
	}

	return result, nil
}

func extractLinks(page *rod.Page) ([]string, error) {
	result, err := page.Eval(`() => {
		return Array.from(document.querySelectorAll('a')).map(a => a.href).filter(href => href);
	}`)
	if err != nil {
		return nil, err
	}

	var links []string
	arr := result.Value.Arr()
	for _, v := range arr {
		if str := v.Str(); str != "" {
			links = append(links, str)
		}
	}

	return links, nil
}

func evaluateScript(opener pageOpener, ctx context.Context, url string, script string, opts PageOptions) (interface{}, error) {
	ctx, cancel := withTimeout(ctx, opts.Timeout)
	defer cancel()

	page, cleanup, err := opener.OpenPage(ctx, url, opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer page.Close()

	result, err := page.Eval(script)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate script: %w", err)
	}

	return result.Value.Raw(), nil
}

func clickElement(opener pageOpener, ctx context.Context, url string, selector string, opts PageOptions) error {
	ctx, cancel := withTimeout(ctx, opts.Timeout)
	defer cancel()

	page, cleanup, err := opener.OpenPage(ctx, url, opts)
	if err != nil {
		return err
	}
	defer cleanup()
	defer page.Close()

	element, err := page.Element(selector)
	if err != nil {
		return fmt.Errorf("element not found: %s", selector)
	}

	if err := element.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("failed to click element: %w", err)
	}

	return nil
}

func fillForm(opener pageOpener, ctx context.Context, url string, inputs map[string]string, opts PageOptions) error {
	ctx, cancel := withTimeout(ctx, opts.Timeout)
	defer cancel()

	page, cleanup, err := opener.OpenPage(ctx, url, opts)
	if err != nil {
		return err
	}
	defer cleanup()
	defer page.Close()

	for selector, value := range inputs {
		element, err := page.Element(selector)
		if err != nil {
			return fmt.Errorf("element not found: %s", selector)
		}

		if err := element.Input(value); err != nil {
			return fmt.Errorf("failed to input value for %s: %w", selector, err)
		}
	}

	return nil
}

func takeScreenshot(opener pageOpener, ctx context.Context, url string, fullPage bool, opts PageOptions) ([]byte, error) {
	ctx, cancel := withTimeout(ctx, opts.Timeout)
	defer cancel()

	page, cleanup, err := opener.OpenPage(ctx, url, opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer page.Close()

	screenshot, err := page.Screenshot(fullPage, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return screenshot, nil
}

func getPageInfo(opener pageOpener, ctx context.Context, url string, opts PageOptions) (*PageResult, error) {
	ctx, cancel := withTimeout(ctx, opts.Timeout)
	defer cancel()

	page, cleanup, err := opener.OpenPage(ctx, url, opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer page.Close()

	info := page.MustInfo()

	return &PageResult{
		URL:   info.URL,
		Title: info.Title,
	}, nil
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}

func applyPageOptions(page *rod.Page, targetURL string, opts PageOptions) error {
	if opts.UserAgent != "" {
		if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: opts.UserAgent}); err != nil {
			return fmt.Errorf("failed to set user agent: %w", err)
		}
	}

	if len(opts.Headers) > 0 {
		pairs := make([]string, 0, len(opts.Headers)*2)
		for key, value := range opts.Headers {
			pairs = append(pairs, key, value)
		}
		if _, err := page.SetExtraHeaders(pairs); err != nil {
			return fmt.Errorf("failed to set headers: %w", err)
		}
	}

	if len(opts.Cookies) > 0 {
		params, err := toCookieParams(targetURL, opts.Cookies)
		if err != nil {
			return err
		}
		if err := page.SetCookies(params); err != nil {
			return fmt.Errorf("failed to set cookies: %w", err)
		}
	}

	return nil
}

func toCookieParams(targetURL string, cookies []CookieParam) ([]*proto.NetworkCookieParam, error) {
	params := make([]*proto.NetworkCookieParam, 0, len(cookies))
	parsedURL, _ := url.Parse(targetURL)

	for _, cookie := range cookies {
		param := &proto.NetworkCookieParam{
			Name:     cookie.Name,
			Value:    cookie.Value,
			URL:      cookie.URL,
			Domain:   cookie.Domain,
			Path:     cookie.Path,
			Secure:   cookie.Secure,
			HTTPOnly: cookie.HTTPOnly,
		}

		if cookie.Expires > 0 {
			param.Expires = proto.TimeSinceEpoch(cookie.Expires)
		}

		if param.URL == "" && param.Domain == "" && parsedURL != nil {
			param.URL = parsedURL.String()
		}

		params = append(params, param)
	}

	return params, nil
}

func noopCleanup() {}
