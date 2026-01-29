package models

import (
	"net"
	"sync"
	"time"
)

// Protocol defines the type of traffic a tunnel carries.
type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
	ProtocolTLS  Protocol = "tls"
)

// Tunnel represents a single active tunnel from a user's local server to the public internet.
// Each tunnel has a unique subdomain and tracks statistics about the traffic flowing through it.
type Tunnel struct {
	ID         string    // Unique identifier combining user and session
	Subdomain  string    // The random subdomain assigned (e.g., "abc12345")
	Protocol   Protocol  // What kind of traffic this tunnel carries
	LocalAddr  string    // The local address we're listening on for this tunnel
	CreatedAt  time.Time // When the tunnel was created
	LastActive time.Time // Last time traffic flowed through

	mu            sync.RWMutex
	listener      net.Listener
	connections   map[string]net.Conn
	closeChan     chan struct{}
	closed        bool
	bytesReceived int64
	bytesSent     int64
	totalRequests int

	SSHConn    interface{} // The underlying SSH connection
	TUISession interface{} // The interactive dashboard session (if running)
}

// NewTunnel creates a fresh tunnel ready to accept connections.
func NewTunnel(id, subdomain string, protocol Protocol, localAddr string) *Tunnel {
	return &Tunnel{
		ID:          id,
		Subdomain:   subdomain,
		Protocol:    protocol,
		LocalAddr:   localAddr,
		CreatedAt:   time.Now(),
		LastActive:  time.Now(),
		connections: make(map[string]net.Conn),
		closeChan:   make(chan struct{}),
	}
}

// UpdateActivity marks the tunnel as recently used (prevents cleanup).
func (t *Tunnel) UpdateActivity() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.LastActive = time.Now()
}

// SetListener stores the network listener that accepts incoming connections.
func (t *Tunnel) SetListener(listener net.Listener) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.listener = listener
}

// GetListener returns the network listener for this tunnel.
func (t *Tunnel) GetListener() net.Listener {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.listener
}

// Close shuts down the tunnel and cleans up all resources.
func (t *Tunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true
	close(t.closeChan)

	if t.listener != nil {
		_ = t.listener.Close()
	}

	// Close all active connections
	for _, conn := range t.connections {
		_ = conn.Close()
	}

	return nil
}

// IsClosed returns true if the tunnel has been shut down.
func (t *Tunnel) IsClosed() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.closed
}

// SetSSHConn stores the SSH connection used for this tunnel.
func (t *Tunnel) SetSSHConn(conn interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.SSHConn = conn
}

// SetTUISession stores the interactive dashboard session.
func (t *Tunnel) SetTUISession(session interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.TUISession = session
}

// GetTUISession returns the interactive dashboard session if one is running.
func (t *Tunnel) GetTUISession() interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.TUISession
}

// AddBytesReceived tracks data received from the user's local server.
func (t *Tunnel) AddBytesReceived(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bytesReceived += bytes
}

// AddBytesSent tracks data sent to the user's local server.
func (t *Tunnel) AddBytesSent(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bytesSent += bytes
}

// IncrementRequests bumps the request counter.
func (t *Tunnel) IncrementRequests() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalRequests++
}

// GetStats returns the tunnel's traffic statistics.
func (t *Tunnel) GetStats() (bytesReceived, bytesSent int64, totalRequests int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.bytesReceived, t.bytesSent, t.totalRequests
}

// GetActiveConnections returns how many connections are currently open.
func (t *Tunnel) GetActiveConnections() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.connections)
}
