package ssh

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mohamed-rekiba/rift/internal/registry"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestRegistry() *registry.Registry {
	return registry.NewRegistry(newTestLogger(), 10*time.Minute)
}

func TestNewServer(t *testing.T) {
	reg := newTestRegistry()
	logger := newTestLogger()

	server, err := NewServer(Config{
		Addr:        ":0",
		BaseDomain:  "localhost",
		Registry:    reg,
		Logger:      logger,
		IdleTimeout: 5 * time.Minute,
		MaxTimeout:  30 * time.Minute,
		HTTPAddr:    ":8080",
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.addr != ":0" {
		t.Errorf("expected addr :0, got %s", server.addr)
	}
	if server.baseDomain != "localhost" {
		t.Errorf("expected baseDomain localhost, got %s", server.baseDomain)
	}
	if server.httpAddr != ":8080" {
		t.Errorf("expected httpAddr :8080, got %s", server.httpAddr)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 100, "100 B"},
		{"1023 bytes", 1023, "1023 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1024 * 1024, "1.0 MB"},
		{"1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"10 GB", 10 * 1024 * 1024 * 1024, "10.0 GB"},
		{"mixed KB", 2500, "2.4 KB"},
		{"mixed MB", 5*1024*1024 + 512*1024, "5.5 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.want {
				t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.want)
			}
		})
	}
}

func TestResolveBindAddress(t *testing.T) {
	tests := []struct {
		name          string
		requestedAddr string
		wantRequested string
		wantActual    string
	}{
		{
			name:          "empty address",
			requestedAddr: "",
			wantRequested: "localhost",
			wantActual:    "127.0.0.1",
		},
		{
			name:          "localhost",
			requestedAddr: "localhost",
			wantRequested: "localhost",
			wantActual:    "127.0.0.1",
		},
		{
			name:          "specific IP",
			requestedAddr: "192.168.1.1",
			wantRequested: "192.168.1.1",
			wantActual:    "192.168.1.1",
		},
		{
			name:          "all interfaces",
			requestedAddr: "0.0.0.0",
			wantRequested: "0.0.0.0",
			wantActual:    "0.0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requested, actual := resolveBindAddress(tt.requestedAddr)
			if requested != tt.wantRequested {
				t.Errorf("requested address: expected %s, got %s", tt.wantRequested, requested)
			}
			if actual != tt.wantActual {
				t.Errorf("actual address: expected %s, got %s", tt.wantActual, actual)
			}
		})
	}
}

func TestGenerateHostKey(t *testing.T) {
	signer, err := generateHostKey()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if signer == nil {
		t.Fatal("expected non-nil signer")
	}

	// Verify the public key can be extracted
	pubKey := signer.PublicKey()
	if pubKey == nil {
		t.Error("expected non-nil public key")
	}

	// Verify key type
	keyType := pubKey.Type()
	if keyType != "ssh-rsa" {
		t.Errorf("expected key type ssh-rsa, got %s", keyType)
	}
}

func TestGenerateHostKey_UniqueKeys(t *testing.T) {
	signer1, err1 := generateHostKey()
	signer2, err2 := generateHostKey()

	if err1 != nil || err2 != nil {
		t.Fatal("expected no errors generating keys")
	}

	// Each generated key should be different
	key1 := signer1.PublicKey().Marshal()
	key2 := signer2.PublicKey().Marshal()

	if string(key1) == string(key2) {
		t.Error("expected different keys for each generation")
	}
}

func TestServer_GetSSHConnection_NotFound(t *testing.T) {
	server, err := NewServer(Config{
		Addr:        ":0",
		BaseDomain:  "localhost",
		Registry:    newTestRegistry(),
		Logger:      newTestLogger(),
		IdleTimeout: 5 * time.Minute,
		MaxTimeout:  30 * time.Minute,
		HTTPAddr:    ":8080",
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	conn, ok := server.GetSSHConnection("nonexistent-session")

	if ok {
		t.Error("expected ok to be false for nonexistent session")
	}
	if conn != nil {
		t.Error("expected nil connection for nonexistent session")
	}
}

func TestServer_SessionTracking(t *testing.T) {
	server, err := NewServer(Config{
		Addr:        ":0",
		BaseDomain:  "localhost",
		Registry:    newTestRegistry(),
		Logger:      newTestLogger(),
		IdleTimeout: 5 * time.Minute,
		MaxTimeout:  30 * time.Minute,
		HTTPAddr:    ":8080",
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Initial state should have empty maps
	server.sessionsMu.RLock()
	sessionCount := len(server.sessions)
	connCount := len(server.sshConns)
	server.sessionsMu.RUnlock()

	if sessionCount != 0 {
		t.Errorf("expected 0 sessions initially, got %d", sessionCount)
	}
	if connCount != 0 {
		t.Errorf("expected 0 connections initially, got %d", connCount)
	}
}

func TestServer_CloseAllSessions_Empty(t *testing.T) {
	server, err := NewServer(Config{
		Addr:        ":0",
		BaseDomain:  "localhost",
		Registry:    newTestRegistry(),
		Logger:      newTestLogger(),
		IdleTimeout: 5 * time.Minute,
		MaxTimeout:  30 * time.Minute,
		HTTPAddr:    ":8080",
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Should not panic when no sessions
	server.CloseAllSessions()
}

func TestServer_ReversePortForwardingCallback(t *testing.T) {
	server, err := NewServer(Config{
		Addr:        ":0",
		BaseDomain:  "localhost",
		Registry:    newTestRegistry(),
		Logger:      newTestLogger(),
		IdleTimeout: 5 * time.Minute,
		MaxTimeout:  30 * time.Minute,
		HTTPAddr:    ":8080",
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// The callback should always return true (allow forwarding)
	// Note: We can't directly test the callback without a full SSH context,
	// but we can verify the server is configured correctly
	if server.sshServer == nil {
		t.Error("expected SSH server to be initialized")
	}
}

func TestConfig_Fields(t *testing.T) {
	config := Config{
		Addr:        ":2222",
		BaseDomain:  "example.com",
		Registry:    nil,
		Logger:      nil,
		IdleTimeout: 5 * time.Minute,
		MaxTimeout:  30 * time.Minute,
		HTTPAddr:    ":8080",
	}

	if config.Addr != ":2222" {
		t.Errorf("expected Addr :2222, got %s", config.Addr)
	}
	if config.BaseDomain != "example.com" {
		t.Errorf("expected BaseDomain example.com, got %s", config.BaseDomain)
	}
	if config.IdleTimeout != 5*time.Minute {
		t.Errorf("expected IdleTimeout 5m, got %v", config.IdleTimeout)
	}
	if config.MaxTimeout != 30*time.Minute {
		t.Errorf("expected MaxTimeout 30m, got %v", config.MaxTimeout)
	}
	if config.HTTPAddr != ":8080" {
		t.Errorf("expected HTTPAddr :8080, got %s", config.HTTPAddr)
	}
}

func TestForwardingConfig(t *testing.T) {
	config := forwardingConfig{
		requestedAddr: "localhost",
		allocatedPort: 12345,
		listener:      nil,
		sshConn:       nil,
		tunnel:        nil,
	}

	if config.requestedAddr != "localhost" {
		t.Errorf("expected requestedAddr localhost, got %s", config.requestedAddr)
	}
	if config.allocatedPort != 12345 {
		t.Errorf("expected allocatedPort 12345, got %d", config.allocatedPort)
	}
}

func TestErrServerClosed(t *testing.T) {
	// Verify the error constant is properly exported
	if ErrServerClosed == nil {
		t.Error("expected ErrServerClosed to be non-nil")
	}
}
