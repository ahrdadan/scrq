package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ahrdadan/scrq/internal/api"
	"github.com/ahrdadan/scrq/internal/browser"
	"github.com/ahrdadan/scrq/internal/config"
	"github.com/ahrdadan/scrq/internal/nats"
	"github.com/ahrdadan/scrq/internal/queue"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Parse CLI flags
	cfg := config.ParseFlags()

	// Handle --version and --help
	config.HandleFlags(cfg)

	// Banner
	log.Printf("Starting %s v%s (Scrape + Queue)", config.AppName, config.Version)

	var browserManager *browser.Manager
	var lightpandaAvailable bool

	// Check and download Lightpanda if needed
	lightpandaPath, available, err := browser.EnsureLightpandaBinary()
	if err != nil {
		log.Printf("Warning: Error checking Lightpanda: %v", err)
	}
	lightpandaAvailable = available

	if lightpandaAvailable {
		// Start Lightpanda browser
		browserManager, err = browser.NewManagerWithPath(lightpandaPath, cfg.BrowserHost, cfg.BrowserPort)
		if err != nil {
			log.Printf("Warning: Failed to initialize browser manager: %v", err)
			lightpandaAvailable = false
		} else {
			if err := browserManager.Start(); err != nil {
				log.Printf("Warning: Failed to start Lightpanda browser: %v", err)
				lightpandaAvailable = false
			} else {
				defer func() {
					if err := browserManager.Stop(); err != nil {
						log.Printf("Failed to stop Lightpanda browser: %v", err)
					}
				}()
			}
		}
	}

	if !lightpandaAvailable {
		log.Printf("⚠️  Lightpanda browser not available - Lightpanda-related APIs will be disabled")
	}

	// Chrome setup
	var chromeManager *browser.ChromeManager
	if cfg.WithChrome {
		chromeBin, err := browser.InstallChrome(context.Background(), cfg.ChromeRevision)
		if err != nil {
			log.Fatalf("Failed to install Chrome: %v", err)
		}

		chromeManager = browser.NewChromeManager(chromeBin)
		if err := chromeManager.Start(); err != nil {
			log.Fatalf("Failed to start Chrome: %v", err)
		}
		defer func() {
			if err := chromeManager.Stop(); err != nil {
				log.Printf("Failed to stop Chrome: %v", err)
			}
		}()
	}

	// NATS + JetStream setup
	var natsServer *nats.Server
	var queueManager *queue.Manager

	if cfg.WithNats {
		log.Printf("Setting up NATS JetStream...")

		natsServer, err = nats.NewServer(nats.ServerConfig{
			BinPath:  cfg.NatsBin,
			StoreDir: cfg.NatsStore,
			URL:      cfg.NatsURL,
			AutoDL:   cfg.NatsAutoDL,
		})
		if err != nil {
			log.Fatalf("Failed to create NATS server: %v", err)
		}

		ctx := context.Background()
		if err := natsServer.Start(ctx); err != nil {
			log.Fatalf("Failed to start NATS server: %v", err)
		}
		defer func() { _ = natsServer.Stop() }()

		// Create queue manager
		js := natsServer.GetJetStream()
		queueManager, err = queue.NewManager(js)
		if err != nil {
			log.Fatalf("Failed to create queue manager: %v", err)
		}

		// Create and start processor
		var lightpandaClient browser.Client
		var chromeClient browser.Client

		if lightpandaAvailable && browserManager != nil {
			lightpandaClient = browserManager
		}
		if chromeManager != nil {
			chromeClient = chromeManager
		}

		processor := queue.NewScrapeProcessor(lightpandaClient, chromeClient)
		if err := queueManager.Start(processor); err != nil {
			log.Fatalf("Failed to start queue processor: %v", err)
		}
		defer queueManager.Stop()
	}

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      config.AppName,
		ErrorHandler: api.ErrorHandler,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	// Setup routes
	if lightpandaAvailable && browserManager != nil {
		api.SetupRoutes(app, browserManager)
	} else {
		// Setup health check only if no browser
		app.Get("/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"status":     "ok",
					"lightpanda": false,
				},
			})
		})
	}

	if chromeManager != nil {
		api.SetupChromeRoutes(app, chromeManager)
	}

	if queueManager != nil {
		// Setup job routes with security configuration
		routeConfig := api.RouteConfig{
			RateLimitRequests: cfg.RateLimitRequests,
			RateLimitWindow:   cfg.RateLimitWindow,
			IdempotencyTTL:    cfg.IdempotencyTTL,
			BaseURL:           cfg.BaseURL,
		}
		api.SetupJobRoutesWithConfig(app, queueManager, routeConfig)
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down server...")
		if browserManager != nil {
			if err := browserManager.Stop(); err != nil {
				log.Printf("Failed to stop Lightpanda browser: %v", err)
			}
		}
		if err := app.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
	}()

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("Starting server on %s", addr)

	if lightpandaAvailable {
		log.Printf("Lightpanda browser CDP endpoint: ws://%s:%d", cfg.BrowserHost, cfg.BrowserPort)
	}
	if cfg.WithNats {
		log.Printf("NATS JetStream enabled at %s", cfg.NatsURL)
	}

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
