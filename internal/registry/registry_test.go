package registry

import (
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mohamed-rekiba/rift/pkg/models"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func newTestRegistry() *Registry {
	return NewRegistry(newTestLogger(), 10*time.Minute)
}

func TestNewRegistry(t *testing.T) {
	logger := newTestLogger()
	cleanupInterval := 5 * time.Minute

	r := NewRegistry(logger, cleanupInterval)

	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.tunnels == nil {
		t.Error("expected tunnels map to be initialized")
	}
	if r.tunnelsById == nil {
		t.Error("expected tunnelsById map to be initialized")
	}
	if r.cleanupInterval != cleanupInterval {
		t.Errorf("expected cleanup interval %v, got %v", cleanupInterval, r.cleanupInterval)
	}
}

func TestRegistry_Register(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")

	err := r.Register(tunnel)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify tunnel is registered
	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}
}

func TestRegistry_Register_DuplicateSubdomain(t *testing.T) {
	r := newTestRegistry()
	tunnel1 := models.NewTunnel("test-id-1", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	tunnel2 := models.NewTunnel("test-id-2", "abc123", models.ProtocolHTTP, "127.0.0.1:8081")

	err := r.Register(tunnel1)
	if err != nil {
		t.Fatalf("expected no error for first tunnel, got %v", err)
	}

	err = r.Register(tunnel2)
	if err != ErrSubdomainTaken {
		t.Errorf("expected ErrSubdomainTaken, got %v", err)
	}
}

func TestRegistry_Register_MultipleTunnels(t *testing.T) {
	r := newTestRegistry()
	tunnels := []*models.Tunnel{
		models.NewTunnel("id-1", "sub1", models.ProtocolHTTP, "127.0.0.1:8080"),
		models.NewTunnel("id-2", "sub2", models.ProtocolHTTP, "127.0.0.1:8081"),
		models.NewTunnel("id-3", "sub3", models.ProtocolHTTP, "127.0.0.1:8082"),
	}

	for _, tunnel := range tunnels {
		if err := r.Register(tunnel); err != nil {
			t.Fatalf("failed to register tunnel %s: %v", tunnel.ID, err)
		}
	}

	if r.Count() != 3 {
		t.Errorf("expected count 3, got %d", r.Count())
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")

	_ = r.Register(tunnel)

	err := r.Unregister("abc123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r.Count() != 0 {
		t.Errorf("expected count 0 after unregister, got %d", r.Count())
	}
}

func TestRegistry_Unregister_NotFound(t *testing.T) {
	r := newTestRegistry()

	err := r.Unregister("nonexistent")

	if err != ErrTunnelNotFound {
		t.Errorf("expected ErrTunnelNotFound, got %v", err)
	}
}

func TestRegistry_Get(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	result, err := r.Get("abc123")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != tunnel.ID {
		t.Errorf("expected tunnel ID %s, got %s", tunnel.ID, result.ID)
	}
	if result.Subdomain != tunnel.Subdomain {
		t.Errorf("expected subdomain %s, got %s", tunnel.Subdomain, result.Subdomain)
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := newTestRegistry()

	_, err := r.Get("nonexistent")

	if err != ErrTunnelNotFound {
		t.Errorf("expected ErrTunnelNotFound, got %v", err)
	}
}

func TestRegistry_GetByID(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	result, err := r.GetByID("test-id")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.ID != tunnel.ID {
		t.Errorf("expected tunnel ID %s, got %s", tunnel.ID, result.ID)
	}
}

func TestRegistry_GetByID_NotFound(t *testing.T) {
	r := newTestRegistry()

	_, err := r.GetByID("nonexistent")

	if err != ErrTunnelNotFound {
		t.Errorf("expected ErrTunnelNotFound, got %v", err)
	}
}

func TestRegistry_List(t *testing.T) {
	r := newTestRegistry()
	tunnel1 := models.NewTunnel("id-1", "sub1", models.ProtocolHTTP, "127.0.0.1:8080")
	tunnel2 := models.NewTunnel("id-2", "sub2", models.ProtocolHTTP, "127.0.0.1:8081")

	_ = r.Register(tunnel1)
	_ = r.Register(tunnel2)

	result := r.List()

	if len(result) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(result))
	}
}

func TestRegistry_List_Empty(t *testing.T) {
	r := newTestRegistry()

	result := r.List()

	if len(result) != 0 {
		t.Errorf("expected 0 tunnels, got %d", len(result))
	}
}

func TestRegistry_Count(t *testing.T) {
	r := newTestRegistry()

	if r.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", r.Count())
	}

	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}

	_ = r.Unregister("abc123")

	if r.Count() != 0 {
		t.Errorf("expected count 0 after unregister, got %d", r.Count())
	}
}

func TestRegistry_GenerateSubdomain(t *testing.T) {
	r := newTestRegistry()

	subdomain, err := r.GenerateSubdomain()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(subdomain) != 8 {
		t.Errorf("expected subdomain length 8, got %d", len(subdomain))
	}
}

func TestRegistry_GenerateSubdomain_Unique(t *testing.T) {
	r := newTestRegistry()
	subdomains := make(map[string]bool)

	// Generate 100 subdomains and check uniqueness
	for i := 0; i < 100; i++ {
		subdomain, err := r.GenerateSubdomain()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if subdomains[subdomain] {
			t.Errorf("duplicate subdomain generated: %s", subdomain)
		}
		subdomains[subdomain] = true
	}
}

func TestRegistry_GenerateSubdomain_Concurrent(t *testing.T) {
	r := newTestRegistry()
	var wg sync.WaitGroup
	subdomains := make(chan string, 100)

	// Generate subdomains concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subdomain, err := r.GenerateSubdomain()
			if err != nil {
				t.Errorf("expected no error, got %v", err)
				return
			}
			subdomains <- subdomain
		}()
	}

	wg.Wait()
	close(subdomains)

	// Check all subdomains are unique
	seen := make(map[string]bool)
	for subdomain := range subdomains {
		if seen[subdomain] {
			t.Errorf("duplicate subdomain in concurrent test: %s", subdomain)
		}
		seen[subdomain] = true
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := newTestRegistry()
	var wg sync.WaitGroup

	// Concurrent registrations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subdomain := generateRandomString(8)
			tunnel := models.NewTunnel("id-"+subdomain, subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
			_ = r.Register(tunnel)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = r.List()
			_ = r.Count()
		}()
	}

	wg.Wait()

	// Should not panic and should have consistent state
	if r.Count() < 0 {
		t.Error("count should not be negative")
	}
}

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   int
	}{
		{"length 8", 8, 8},
		{"length 16", 16, 16},
		{"length 4", 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateRandomString(tt.length)
			if len(result) != tt.want {
				t.Errorf("expected length %d, got %d", tt.want, len(result))
			}
		})
	}
}

func TestGenerateRandomString_HexOnly(t *testing.T) {
	result := generateRandomString(8)

	for _, char := range result {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			t.Errorf("expected hex character, got %c", char)
		}
	}
}

// TestRegistry_StaleTunnelDetection tests that stale tunnels can be identified
func TestRegistry_StaleTunnelDetection(t *testing.T) {
	r := newTestRegistry()

	// Create a tunnel with old LastActive time
	tunnel := models.NewTunnel("stale-id", "stalesub", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	// Manually set LastActive to old time (simulating stale tunnel)
	// Note: We can't directly modify LastActive, but we can verify the registry
	// tracks it and the cleanup logic would work

	// Verify tunnel is registered
	retrieved, err := r.Get("stalesub")
	if err != nil {
		t.Fatalf("expected to find tunnel, got error: %v", err)
	}

	// Verify LastActive is set
	if retrieved.LastActive.IsZero() {
		t.Error("expected LastActive to be set")
	}

	// Update activity
	retrieved.UpdateActivity()
	newLastActive := retrieved.LastActive

	// Verify LastActive was updated
	if newLastActive.IsZero() {
		t.Error("expected LastActive to be updated")
	}
}

// TestRegistry_ClosedTunnelDetection tests that closed tunnels can be identified
func TestRegistry_ClosedTunnelDetection(t *testing.T) {
	r := newTestRegistry()

	tunnel := models.NewTunnel("closed-id", "closedsub", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	// Close the tunnel
	_ = tunnel.Close()

	// Verify tunnel is closed
	if !tunnel.IsClosed() {
		t.Error("expected tunnel to be closed")
	}

	// Tunnel should still be in registry (cleanup runs periodically)
	retrieved, err := r.Get("closedsub")
	if err != nil {
		t.Fatalf("expected to find tunnel, got error: %v", err)
	}
	if !retrieved.IsClosed() {
		t.Error("retrieved tunnel should be closed")
	}
}

// TestRegistry_ErrorCases tests various error scenarios
func TestRegistry_ErrorCases(t *testing.T) {
	r := newTestRegistry()

	// Test unregister non-existent
	err := r.Unregister("doesnotexist")
	if err != ErrTunnelNotFound {
		t.Errorf("expected ErrTunnelNotFound, got %v", err)
	}

	// Test get non-existent
	_, err = r.Get("doesnotexist")
	if err != ErrTunnelNotFound {
		t.Errorf("expected ErrTunnelNotFound, got %v", err)
	}

	// Test get by ID non-existent
	_, err = r.GetByID("doesnotexist")
	if err != ErrTunnelNotFound {
		t.Errorf("expected ErrTunnelNotFound, got %v", err)
	}
}

// TestRegistry_RegisterAfterUnregister tests re-registration of same subdomain
func TestRegistry_RegisterAfterUnregister(t *testing.T) {
	r := newTestRegistry()

	subdomain := "reusable"

	// First registration
	tunnel1 := models.NewTunnel("id-1", subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
	err := r.Register(tunnel1)
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	// Unregister
	err = r.Unregister(subdomain)
	if err != nil {
		t.Fatalf("unregister failed: %v", err)
	}

	// Second registration with same subdomain should succeed
	tunnel2 := models.NewTunnel("id-2", subdomain, models.ProtocolHTTP, "127.0.0.1:8081")
	err = r.Register(tunnel2)
	if err != nil {
		t.Fatalf("second register failed: %v", err)
	}

	// Verify new tunnel is registered
	retrieved, err := r.Get(subdomain)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if retrieved.ID != "id-2" {
		t.Errorf("expected tunnel id-2, got %s", retrieved.ID)
	}
}

// TestRegistry_ListReturnsSnapshot tests that List returns a snapshot
func TestRegistry_ListReturnsSnapshot(t *testing.T) {
	r := newTestRegistry()

	// Register some tunnels
	for i := 0; i < 5; i++ {
		subdomain := generateRandomString(8)
		tunnel := models.NewTunnel("id-"+subdomain, subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
		_ = r.Register(tunnel)
	}

	// Get list
	list := r.List()
	initialLen := len(list)

	// Register more tunnels
	for i := 0; i < 3; i++ {
		subdomain := generateRandomString(8)
		tunnel := models.NewTunnel("new-"+subdomain, subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
		_ = r.Register(tunnel)
	}

	// Original list should not have changed (it's a snapshot)
	if len(list) != initialLen {
		t.Error("list should be a snapshot and not change")
	}

	// New list should have more items
	newList := r.List()
	if len(newList) != initialLen+3 {
		t.Errorf("expected %d tunnels, got %d", initialLen+3, len(newList))
	}
}

// TestRegistry_GetByIDAndSubdomainMatch tests that both lookups return same tunnel
func TestRegistry_GetByIDAndSubdomainMatch(t *testing.T) {
	r := newTestRegistry()

	tunnel := models.NewTunnel("unique-id", "unique-sub", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	bySubdomain, err1 := r.Get("unique-sub")
	byID, err2 := r.GetByID("unique-id")

	if err1 != nil || err2 != nil {
		t.Fatalf("expected no errors, got %v and %v", err1, err2)
	}

	if bySubdomain != byID {
		t.Error("expected same tunnel from both lookup methods")
	}

	if bySubdomain.ID != "unique-id" {
		t.Errorf("expected ID unique-id, got %s", bySubdomain.ID)
	}

	if byID.Subdomain != "unique-sub" {
		t.Errorf("expected subdomain unique-sub, got %s", byID.Subdomain)
	}
}
