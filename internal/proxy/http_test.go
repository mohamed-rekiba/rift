package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mohamed-rekiba/rift/internal/registry"
	"github.com/mohamed-rekiba/rift/pkg/models"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestRegistry() *registry.Registry {
	return registry.NewRegistry(newTestLogger(), 10*time.Minute)
}

func newTestProxy(baseDomain string) *HTTPProxy {
	return NewHTTPProxy(Config{
		Addr:       ":0",
		BaseDomain: baseDomain,
		Registry:   newTestRegistry(),
		Logger:     newTestLogger(),
	})
}

func TestNewHTTPProxy(t *testing.T) {
	reg := newTestRegistry()
	logger := newTestLogger()

	proxy := NewHTTPProxy(Config{
		Addr:       ":8080",
		BaseDomain: "example.com",
		Registry:   reg,
		Logger:     logger,
	})

	if proxy == nil {
		t.Fatal("expected non-nil proxy")
	}
	if proxy.baseDomain != "example.com" {
		t.Errorf("expected baseDomain example.com, got %s", proxy.baseDomain)
	}
	if proxy.registry != reg {
		t.Error("expected registry to be set")
	}
}

func TestExtractSubdomain_WithPort(t *testing.T) {
	tests := []struct {
		name       string
		baseDomain string
		host       string
		want       string
	}{
		{
			name:       "subdomain with port",
			baseDomain: "example.com",
			host:       "abc123.example.com:8080",
			want:       "abc123",
		},
		{
			name:       "nested subdomain with port",
			baseDomain: "example.com",
			host:       "test.abc123.example.com:8080",
			want:       "test.abc123",
		},
		{
			name:       "localhost with port",
			baseDomain: "localhost",
			host:       "abc123.localhost:8080",
			want:       "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := newTestProxy(tt.baseDomain)
			result := proxy.extractSubdomain(tt.host)
			if result != tt.want {
				t.Errorf("expected %s, got %s", tt.want, result)
			}
		})
	}
}

func TestExtractSubdomain_WithoutPort(t *testing.T) {
	tests := []struct {
		name       string
		baseDomain string
		host       string
		want       string
	}{
		{
			name:       "subdomain without port",
			baseDomain: "example.com",
			host:       "abc123.example.com",
			want:       "abc123",
		},
		{
			name:       "nested subdomain without port",
			baseDomain: "example.com",
			host:       "test.abc123.example.com",
			want:       "test.abc123",
		},
		{
			name:       "localhost without port",
			baseDomain: "localhost",
			host:       "abc123.localhost",
			want:       "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := newTestProxy(tt.baseDomain)
			result := proxy.extractSubdomain(tt.host)
			if result != tt.want {
				t.Errorf("expected %s, got %s", tt.want, result)
			}
		})
	}
}

func TestExtractSubdomain_BaseDomain(t *testing.T) {
	tests := []struct {
		name       string
		baseDomain string
		host       string
		want       string
	}{
		{
			name:       "base domain only",
			baseDomain: "example.com",
			host:       "example.com",
			want:       "",
		},
		{
			name:       "base domain with port",
			baseDomain: "example.com",
			host:       "example.com:8080",
			want:       "",
		},
		{
			name:       "localhost base domain",
			baseDomain: "localhost",
			host:       "localhost",
			want:       "",
		},
		{
			name:       "localhost with port",
			baseDomain: "localhost",
			host:       "localhost:8080",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := newTestProxy(tt.baseDomain)
			result := proxy.extractSubdomain(tt.host)
			if result != tt.want {
				t.Errorf("expected %q, got %q", tt.want, result)
			}
		})
	}
}

func TestExtractSubdomain_DirectHost(t *testing.T) {
	tests := []struct {
		name       string
		baseDomain string
		host       string
		want       string
	}{
		{
			name:       "different domain",
			baseDomain: "example.com",
			host:       "other.com",
			want:       "other.com",
		},
		{
			name:       "IP address",
			baseDomain: "example.com",
			host:       "192.168.1.1",
			want:       "192.168.1.1",
		},
		{
			name:       "IP with port",
			baseDomain: "example.com",
			host:       "192.168.1.1:8080",
			want:       "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy := newTestProxy(tt.baseDomain)
			result := proxy.extractSubdomain(tt.host)
			if result != tt.want {
				t.Errorf("expected %s, got %s", tt.want, result)
			}
		})
	}
}

func TestServeHTTP_StatusPage(t *testing.T) {
	proxy := newTestProxy("localhost")

	tests := []struct {
		name string
		host string
	}{
		{"base domain", "localhost"},
		{"base domain with port", "localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status 200, got %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "Rift") {
				t.Error("expected status page to contain 'Rift'")
			}
		})
	}
}

func TestServeHTTP_TunnelNotFound(t *testing.T) {
	proxy := newTestProxy("localhost")

	req := httptest.NewRequest("GET", "/", nil)
	req.Host = "nonexistent.localhost:8080"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "nonexistent") {
		t.Error("expected error message to contain subdomain")
	}
}

func TestServeHTTP_TunnelFound(t *testing.T) {
	reg := newTestRegistry()
	proxy := NewHTTPProxy(Config{
		Addr:       ":0",
		BaseDomain: "localhost",
		Registry:   reg,
		Logger:     newTestLogger(),
	})

	// Register a tunnel
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:0")
	_ = reg.Register(tunnel)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "abc123.localhost:8080"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	// Should fail to connect since there's no actual server
	// but the tunnel lookup should succeed and UpdateActivity should be called
	// The status will be 502 (Bad Gateway) because we can't connect to the local server
	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502 (no backend), got %d", resp.StatusCode)
	}

	// Verify activity was updated
	if tunnel.LastActive.IsZero() {
		t.Error("expected LastActive to be updated")
	}
}

func TestHandleStatusPage(t *testing.T) {
	reg := newTestRegistry()
	proxy := NewHTTPProxy(Config{
		Addr:       ":0",
		BaseDomain: "test.example.com",
		Registry:   reg,
		Logger:     newTestLogger(),
	})

	// Add some tunnels
	for i := 0; i < 3; i++ {
		subdomain := "sub" + string(rune('a'+i))
		tunnel := models.NewTunnel("id-"+subdomain, subdomain, models.ProtocolHTTP, "127.0.0.1:0")
		_ = reg.Register(tunnel)
	}

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	proxy.handleStatusPage(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check for expected content
	if !strings.Contains(bodyStr, "test.example.com") {
		t.Error("expected status page to contain base domain")
	}
	if !strings.Contains(bodyStr, "3") {
		t.Error("expected status page to contain rift count")
	}
	if resp.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected content-type text/html, got %s", resp.Header.Get("Content-Type"))
	}
}

func TestServeHTTP_Methods(t *testing.T) {
	proxy := newTestProxy("localhost")

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			req.Host = "localhost"
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			// Should return status page for base domain regardless of method
			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status 200 for method %s, got %d", method, resp.StatusCode)
			}
		})
	}
}

func TestServeHTTP_LogsRequest(t *testing.T) {
	proxy := newTestProxy("localhost")

	req := httptest.NewRequest("POST", "/api/users", nil)
	req.Host = "test.localhost:8080"
	w := httptest.NewRecorder()

	// Should not panic and should log the request
	proxy.ServeHTTP(w, req)

	// Request should be handled (404 for non-existent tunnel)
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}
