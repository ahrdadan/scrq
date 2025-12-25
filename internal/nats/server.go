package nats

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Server manages a local NATS server instance
type Server struct {
	binPath   string
	storeDir  string
	url       string
	cmd       *exec.Cmd
	nc        *nats.Conn
	js        jetstream.JetStream
	mu        sync.Mutex
	isRunning bool
}

// ServerConfig holds configuration for the NATS server
type ServerConfig struct {
	BinPath  string
	StoreDir string
	URL      string
	AutoDL   bool
}

// NewServer creates a new NATS server manager
func NewServer(cfg ServerConfig) (*Server, error) {
	// Ensure binary exists
	binPath, err := EnsureNATSBinary(cfg.BinPath, cfg.AutoDL)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure NATS binary: %w", err)
	}

	return &Server{
		binPath:  binPath,
		storeDir: cfg.StoreDir,
		url:      cfg.URL,
	}, nil
}

// Start starts the NATS server if not already running
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}

	// Check if NATS is already running at the URL
	if s.isReachable() {
		log.Printf("NATS server already running at %s", s.url)
		return s.connect()
	}

	// Create store directory
	absStoreDir, err := filepath.Abs(s.storeDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for store dir: %w", err)
	}

	if err := os.MkdirAll(absStoreDir, 0755); err != nil {
		return fmt.Errorf("failed to create store directory: %w", err)
	}

	// Parse host and port from URL
	host, port, err := parseNatsURL(s.url)
	if err != nil {
		return fmt.Errorf("failed to parse NATS URL: %w", err)
	}

	// Start NATS server with JetStream
	s.cmd = exec.CommandContext(ctx, s.binPath,
		"-js",
		"-sd", absStoreDir,
		"-a", host,
		"-p", port,
	)
	s.cmd.Stdout = os.Stdout
	s.cmd.Stderr = os.Stderr

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start NATS server: %w", err)
	}

	// Wait for server to be ready
	time.Sleep(2 * time.Second)

	if err := s.connect(); err != nil {
		_ = s.Stop()
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	s.isRunning = true
	log.Printf("NATS server started at %s with JetStream enabled", s.url)
	return nil
}

// Stop stops the NATS server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}

	if s.nc != nil {
		s.nc.Close()
		s.nc = nil
	}

	if s.cmd != nil && s.cmd.Process != nil {
		if err := s.cmd.Process.Kill(); err != nil {
			log.Printf("Warning: failed to kill NATS process: %v", err)
		}
		if err := s.cmd.Wait(); err != nil {
			log.Printf("Warning: failed to wait for NATS process: %v", err)
		}
	}

	s.cmd = nil
	s.js = nil
	s.isRunning = false

	log.Println("NATS server stopped")
	return nil
}

// IsRunning returns true if NATS server is running
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// GetConnection returns the NATS connection
func (s *Server) GetConnection() *nats.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nc
}

// GetJetStream returns the JetStream context
func (s *Server) GetJetStream() jetstream.JetStream {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.js
}

func (s *Server) isReachable() bool {
	host, port, err := parseNatsURL(s.url)
	if err != nil {
		return false
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (s *Server) connect() error {
	nc, err := nats.Connect(s.url)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return fmt.Errorf("failed to create JetStream context: %w", err)
	}

	s.nc = nc
	s.js = js
	return nil
}

func parseNatsURL(natsURL string) (host, port string, err error) {
	// Remove nats:// prefix
	url := strings.TrimPrefix(natsURL, "nats://")

	// Split host and port
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid NATS URL format: %s", natsURL)
	}

	return parts[0], parts[1], nil
}
