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
		t.Fatal("NewHTTPProxy returned nil")
	}
	if proxy.baseDomain != "example.com" {
		t.Errorf("baseDomain should be example.com, got %s", proxy.baseDomain)
	}
	if proxy.registry != reg {
		t.Error("registry should be the one we passed in")
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
			name:       "simple subdomain with port",
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
			name:       "localhost subdomain with port",
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
				t.Errorf("extractSubdomain(%q) = %q, want %q", tt.host, result, tt.want)
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
			name:       "simple subdomain",
			baseDomain: "example.com",
			host:       "abc123.example.com",
			want:       "abc123",
		},
		{
			name:       "nested subdomain",
			baseDomain: "example.com",
			host:       "test.abc123.example.com",
			want:       "test.abc123",
		},
		{
			name:       "localhost subdomain",
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
				t.Errorf("extractSubdomain(%q) = %q, want %q", tt.host, result, tt.want)
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
			name:       "just the base domain",
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
			name:       "localhost without subdomain",
			baseDomain: "localhost",
			host:       "localhost",
			want:       "",
		},
		{
			name:       "localhost with port only",
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
				t.Errorf("extractSubdomain(%q) = %q, want %q (empty)", tt.host, result, tt.want)
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
			name:       "completely different domain",
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
			name:       "IP address with port",
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
				t.Errorf("extractSubdomain(%q) = %q, want %q", tt.host, result, tt.want)
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
		{"base domain shows status page", "localhost"},
		{"base domain with port shows status page", "localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Host = tt.host
			w := httptest.NewRecorder()

			proxy.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status page should return 200, got %d", resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "Rift") {
				t.Error("status page should mention 'Rift'")
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
		t.Errorf("unknown subdomain should return 404, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "nonexistent") {
		t.Error("error page should mention the subdomain that wasn't found")
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

	// Register a tunnel (without a real backend)
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:0")
	_ = reg.Register(tunnel)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Host = "abc123.localhost:8080"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	// Should get 502 because there's no actual backend listening
	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("no backend should return 502, got %d", resp.StatusCode)
	}

	// But the tunnel's activity should still be updated
	if tunnel.LastActive.IsZero() {
		t.Error("tunnel activity should be updated even when backend fails")
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

	// Add a few tunnels
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
		t.Errorf("status page should return 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "test.example.com") {
		t.Error("status page should show the base domain")
	}
	if !strings.Contains(bodyStr, "3") {
		t.Error("status page should show the tunnel count")
	}
	if resp.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type should be text/html, got %s", resp.Header.Get("Content-Type"))
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

			// Base domain should return status page for any method
			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s request should return 200, got %d", method, resp.StatusCode)
			}
		})
	}
}

func TestServeHTTP_LogsRequest(t *testing.T) {
	proxy := newTestProxy("localhost")

	req := httptest.NewRequest("POST", "/api/users", nil)
	req.Host = "test.localhost:8080"
	w := httptest.NewRecorder()

	// This shouldn't panic
	proxy.ServeHTTP(w, req)

	// Should get 404 since "test" subdomain doesn't exist
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown subdomain should return 404, got %d", resp.StatusCode)
	}
}
