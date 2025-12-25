package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	// Version is the current version of Scrq
	Version = "1"
	// AppName is the application name
	AppName = "Scrq Server"
)

// Config holds all configuration options for the Scrq server
type Config struct {
	// Server
	Host    string
	Port    int
	BaseURL string // Full base URL for API responses (e.g., http://localhost:8000)

	// Browser (Lightpanda CDP)
	BrowserHost string
	BrowserPort int

	// Chrome
	WithChrome     bool
	ChromeRevision int

	// Queue (NATS JetStream)
	WithNats   bool
	NatsURL    string
	NatsStore  string
	NatsAutoDL bool
	NatsBin    string

	// Security
	RateLimitRequests int           // requests per window
	RateLimitWindow   time.Duration // time window for rate limiting
	IdempotencyTTL    time.Duration // TTL for idempotency keys
	ResultTTL         time.Duration // TTL for job results
	MaxJobTimeout     time.Duration // Maximum allowed job timeout
	MaxRetries        int           // Maximum retries per job

	// Flags
	ShowVersion bool
	ShowHelp    bool
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Host:              "0.0.0.0",
		Port:              8000,
		BaseURL:           "", // Will be auto-generated if empty
		BrowserHost:       "127.0.0.1",
		BrowserPort:       9222,
		WithChrome:        false,
		ChromeRevision:    0,
		WithNats:          true,
		NatsURL:           "nats://127.0.0.1:4222",
		NatsStore:         "./data/nats",
		NatsAutoDL:        true,
		NatsBin:           "./bin/nats-server",
		RateLimitRequests: 100,
		RateLimitWindow:   time.Minute,
		IdempotencyTTL:    24 * time.Hour,
		ResultTTL:         7 * 24 * time.Hour, // 7 days
		MaxJobTimeout:     5 * time.Minute,
		MaxRetries:        5,
		ShowVersion:       false,
		ShowHelp:          false,
	}
}

// ParseFlags parses command line flags and returns the config
func ParseFlags() *Config {
	cfg := DefaultConfig()

	// Server flags
	flag.StringVar(&cfg.Host, "host", cfg.Host, "Host address to bind the server")
	flag.IntVar(&cfg.Port, "port", cfg.Port, "Port number for the server")
	flag.StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "Base URL for API responses (e.g., http://localhost:8000)")

	// Browser flags
	flag.StringVar(&cfg.BrowserHost, "browser-host", cfg.BrowserHost, "Lightpanda browser CDP host")
	flag.IntVar(&cfg.BrowserPort, "browser-port", cfg.BrowserPort, "Lightpanda browser CDP port")

	// Chrome flags
	flag.BoolVar(&cfg.WithChrome, "with-chrome", cfg.WithChrome, "Download Chrome and enable Chrome-backed endpoints")
	flag.IntVar(&cfg.ChromeRevision, "chrome-revision", cfg.ChromeRevision, "Chromium revision to download (0 uses default)")

	// NATS flags
	flag.BoolVar(&cfg.WithNats, "with-nats", cfg.WithNats, "Enable NATS JetStream for job queue")
	flag.StringVar(&cfg.NatsURL, "nats-url", cfg.NatsURL, "NATS server URL")
	flag.StringVar(&cfg.NatsStore, "nats-store", cfg.NatsStore, "NATS JetStream storage directory")
	flag.BoolVar(&cfg.NatsAutoDL, "nats-autodl", cfg.NatsAutoDL, "Auto-download NATS server binary")
	flag.StringVar(&cfg.NatsBin, "nats-bin", cfg.NatsBin, "Path to NATS server binary")

	// Security flags
	flag.IntVar(&cfg.RateLimitRequests, "rate-limit", cfg.RateLimitRequests, "Rate limit requests per minute")
	flag.IntVar(&cfg.MaxRetries, "max-retries", cfg.MaxRetries, "Maximum retries per job (1-10)")

	// Other flags
	flag.BoolVar(&cfg.ShowVersion, "version", cfg.ShowVersion, "Show version information")
	flag.BoolVar(&cfg.ShowHelp, "help", cfg.ShowHelp, "Show help message")

	// Custom usage function
	flag.Usage = func() {
		PrintHelp()
	}

	flag.Parse()

	// Auto-generate BaseURL if not provided
	if cfg.BaseURL == "" {
		host := cfg.Host
		if host == "0.0.0.0" {
			host = "localhost"
		}
		cfg.BaseURL = fmt.Sprintf("http://%s:%d", host, cfg.Port)
	}

	// Validate
	if cfg.MaxRetries < 1 {
		cfg.MaxRetries = 1
	}
	if cfg.MaxRetries > 10 {
		cfg.MaxRetries = 10
	}
	if cfg.RateLimitRequests < 1 {
		cfg.RateLimitRequests = 100
	}

	return cfg
}

// PrintVersion prints version information
func PrintVersion() {
	fmt.Printf("%s v%s\n", AppName, Version)
}

// PrintHelp prints help information
func PrintHelp() {
	fmt.Printf(`%s v%s (Scrape + Queue)

Usage:
  ./server [flags]

Server:
  --host            %s
  --port            %d
  --base-url        %s (auto-generated if empty)

Browser (Lightpanda CDP):
  --browser-host    %s
  --browser-port    %d

Chrome:
  --with-chrome     %v
  --chrome-revision %d

Queue (NATS JetStream):
  --with-nats        %v
  --nats-url         %s
  --nats-store       %s
  --nats-autodl      %v
  --nats-bin         %s

Security:
  --rate-limit       %d (requests per minute)
  --max-retries      %d (max retries per job)

Other:
  --version         show version
  --help            show this help

`, AppName, Version,
		"0.0.0.0", 8000, "http://localhost:8000",
		"127.0.0.1", 9222,
		false, 0,
		true, "nats://127.0.0.1:4222", "./data/nats", true, "./bin/nats-server",
		100, 5)
}

// HandleFlags handles version and help flags, exits if needed
func HandleFlags(cfg *Config) {
	if cfg.ShowVersion {
		PrintVersion()
		os.Exit(0)
	}

	if cfg.ShowHelp {
		PrintHelp()
		os.Exit(0)
	}
}
