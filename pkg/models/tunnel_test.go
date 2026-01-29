package models

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestNewTunnel(t *testing.T) {
	id := "test-id"
	subdomain := "abc123"
	protocol := ProtocolHTTP
	localAddr := "127.0.0.1:8080"

	tunnel := NewTunnel(id, subdomain, protocol, localAddr)

	if tunnel.ID != id {
		t.Errorf("expected ID %s, got %s", id, tunnel.ID)
	}
	if tunnel.Subdomain != subdomain {
		t.Errorf("expected Subdomain %s, got %s", subdomain, tunnel.Subdomain)
	}
	if tunnel.Protocol != protocol {
		t.Errorf("expected Protocol %s, got %s", protocol, tunnel.Protocol)
	}
	if tunnel.LocalAddr != localAddr {
		t.Errorf("expected LocalAddr %s, got %s", localAddr, tunnel.LocalAddr)
	}
	if tunnel.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if tunnel.LastActive.IsZero() {
		t.Error("expected LastActive to be set")
	}
	if tunnel.IsClosed() {
		t.Error("expected tunnel to not be closed initially")
	}
}

func TestTunnel_UpdateActivity(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")
	initialLastActive := tunnel.LastActive

	// Wait a bit to ensure time changes
	time.Sleep(10 * time.Millisecond)

	tunnel.UpdateActivity()

	if !tunnel.LastActive.After(initialLastActive) {
		t.Error("expected LastActive to be updated to a later time")
	}
}

func TestTunnel_SetListener(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	// Create a mock listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	tunnel.SetListener(listener)

	if tunnel.GetListener() != listener {
		t.Error("expected listener to be set")
	}
}

func TestTunnel_GetListener_Nil(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.GetListener() != nil {
		t.Error("expected nil listener initially")
	}
}

func TestTunnel_Close(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	// Create and set a listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	tunnel.SetListener(listener)

	err = tunnel.Close()

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if !tunnel.IsClosed() {
		t.Error("expected tunnel to be closed")
	}
}

func TestTunnel_Close_Idempotent(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	// Close multiple times should not panic
	err1 := tunnel.Close()
	err2 := tunnel.Close()
	err3 := tunnel.Close()

	if err1 != nil {
		t.Errorf("first close: expected no error, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("second close: expected no error, got %v", err2)
	}
	if err3 != nil {
		t.Errorf("third close: expected no error, got %v", err3)
	}
}

func TestTunnel_IsClosed(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.IsClosed() {
		t.Error("expected tunnel to not be closed initially")
	}

	_ = tunnel.Close()

	if !tunnel.IsClosed() {
		t.Error("expected tunnel to be closed after Close()")
	}
}

func TestTunnel_SetSSHConn(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	mockConn := "mock-connection"
	tunnel.SetSSHConn(mockConn)

	if tunnel.SSHConn != mockConn {
		t.Error("expected SSH connection to be set")
	}
}

func TestTunnel_SetTUISession(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	mockSession := "mock-session"
	tunnel.SetTUISession(mockSession)

	if tunnel.GetTUISession() != mockSession {
		t.Error("expected TUI session to be set")
	}
}

func TestTunnel_GetTUISession_Nil(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.GetTUISession() != nil {
		t.Error("expected nil TUI session initially")
	}
}

func TestTunnel_AddBytesReceived(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	tunnel.AddBytesReceived(100)
	tunnel.AddBytesReceived(50)

	bytesReceived, _, _ := tunnel.GetStats()
	if bytesReceived != 150 {
		t.Errorf("expected bytesReceived 150, got %d", bytesReceived)
	}
}

func TestTunnel_AddBytesSent(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	tunnel.AddBytesSent(200)
	tunnel.AddBytesSent(75)

	_, bytesSent, _ := tunnel.GetStats()
	if bytesSent != 275 {
		t.Errorf("expected bytesSent 275, got %d", bytesSent)
	}
}

func TestTunnel_IncrementRequests(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	tunnel.IncrementRequests()
	tunnel.IncrementRequests()
	tunnel.IncrementRequests()

	_, _, totalRequests := tunnel.GetStats()
	if totalRequests != 3 {
		t.Errorf("expected totalRequests 3, got %d", totalRequests)
	}
}

func TestTunnel_GetStats(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	tunnel.AddBytesReceived(1000)
	tunnel.AddBytesSent(500)
	tunnel.IncrementRequests()
	tunnel.IncrementRequests()

	bytesReceived, bytesSent, totalRequests := tunnel.GetStats()

	if bytesReceived != 1000 {
		t.Errorf("expected bytesReceived 1000, got %d", bytesReceived)
	}
	if bytesSent != 500 {
		t.Errorf("expected bytesSent 500, got %d", bytesSent)
	}
	if totalRequests != 2 {
		t.Errorf("expected totalRequests 2, got %d", totalRequests)
	}
}

func TestTunnel_GetActiveConnections_Initial(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.GetActiveConnections() != 0 {
		t.Errorf("expected 0 active connections initially, got %d", tunnel.GetActiveConnections())
	}
}

func TestTunnel_Concurrent(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")
	var wg sync.WaitGroup

	// Concurrent stat updates
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			tunnel.AddBytesReceived(10)
		}()
		go func() {
			defer wg.Done()
			tunnel.AddBytesSent(5)
		}()
		go func() {
			defer wg.Done()
			tunnel.IncrementRequests()
		}()
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tunnel.GetStats()
			tunnel.IsClosed()
			tunnel.GetActiveConnections()
		}()
	}

	wg.Wait()

	bytesReceived, bytesSent, totalRequests := tunnel.GetStats()

	if bytesReceived != 1000 {
		t.Errorf("expected bytesReceived 1000, got %d", bytesReceived)
	}
	if bytesSent != 500 {
		t.Errorf("expected bytesSent 500, got %d", bytesSent)
	}
	if totalRequests != 100 {
		t.Errorf("expected totalRequests 100, got %d", totalRequests)
	}
}

func TestTunnel_UpdateActivity_Concurrent(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")
	var wg sync.WaitGroup

	// Concurrent activity updates
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tunnel.UpdateActivity()
		}()
	}

	wg.Wait()

	// Should not panic and LastActive should be recent
	if time.Since(tunnel.LastActive) > time.Second {
		t.Error("expected LastActive to be recent")
	}
}

func TestProtocol_Constants(t *testing.T) {
	tests := []struct {
		protocol Protocol
		expected string
	}{
		{ProtocolHTTP, "http"},
		{ProtocolTCP, "tcp"},
		{ProtocolUDP, "udp"},
		{ProtocolTLS, "tls"},
	}

	for _, tt := range tests {
		t.Run(string(tt.protocol), func(t *testing.T) {
			if string(tt.protocol) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(tt.protocol))
			}
		})
	}
}
