package browser

import "context"

// Client defines the browser operations used by the API handlers.
type Client interface {
	IsRunning() bool
	GetEndpoint() string
	FetchPage(ctx context.Context, url string, opts PageOptions) (*PageResult, error)
	TakeScreenshot(ctx context.Context, url string, fullPage bool, opts PageOptions) ([]byte, error)
	EvaluateScript(ctx context.Context, url string, script string, opts PageOptions) (interface{}, error)
	ClickElement(ctx context.Context, url string, selector string, opts PageOptions) error
	FillForm(ctx context.Context, url string, inputs map[string]string, opts PageOptions) error
	GetPageInfo(ctx context.Context, url string, opts PageOptions) (*PageResult, error)
}
