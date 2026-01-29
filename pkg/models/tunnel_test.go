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
		t.Errorf("ID mismatch: got %s, want %s", tunnel.ID, id)
	}
	if tunnel.Subdomain != subdomain {
		t.Errorf("Subdomain mismatch: got %s, want %s", tunnel.Subdomain, subdomain)
	}
	if tunnel.Protocol != protocol {
		t.Errorf("Protocol mismatch: got %s, want %s", tunnel.Protocol, protocol)
	}
	if tunnel.LocalAddr != localAddr {
		t.Errorf("LocalAddr mismatch: got %s, want %s", tunnel.LocalAddr, localAddr)
	}
	if tunnel.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set automatically")
	}
	if tunnel.LastActive.IsZero() {
		t.Error("LastActive should be set automatically")
	}
	if tunnel.IsClosed() {
		t.Error("new tunnel should not be closed")
	}
}

func TestTunnel_UpdateActivity(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")
	initialLastActive := tunnel.LastActive

	// Wait a tiny bit so the timestamp will differ
	time.Sleep(10 * time.Millisecond)

	tunnel.UpdateActivity()

	if !tunnel.LastActive.After(initialLastActive) {
		t.Error("LastActive should be updated to a later time")
	}
}

func TestTunnel_SetListener(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("couldn't create test listener: %v", err)
	}
	defer listener.Close()

	tunnel.SetListener(listener)

	if tunnel.GetListener() != listener {
		t.Error("listener should be retrievable after SetListener")
	}
}

func TestTunnel_GetListener_Nil(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.GetListener() != nil {
		t.Error("listener should be nil on new tunnel")
	}
}

func TestTunnel_Close(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	// Attach a real listener so Close has something to clean up
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("couldn't create test listener: %v", err)
	}
	tunnel.SetListener(listener)

	err = tunnel.Close()

	if err != nil {
		t.Errorf("Close should succeed, got error: %v", err)
	}
	if !tunnel.IsClosed() {
		t.Error("tunnel should be marked as closed")
	}
}

func TestTunnel_Close_Idempotent(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	// Closing multiple times should be safe
	err1 := tunnel.Close()
	err2 := tunnel.Close()
	err3 := tunnel.Close()

	if err1 != nil {
		t.Errorf("first Close failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Close failed: %v", err2)
	}
	if err3 != nil {
		t.Errorf("third Close failed: %v", err3)
	}
}

func TestTunnel_IsClosed(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.IsClosed() {
		t.Error("new tunnel should not be closed")
	}

	_ = tunnel.Close()

	if !tunnel.IsClosed() {
		t.Error("tunnel should report closed after Close()")
	}
}

func TestTunnel_SetSSHConn(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	mockConn := "mock-connection"
	tunnel.SetSSHConn(mockConn)

	if tunnel.SSHConn != mockConn {
		t.Error("SSH connection should be stored")
	}
}

func TestTunnel_SetTUISession(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	mockSession := "mock-session"
	tunnel.SetTUISession(mockSession)

	if tunnel.GetTUISession() != mockSession {
		t.Error("TUI session should be retrievable")
	}
}

func TestTunnel_GetTUISession_Nil(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.GetTUISession() != nil {
		t.Error("TUI session should be nil initially")
	}
}

func TestTunnel_AddBytesReceived(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	tunnel.AddBytesReceived(100)
	tunnel.AddBytesReceived(50)

	bytesReceived, _, _ := tunnel.GetStats()
	if bytesReceived != 150 {
		t.Errorf("bytesReceived should be 150, got %d", bytesReceived)
	}
}

func TestTunnel_AddBytesSent(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	tunnel.AddBytesSent(200)
	tunnel.AddBytesSent(75)

	_, bytesSent, _ := tunnel.GetStats()
	if bytesSent != 275 {
		t.Errorf("bytesSent should be 275, got %d", bytesSent)
	}
}

func TestTunnel_IncrementRequests(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	tunnel.IncrementRequests()
	tunnel.IncrementRequests()
	tunnel.IncrementRequests()

	_, _, totalRequests := tunnel.GetStats()
	if totalRequests != 3 {
		t.Errorf("totalRequests should be 3, got %d", totalRequests)
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
		t.Errorf("bytesReceived should be 1000, got %d", bytesReceived)
	}
	if bytesSent != 500 {
		t.Errorf("bytesSent should be 500, got %d", bytesSent)
	}
	if totalRequests != 2 {
		t.Errorf("totalRequests should be 2, got %d", totalRequests)
	}
}

func TestTunnel_GetActiveConnections_Initial(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")

	if tunnel.GetActiveConnections() != 0 {
		t.Errorf("new tunnel should have 0 active connections, got %d", tunnel.GetActiveConnections())
	}
}

func TestTunnel_Concurrent(t *testing.T) {
	tunnel := NewTunnel("test-id", "abc123", ProtocolHTTP, "127.0.0.1:8080")
	var wg sync.WaitGroup

	// Hammer the tunnel with concurrent stat updates
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

	// Also do concurrent reads
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
		t.Errorf("bytesReceived should be 1000 (100x10), got %d", bytesReceived)
	}
	if bytesSent != 500 {
		t.Errorf("bytesSent should be 500 (100x5), got %d", bytesSent)
	}
	if totalRequests != 100 {
		t.Errorf("totalRequests should be 100, got %d", totalRequests)
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

	// LastActive should be very recent
	if time.Since(tunnel.LastActive) > time.Second {
		t.Error("LastActive should be within the last second after all those updates")
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
				t.Errorf("protocol constant mismatch: got %s, want %s", string(tt.protocol), tt.expected)
			}
		})
	}
}
