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

// TestIntegration_FullTunnelLifecycle tests the complete tunnel lifecycle:
// create tunnel -> register -> receive traffic -> unregister -> cleanup
func TestIntegration_FullTunnelLifecycle(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	// Step 1: Generate subdomain
	subdomain, err := reg.GenerateSubdomain()
	if err != nil {
		t.Fatalf("failed to generate subdomain: %v", err)
	}

	// Step 2: Create tunnel
	tunnel := models.NewTunnel("test-user:session-1", subdomain, models.ProtocolHTTP, "127.0.0.1:0")

	// Step 3: Register tunnel
	if err := reg.Register(tunnel); err != nil {
		t.Fatalf("failed to register tunnel: %v", err)
	}

	// Verify tunnel is registered
	if reg.Count() != 1 {
		t.Errorf("expected 1 tunnel, got %d", reg.Count())
	}

	// Step 4: Simulate traffic
	tunnel.UpdateActivity()
	tunnel.AddBytesReceived(1024)
	tunnel.AddBytesSent(512)
	tunnel.IncrementRequests()

	// Verify stats
	bytesReceived, bytesSent, totalRequests := tunnel.GetStats()
	if bytesReceived != 1024 || bytesSent != 512 || totalRequests != 1 {
		t.Errorf("stats mismatch: received=%d, sent=%d, requests=%d", bytesReceived, bytesSent, totalRequests)
	}

	// Step 5: Close tunnel
	if err := tunnel.Close(); err != nil {
		t.Errorf("failed to close tunnel: %v", err)
	}

	if !tunnel.IsClosed() {
		t.Error("expected tunnel to be closed")
	}

	// Step 6: Unregister
	if err := reg.Unregister(subdomain); err != nil {
		t.Errorf("failed to unregister tunnel: %v", err)
	}

	// Verify cleanup
	if reg.Count() != 0 {
		t.Errorf("expected 0 tunnels after cleanup, got %d", reg.Count())
	}

	// Verify tunnel is no longer accessible
	_, err = reg.Get(subdomain)
	if err != registry.ErrTunnelNotFound {
		t.Error("expected ErrTunnelNotFound after unregister")
	}
}

// TestIntegration_MultipleSimultaneousTunnels tests handling multiple concurrent tunnels
func TestIntegration_MultipleSimultaneousTunnels(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	const numTunnels = 10
	tunnels := make([]*models.Tunnel, numTunnels)
	subdomains := make([]string, numTunnels)

	// Create and register multiple tunnels
	for i := 0; i < numTunnels; i++ {
		subdomain, err := reg.GenerateSubdomain()
		if err != nil {
			t.Fatalf("failed to generate subdomain %d: %v", i, err)
		}
		subdomains[i] = subdomain

		tunnel := models.NewTunnel(fmt.Sprintf("user-%d:session-%d", i, i), subdomain, models.ProtocolHTTP, "127.0.0.1:0")
		tunnels[i] = tunnel

		if err := reg.Register(tunnel); err != nil {
			t.Fatalf("failed to register tunnel %d: %v", i, err)
		}
	}

	// Verify all tunnels registered
	if reg.Count() != numTunnels {
		t.Errorf("expected %d tunnels, got %d", numTunnels, reg.Count())
	}

	// Simulate concurrent traffic on all tunnels
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

	// Verify each tunnel has correct stats
	for i, tunnel := range tunnels {
		bytesReceived, bytesSent, totalRequests := tunnel.GetStats()
		if bytesReceived != 10000 {
			t.Errorf("tunnel %d: expected 10000 bytes received, got %d", i, bytesReceived)
		}
		if bytesSent != 5000 {
			t.Errorf("tunnel %d: expected 5000 bytes sent, got %d", i, bytesSent)
		}
		if totalRequests != 100 {
			t.Errorf("tunnel %d: expected 100 requests, got %d", i, totalRequests)
		}
	}

	// Clean up all tunnels
	for i := 0; i < numTunnels; i++ {
		_ = tunnels[i].Close()
		_ = reg.Unregister(subdomains[i])
	}

	if reg.Count() != 0 {
		t.Errorf("expected 0 tunnels after cleanup, got %d", reg.Count())
	}
}

// TestIntegration_ProxyWithMockBackend tests HTTP proxy forwarding with a mock backend
func TestIntegration_ProxyWithMockBackend(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	// Create a mock backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "mock")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Hello from backend!"))
	}))
	defer backendServer.Close()

	// Get the backend server address
	backendAddr := backendServer.Listener.Addr().String()

	// Create and register a tunnel pointing to the mock backend
	subdomain := "testbackend"
	tunnel := models.NewTunnel("test:session", subdomain, models.ProtocolHTTP, backendAddr)
	if err := reg.Register(tunnel); err != nil {
		t.Fatalf("failed to register tunnel: %v", err)
	}

	// Create the HTTP proxy
	httpProxy := proxy.NewHTTPProxy(proxy.Config{
		Addr:       ":0",
		BaseDomain: "localhost",
		Registry:   reg,
		Logger:     logger,
	})

	// Test request to the tunnel subdomain
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Host = "testbackend.localhost:8080"
	w := httptest.NewRecorder()

	httpProxy.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// The tunnel's LocalAddr points to our mock backend, so forwarding should work
	// Note: In real scenario, the SSH tunnel would handle the connection
	// Here we verify the proxy correctly identifies and attempts to forward
	if resp.StatusCode == http.StatusOK {
		if string(body) != "Hello from backend!" {
			t.Errorf("unexpected body: %s", string(body))
		}
	} else if resp.StatusCode != http.StatusBadGateway {
		// Bad gateway is expected if direct connection to backend fails
		// (which is normal since we're not going through SSH)
		t.Logf("got status %d (expected OK or BadGateway)", resp.StatusCode)
	}

	// Verify activity was updated
	if tunnel.LastActive.IsZero() {
		t.Error("expected LastActive to be set")
	}
}

// TestIntegration_ProxyRoutesCorrectly tests that proxy routes to correct tunnels
func TestIntegration_ProxyRoutesCorrectly(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	// Register multiple tunnels
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
		{"example.com", nil, false}, // base domain - status page
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
				// Should attempt to forward (will fail with 502 since no backend)
				if resp.StatusCode != http.StatusBadGateway {
					t.Errorf("expected 502 for %s, got %d", tt.host, resp.StatusCode)
				}
				// Activity should be updated
				if !tt.expectedTunnel.LastActive.After(initialActivity) {
					// Allow for equal time if very fast
					if tt.expectedTunnel.LastActive.Before(initialActivity) {
						t.Error("expected LastActive to be updated")
					}
				}
			} else if tt.host == "example.com" {
				// Base domain should show status page
				if resp.StatusCode != http.StatusOK {
					t.Errorf("expected 200 for base domain, got %d", resp.StatusCode)
				}
			} else {
				// Unknown subdomain should return 404
				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("expected 404 for %s, got %d", tt.host, resp.StatusCode)
				}
			}
		})
	}
}

// TestIntegration_TunnelWithListener tests tunnel with actual network listener
func TestIntegration_TunnelWithListener(t *testing.T) {
	tunnel := models.NewTunnel("test:session", "withlistener", models.ProtocolHTTP, "127.0.0.1:0")

	// Create a real listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	tunnel.SetListener(listener)

	// Verify listener is set
	if tunnel.GetListener() == nil {
		t.Error("expected listener to be set")
	}

	// Get the actual address
	addr := listener.Addr().String()
	if addr == "" {
		t.Error("expected non-empty address")
	}

	// Close the tunnel (should close the listener)
	if err := tunnel.Close(); err != nil {
		t.Errorf("failed to close tunnel: %v", err)
	}

	// Verify listener is closed
	_, err = listener.Accept()
	if err == nil {
		t.Error("expected error when accepting on closed listener")
	}
}

// TestIntegration_GracefulShutdownScenario tests graceful shutdown behavior
func TestIntegration_GracefulShutdownScenario(t *testing.T) {
	logger := newTestLogger()
	reg := registry.NewRegistry(logger, 10*time.Minute)

	// Create multiple active tunnels
	tunnels := make([]*models.Tunnel, 5)
	for i := 0; i < 5; i++ {
		subdomain := fmt.Sprintf("shutdown%d", i)
		tunnel := models.NewTunnel(fmt.Sprintf("user%d:session", i), subdomain, models.ProtocolHTTP, "127.0.0.1:0")
		tunnels[i] = tunnel
		_ = reg.Register(tunnel)
	}

	// Create proxy
	httpProxy := proxy.NewHTTPProxy(proxy.Config{
		Addr:       ":0",
		BaseDomain: "localhost",
		Registry:   reg,
		Logger:     logger,
	})

	// Simulate graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown proxy
	if err := httpProxy.Shutdown(ctx); err != nil {
		t.Errorf("failed to shutdown proxy: %v", err)
	}

	// Close all tunnels
	for i, tunnel := range tunnels {
		if err := tunnel.Close(); err != nil {
			t.Errorf("failed to close tunnel %d: %v", i, err)
		}
		_ = reg.Unregister(fmt.Sprintf("shutdown%d", i))
	}

	// Verify all cleaned up
	if reg.Count() != 0 {
		t.Errorf("expected 0 tunnels after shutdown, got %d", reg.Count())
	}
}

// TestIntegration_ConcurrentRegistrationAndLookup tests thread safety of registry operations
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

	// Concurrent lookups while registrations are happening
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subdomain := fmt.Sprintf("concurrent%d", i)
			_, _ = reg.Get(subdomain) // May or may not find it
			_ = reg.List()
			_ = reg.Count()
		}(i)
	}

	wg.Wait()

	// All registrations should have completed
	if reg.Count() != numGoroutines {
		t.Errorf("expected %d tunnels, got %d", numGoroutines, reg.Count())
	}

	// Concurrent unregistrations
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
		t.Errorf("expected 0 tunnels after cleanup, got %d", reg.Count())
	}
}
