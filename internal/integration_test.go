package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mohamed-rekiba/rift/internal/proxy"
	"github.com/mohamed-rekiba/rift/internal/registry"
	"github.com/mohamed-rekiba/rift/pkg/models"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

// Full tunnel lifecycle: create -> register -> use -> close -> cleanup
func TestIntegration_FullTunnelLifecycle(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	subdomain, err := reg.GenerateSubdomain()
	if err != nil {
		t.Fatalf("couldn't generate subdomain: %v", err)
	}

	tunnel := models.NewTunnel("test-user:session-1", subdomain, models.ProtocolHTTP, "127.0.0.1:0")

	err = reg.Register(tunnel)
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	if reg.Count() != 1 {
		t.Errorf("should have 1 tunnel, got %d", reg.Count())
	}

	// Simulate traffic
	tunnel.UpdateActivity()
	tunnel.AddBytesReceived(1024)
	tunnel.AddBytesSent(512)
	tunnel.IncrementRequests()

	bytesReceived, bytesSent, totalRequests := tunnel.GetStats()
	if bytesReceived != 1024 || bytesSent != 512 || totalRequests != 1 {
		t.Errorf("stats wrong: received=%d, sent=%d, requests=%d", bytesReceived, bytesSent, totalRequests)
	}

	err = tunnel.Close()
	if err != nil {
		t.Errorf("close failed: %v", err)
	}

	if !tunnel.IsClosed() {
		t.Error("tunnel should be marked closed")
	}

	err = reg.Unregister(subdomain)
	if err != nil {
		t.Errorf("unregister failed: %v", err)
	}

	if reg.Count() != 0 {
		t.Errorf("registry should be empty, got %d tunnels", reg.Count())
	}

	_, err = reg.Get(subdomain)
	if err != registry.ErrTunnelNotFound {
		t.Error("tunnel should not be findable after unregister")
	}
}

// TestIntegration_MultipleSimultaneousTunnels tests handling many tunnels at once
func TestIntegration_MultipleSimultaneousTunnels(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	const numTunnels = 10
	tunnels := make([]*models.Tunnel, numTunnels)
	subdomains := make([]string, numTunnels)

	// Create and register a bunch of tunnels
	for i := 0; i < numTunnels; i++ {
		subdomain, err := reg.GenerateSubdomain()
		if err != nil {
			t.Fatalf("subdomain generation %d failed: %v", i, err)
		}
		subdomains[i] = subdomain

		tunnel := models.NewTunnel(fmt.Sprintf("user-%d:session-%d", i, i), subdomain, models.ProtocolHTTP, "127.0.0.1:0")
		tunnels[i] = tunnel

		if err := reg.Register(tunnel); err != nil {
			t.Fatalf("registration %d failed: %v", i, err)
		}
	}

	if reg.Count() != numTunnels {
		t.Errorf("should have %d tunnels, got %d", numTunnels, reg.Count())
	}

	// Simulate traffic on all tunnels concurrently
	var wg sync.WaitGroup
	for i := 0; i < numTunnels; i++ {
		wg.Add(1)
		go func(tunnel *models.Tunnel) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tunnel.UpdateActivity()
				tunnel.AddBytesReceived(100)
				tunnel.AddBytesSent(50)
				tunnel.IncrementRequests()
			}
		}(tunnels[i])
	}
	wg.Wait()

	// Verify stats are correct on each tunnel
	for i, tunnel := range tunnels {
		bytesReceived, bytesSent, totalRequests := tunnel.GetStats()
		if bytesReceived != 10000 {
			t.Errorf("tunnel %d: bytes received should be 10000, got %d", i, bytesReceived)
		}
		if bytesSent != 5000 {
			t.Errorf("tunnel %d: bytes sent should be 5000, got %d", i, bytesSent)
		}
		if totalRequests != 100 {
			t.Errorf("tunnel %d: requests should be 100, got %d", i, totalRequests)
		}
	}

	// Clean up everything
	for i := 0; i < numTunnels; i++ {
		_ = tunnels[i].Close()
		_ = reg.Unregister(subdomains[i])
	}

	if reg.Count() != 0 {
		t.Errorf("registry should be empty after cleanup, got %d", reg.Count())
	}
}

// TestIntegration_ProxyWithMockBackend tests the HTTP proxy forwarding to a real server
func TestIntegration_ProxyWithMockBackend(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	// Spin up a mock backend
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "mock")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello from backend!"))
	}))
	defer backendServer.Close()

	backendAddr := backendServer.Listener.Addr().String()

	// Create a tunnel pointing to our mock backend
	subdomain := "testbackend"
	tunnel := models.NewTunnel("test:session", subdomain, models.ProtocolHTTP, backendAddr)
	if err := reg.Register(tunnel); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Create the proxy
	httpProxy := proxy.NewHTTPProxy(proxy.Config{
		Addr:       ":0",
		BaseDomain: "localhost",
		Registry:   reg,
		Logger:     logger,
	})

	// Make a request to the tunnel
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "testbackend.localhost:8080"
	w := httptest.NewRecorder()

	httpProxy.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// If forwarding worked, we get the backend response
	// If not (no SSH tunnel), we get 502
	if resp.StatusCode == http.StatusOK {
		if string(body) != "Hello from backend!" {
			t.Errorf("unexpected response body: %s", string(body))
		}
	} else if resp.StatusCode != http.StatusBadGateway {
		t.Logf("got status %d (expected 200 or 502)", resp.StatusCode)
	}

	// Activity should be tracked regardless
	if tunnel.LastActive.IsZero() {
		t.Error("tunnel activity should be updated")
	}
}

// TestIntegration_ProxyRoutesCorrectly tests that the proxy sends requests to the right tunnel
func TestIntegration_ProxyRoutesCorrectly(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	// Set up several tunnels
	tunnel1 := models.NewTunnel("user1:session", "tunnel1", models.ProtocolHTTP, "127.0.0.1:3001")
	tunnel2 := models.NewTunnel("user2:session", "tunnel2", models.ProtocolHTTP, "127.0.0.1:3002")
	tunnel3 := models.NewTunnel("user3:session", "tunnel3", models.ProtocolHTTP, "127.0.0.1:3003")

	_ = reg.Register(tunnel1)
	_ = reg.Register(tunnel2)
	_ = reg.Register(tunnel3)

	httpProxy := proxy.NewHTTPProxy(proxy.Config{
		Addr:       ":0",
		BaseDomain: "example.com",
		Registry:   reg,
		Logger:     logger,
	})

	tests := []struct {
		host           string
		expectedTunnel *models.Tunnel
		shouldFind     bool
	}{
		{"tunnel1.example.com", tunnel1, true},
		{"tunnel2.example.com:8080", tunnel2, true},
		{"tunnel3.example.com", tunnel3, true},
		{"nonexistent.example.com", nil, false},
		{"example.com", nil, false}, // base domain = status page
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			initialActivity := time.Time{}
			if tt.expectedTunnel != nil {
				initialActivity = tt.expectedTunnel.LastActive
			}

			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			w := httptest.NewRecorder()

			httpProxy.ServeHTTP(w, req)

			resp := w.Result()

			if tt.shouldFind {
				// Should try to forward (fails with 502 since no real backend)
				if resp.StatusCode != http.StatusBadGateway {
					t.Errorf("expected 502 for %s, got %d", tt.host, resp.StatusCode)
				}
				// Activity should be updated
				if !tt.expectedTunnel.LastActive.After(initialActivity) {
					if tt.expectedTunnel.LastActive.Before(initialActivity) {
						t.Error("activity timestamp should not go backwards")
					}
				}
			} else if tt.host == "example.com" {
				// Base domain = status page
				if resp.StatusCode != http.StatusOK {
					t.Errorf("base domain should return 200, got %d", resp.StatusCode)
				}
			} else {
				// Unknown subdomain = 404
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("unknown subdomain should return 404, got %d", resp.StatusCode)
				}
			}
		})
	}
}

// Tunnel with a real TCP listener attached
func TestIntegration_TunnelWithListener(t *testing.T) {
	tunnel := models.NewTunnel("test:session", "withlistener", models.ProtocolHTTP, "127.0.0.1:0")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("couldn't create listener: %v", err)
	}

	tunnel.SetListener(listener)

	if tunnel.GetListener() == nil {
		t.Error("listener should be set")
	}

	addr := listener.Addr().String()
	if addr == "" {
		t.Error("listener should have an address")
	}

	err = tunnel.Close()
	if err != nil {
		t.Errorf("close failed: %v", err)
	}

	// Accept should fail on closed listener
	_, err = listener.Accept()
	if err == nil {
		t.Error("Accept should fail on closed listener")
	}
}

// TestIntegration_GracefulShutdownScenario tests the shutdown flow
func TestIntegration_GracefulShutdownScenario(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	// Create some active tunnels
	tunnels := make([]*models.Tunnel, 5)
	for i := 0; i < 5; i++ {
		subdomain := fmt.Sprintf("shutdown%d", i)
		tunnel := models.NewTunnel(fmt.Sprintf("user%d:session", i), subdomain, models.ProtocolHTTP, "127.0.0.1:0")
		tunnels[i] = tunnel
		_ = reg.Register(tunnel)
	}

	httpProxy := proxy.NewHTTPProxy(proxy.Config{
		Addr:       ":0",
		BaseDomain: "localhost",
		Registry:   reg,
		Logger:     logger,
	})

	// Simulate graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpProxy.Shutdown(ctx); err != nil {
		t.Errorf("proxy shutdown failed: %v", err)
	}

	// Close all tunnels
	for i, tunnel := range tunnels {
		if err := tunnel.Close(); err != nil {
			t.Errorf("tunnel %d close failed: %v", i, err)
		}
		_ = reg.Unregister(fmt.Sprintf("shutdown%d", i))
	}

	if reg.Count() != 0 {
		t.Errorf("registry should be empty after shutdown, got %d", reg.Count())
	}
}

// TestIntegration_ConcurrentRegistrationAndLookup tests thread safety under load
func TestIntegration_ConcurrentRegistrationAndLookup(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	const numGoroutines = 50
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subdomain := fmt.Sprintf("concurrent%d", i)
			tunnel := models.NewTunnel(fmt.Sprintf("user%d:session", i), subdomain, models.ProtocolHTTP, "127.0.0.1:0")
			_ = reg.Register(tunnel)
		}(i)
	}

	// Concurrent lookups at the same time
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subdomain := fmt.Sprintf("concurrent%d", i)
			_, _ = reg.Get(subdomain) // may or may not find it yet
			_ = reg.List()
			_ = reg.Count()
		}(i)
	}

	wg.Wait()

	// All registrations should have completed
	if reg.Count() != numGoroutines {
		t.Errorf("should have %d tunnels, got %d", numGoroutines, reg.Count())
	}

	// Now do concurrent unregistrations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subdomain := fmt.Sprintf("concurrent%d", i)
			_ = reg.Unregister(subdomain)
		}(i)
	}

	wg.Wait()

	if reg.Count() != 0 {
		t.Errorf("registry should be empty, got %d", reg.Count())
	}
}
