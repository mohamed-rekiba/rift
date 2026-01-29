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
		t.Fatalf("NewServer should succeed, got error: %v", err)
	}
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.addr != ":0" {
		t.Errorf("addr: got %s, want :0", server.addr)
	}
	if server.baseDomain != "localhost" {
		t.Errorf("baseDomain: got %s, want localhost", server.baseDomain)
	}
	if server.httpAddr != ":8080" {
		t.Errorf("httpAddr: got %s, want :8080", server.httpAddr)
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
		{"just under 1KB", 1023, "1023 B"},
		{"exactly 1KB", 1024, "1.0 KB"},
		{"1.5KB", 1536, "1.5 KB"},
		{"1MB", 1024 * 1024, "1.0 MB"},
		{"1GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"10GB", 10 * 1024 * 1024 * 1024, "10.0 GB"},
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
			name:          "empty defaults to localhost",
			requestedAddr: "",
			wantRequested: "localhost",
			wantActual:    "127.0.0.1",
		},
		{
			name:          "localhost resolves to 127.0.0.1",
			requestedAddr: "localhost",
			wantRequested: "localhost",
			wantActual:    "127.0.0.1",
		},
		{
			name:          "specific IP stays as-is",
			requestedAddr: "192.168.1.1",
			wantRequested: "192.168.1.1",
			wantActual:    "192.168.1.1",
		},
		{
			name:          "all interfaces stays as-is",
			requestedAddr: "0.0.0.0",
			wantRequested: "0.0.0.0",
			wantActual:    "0.0.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requested, actual := resolveBindAddress(tt.requestedAddr)
			if requested != tt.wantRequested {
				t.Errorf("requested address: got %s, want %s", requested, tt.wantRequested)
			}
			if actual != tt.wantActual {
				t.Errorf("actual address: got %s, want %s", actual, tt.wantActual)
			}
		})
	}
}

func TestGenerateHostKey(t *testing.T) {
	signer, err := generateHostKey()

	if err != nil {
		t.Fatalf("generateHostKey should succeed: %v", err)
	}
	if signer == nil {
		t.Fatal("signer should not be nil")
	}

	// Verify we can get the public key
	pubKey := signer.PublicKey()
	if pubKey == nil {
		t.Error("public key should be extractable")
	}

	// Check key type
	keyType := pubKey.Type()
	if keyType != "ssh-rsa" {
		t.Errorf("key type should be ssh-rsa, got %s", keyType)
	}
}

func TestGenerateHostKey_UniqueKeys(t *testing.T) {
	signer1, err1 := generateHostKey()
	signer2, err2 := generateHostKey()

	if err1 != nil || err2 != nil {
		t.Fatal("both key generations should succeed")
	}

	// Each call should produce a different key
	key1 := signer1.PublicKey().Marshal()
	key2 := signer2.PublicKey().Marshal()

	if string(key1) == string(key2) {
		t.Error("each call should generate a unique key")
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
		t.Fatalf("NewServer failed: %v", err)
	}

	conn, ok := server.GetSSHConnection("nonexistent-session")

	if ok {
		t.Error("should not find a connection that doesn't exist")
	}
	if conn != nil {
		t.Error("connection should be nil for unknown session")
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
		t.Fatalf("NewServer failed: %v", err)
	}

	// New server should have no sessions
	server.sessionsMu.RLock()
	sessionCount := len(server.sessions)
	connCount := len(server.sshConns)
	server.sessionsMu.RUnlock()

	if sessionCount != 0 {
		t.Errorf("new server should have 0 sessions, got %d", sessionCount)
	}
	if connCount != 0 {
		t.Errorf("new server should have 0 SSH connections, got %d", connCount)
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
		t.Fatalf("NewServer failed: %v", err)
	}

	// Should not panic when there are no sessions to close
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
		t.Fatalf("NewServer failed: %v", err)
	}

	// Server should have an SSH server configured
	if server.sshServer == nil {
		t.Error("SSH server should be initialized")
	}
}

func TestConfig_Fields(t *testing.T) {
	config := Config{
		Addr:        ":2222",
		BaseDomain:  "example.com",
		IdleTimeout: 5 * time.Minute,
		MaxTimeout:  30 * time.Minute,
		HTTPAddr:    ":8080",
	}

	if config.Addr != ":2222" {
		t.Errorf("Addr: got %s, want :2222", config.Addr)
	}
	if config.BaseDomain != "example.com" {
		t.Errorf("BaseDomain: got %s, want example.com", config.BaseDomain)
	}
	if config.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout: got %v, want 5m", config.IdleTimeout)
	}
	if config.MaxTimeout != 30*time.Minute {
		t.Errorf("MaxTimeout: got %v, want 30m", config.MaxTimeout)
	}
	if config.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr: got %s, want :8080", config.HTTPAddr)
	}
}

func TestForwardingConfig(t *testing.T) {
	config := forwardingConfig{
		requestedAddr: "localhost",
		allocatedPort: 12345,
	}

	if config.requestedAddr != "localhost" {
		t.Errorf("requestedAddr: got %s, want localhost", config.requestedAddr)
	}
	if config.allocatedPort != 12345 {
		t.Errorf("allocatedPort: got %d, want 12345", config.allocatedPort)
	}
}

func TestErrServerClosed(t *testing.T) {
	if ErrServerClosed == nil {
		t.Error("ErrServerClosed should be defined")
	}
}
