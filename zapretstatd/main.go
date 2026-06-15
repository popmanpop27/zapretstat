package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	checkInterval = 15 * time.Second
	logFileName   = "zstats.jsonl"
)

func main() {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf(
			"failed to get config dir: %v",
			err,
		)
	}

	configDir := filepath.Join(
		configRoot,
		"zapretstat",
	)

	if err := os.MkdirAll(
		configDir,
		0o755,
	); err != nil {
		log.Fatalf(
			"failed to create config dir: %v",
			err,
		)
	}

	logPath := filepath.Join(
		configDir,
		logFileName,
	)

	storage, err := NewStorage(logPath)
	if err != nil {
		log.Fatalf("failed to init storage: %v", err)
	}
	defer storage.Close()

	logger := log.New(os.Stdout, "[zapretstatd] ", log.LstdFlags)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor := NewMonitor(storage, logger)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	logger.Println("zapretstat daemon started")

	go func() {
		if err := monitor.RunCheck(ctx); err != nil {
			logger.Printf("initial check error: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(
		sigChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	go func() {
		sig := <-sigChan
		logger.Printf("received signal: %s", sig)

		cancel()
		ticker.Stop()

		logger.Println("shutting down zapretstat daemon")
		os.Exit(0)
	}()

	for {
		select {
		case <-ctx.Done():
			logger.Println("context cancelled")
			return

		case <-ticker.C:
			go func() {
				if err := monitor.RunCheck(ctx); err != nil {
					logger.Printf("monitor error: %v", err)
				}
			}()
		}
	}
}

type Sample struct {
	Timestamp time.Time `json:"ts"`
	Type      string    `json:"type"`

	LatencyMS int  `json:"latency_ms"`
	Internet  bool `json:"internet"`
}

type Event struct {
	Timestamp time.Time `json:"ts"`
	Type      string    `json:"type"`

	Event   string `json:"event"`
	Message string `json:"message"`
}

type Storage struct {
	file *os.File
	path string
	mu   sync.Mutex
}

func NewStorage(path string) (*Storage, error) {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o644,
	)
	if err != nil {
		return nil, err
	}

	return &Storage{
		file: file,
		path: path,
	}, nil
}

func (s *Storage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file == nil {
		return nil
	}

	return s.file.Close()
}

func (s *Storage) WriteSample(sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(sample)
	if err != nil {
		return err
	}

	_, err = s.file.Write(
		append(data, '\n'),
	)

	return err
}

func (s *Storage) WriteEvent(event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = s.file.Write(
		append(data, '\n'),
	)

	return err
}

type Monitor struct {
	storage *Storage
	logger  *log.Logger

	lastInternetState bool
	internetKnown     bool

	lastResourceState map[string]bool
}

func NewMonitor(
	storage *Storage,
	logger *log.Logger,
) *Monitor {
	return &Monitor{
		storage: storage,
		logger:  logger,

		lastResourceState: make(map[string]bool),
	}
}

func (m *Monitor) RunCheck(
	ctx context.Context,
) error {
	results := []resourceCheck{
		m.checkGoogleDNS(ctx),
		m.checkCloudflare(ctx),
		m.checkGoogleTCP(ctx),
	}

	internetOK := false
	latency := 0

	var totalLatency int
	var latencyCount int

	for _, r := range results {
		m.handleResourceState(r)

		if r.OK {
			internetOK = true
		}

		if r.OK && r.LatencyMS > 0 {
			totalLatency += r.LatencyMS
			latencyCount++
		}
	}

	if internetOK && latencyCount > 0 {
		latency = totalLatency / latencyCount
	}

	if !internetOK {
		latency = 0
	}

	sample := Sample{
		Timestamp: time.Now().UTC(),
		Type:      "sample",
		LatencyMS: latency,
		Internet:  internetOK,
	}

	if err := m.storage.WriteSample(sample); err != nil {
		return err
	}

	m.handleInternetState(internetOK)

	return nil
}

type resourceCheck struct {
	Name        string
	OK          bool
	LatencyMS   int
	Error       error
	Description string
}

func (m *Monitor) checkGoogleTCP(
	ctx context.Context,
) resourceCheck {
	const (
		target  = "8.8.8.8:53"
		timeout = 4 * time.Second
	)

	start := time.Now()

	dialer := net.Dialer{
		Timeout: timeout,
	}

	conn, err := dialer.DialContext(
		ctx,
		"tcp",
		target,
	)
	if err != nil {
		return resourceCheck{
			Name:        "google_tcp",
			OK:          false,
			LatencyMS:   0,
			Error:       err,
			Description: "tcp dial failed",
		}
	}

	_ = conn.Close()

	latency := int(time.Since(start).Milliseconds())

	return resourceCheck{
		Name:        "google_tcp",
		OK:          true,
		LatencyMS:   latency,
		Description: "tcp connect success",
	}
}

func (m *Monitor) checkGoogleDNS(
	ctx context.Context,
) resourceCheck {
	const timeout = 4 * time.Second

	start := time.Now()

	resolver := &net.Resolver{}

	resolveCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := resolver.LookupHost(
		resolveCtx,
		"google.com",
	)
	if err != nil {
		return resourceCheck{
			Name:        "google_dns",
			OK:          false,
			LatencyMS:   0,
			Error:       err,
			Description: "dns resolve failed",
		}
	}

	latency := int(time.Since(start).Milliseconds())

	return resourceCheck{
		Name:        "google_dns",
		OK:          true,
		LatencyMS:   latency,
		Description: "dns resolve success",
	}
}

func (m *Monitor) checkCloudflare(
	ctx context.Context,
) resourceCheck {
	const timeout = 5 * time.Second

	start := time.Now()

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSHandshakeTimeout: 4 * time.Second,

			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},

			DisableKeepAlives: true,
		},
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodHead,
		"https://cloudflare.com",
		nil,
	)
	if err != nil {
		return resourceCheck{
			Name:        "cloudflare",
			OK:          false,
			LatencyMS:   0,
			Error:       err,
			Description: "request build failed",
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return resourceCheck{
			Name:        "cloudflare",
			OK:          false,
			LatencyMS:   0,
			Error:       err,
			Description: "http request failed",
		}
	}

	_ = resp.Body.Close()

	if resp.StatusCode >= 500 {
		return resourceCheck{
			Name:        "cloudflare",
			OK:          false,
			LatencyMS:   0,
			Description: "server error",
		}
	}

	latency := int(time.Since(start).Milliseconds())

	return resourceCheck{
		Name:        "cloudflare",
		OK:          true,
		LatencyMS:   latency,
		Description: "head request success",
	}
}

func (m *Monitor) handleInternetState(
	internetOK bool,
) {
	now := time.Now().UTC()

	if !m.internetKnown {
		m.internetKnown = true
		m.lastInternetState = internetOK
		return
	}

	if m.lastInternetState == internetOK {
		return
	}

	if !internetOK {
		_ = m.storage.WriteEvent(Event{
			Timestamp: now,
			Type:      "event",
			Event:     "internet_down",
			Message:   "all targets unreachable",
		})

		m.logger.Println("internet down")
	} else {
		_ = m.storage.WriteEvent(Event{
			Timestamp: now,
			Type:      "event",
			Event:     "recovered",
			Message:   "internet restored",
		})

		m.logger.Println("internet restored")
	}

	m.lastInternetState = internetOK
}

func (m *Monitor) handleResourceState(
	check resourceCheck,
) {
	prev, exists := m.lastResourceState[check.Name]

	current := check.OK

	if !exists {
		m.lastResourceState[check.Name] = current
		return
	}

	if prev == current {
		return
	}

	now := time.Now().UTC()

	if !current {
		_ = m.storage.WriteEvent(Event{
			Timestamp: now,
			Type:      "event",
			Event:     "resource_unreachable",
			Message: check.Name +
				" unavailable",
		})

		m.logger.Printf(
			"%s unreachable",
			check.Name,
		)
	} else {
		_ = m.storage.WriteEvent(Event{
			Timestamp: now,
			Type:      "event",
			Event:     "resource_recovered",
			Message: check.Name +
				" restored",
		})
	}

	m.lastResourceState[check.Name] = current
}
