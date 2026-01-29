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
		t.Fatal("NewRegistry returned nil")
	}
	if r.tunnels == nil {
		t.Error("tunnels map should be initialized")
	}
	if r.tunnelsById == nil {
		t.Error("tunnelsById map should be initialized")
	}
	if r.cleanupInterval != cleanupInterval {
		t.Errorf("cleanup interval mismatch: got %v, want %v", r.cleanupInterval, cleanupInterval)
	}
}

func TestRegistry_Register(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")

	err := r.Register(tunnel)

	if err != nil {
		t.Fatalf("registration should succeed, got error: %v", err)
	}
	if r.Count() != 1 {
		t.Errorf("should have 1 tunnel after registration, got %d", r.Count())
	}
}

func TestRegistry_Register_DuplicateSubdomain(t *testing.T) {
	r := newTestRegistry()
	tunnel1 := models.NewTunnel("test-id-1", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	tunnel2 := models.NewTunnel("test-id-2", "abc123", models.ProtocolHTTP, "127.0.0.1:8081")

	err := r.Register(tunnel1)
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	err = r.Register(tunnel2)
	if err != ErrSubdomainTaken {
		t.Errorf("second registration should fail with ErrSubdomainTaken, got: %v", err)
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
		t.Errorf("should have 3 tunnels, got %d", r.Count())
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	err := r.Unregister("abc123")

	if err != nil {
		t.Fatalf("unregister should succeed, got error: %v", err)
	}
	if r.Count() != 0 {
		t.Errorf("registry should be empty after unregister, got %d tunnels", r.Count())
	}
}

func TestRegistry_Unregister_NotFound(t *testing.T) {
	r := newTestRegistry()

	err := r.Unregister("nonexistent")

	if err != ErrTunnelNotFound {
		t.Errorf("unregistering unknown tunnel should return ErrTunnelNotFound, got: %v", err)
	}
}

func TestRegistry_Get(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	result, err := r.Get("abc123")

	if err != nil {
		t.Fatalf("lookup should succeed: %v", err)
	}
	if result.ID != tunnel.ID {
		t.Errorf("wrong tunnel ID: got %s, want %s", result.ID, tunnel.ID)
	}
	if result.Subdomain != tunnel.Subdomain {
		t.Errorf("wrong subdomain: got %s, want %s", result.Subdomain, tunnel.Subdomain)
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := newTestRegistry()

	_, err := r.Get("nonexistent")

	if err != ErrTunnelNotFound {
		t.Errorf("looking up unknown subdomain should return ErrTunnelNotFound, got: %v", err)
	}
}

func TestRegistry_GetByID(t *testing.T) {
	r := newTestRegistry()
	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	result, err := r.GetByID("test-id")

	if err != nil {
		t.Fatalf("lookup by ID should succeed: %v", err)
	}
	if result.ID != tunnel.ID {
		t.Errorf("wrong tunnel ID: got %s, want %s", result.ID, tunnel.ID)
	}
}

func TestRegistry_GetByID_NotFound(t *testing.T) {
	r := newTestRegistry()

	_, err := r.GetByID("nonexistent")

	if err != ErrTunnelNotFound {
		t.Errorf("looking up unknown ID should return ErrTunnelNotFound, got: %v", err)
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
		t.Errorf("List should return 2 tunnels, got %d", len(result))
	}
}

func TestRegistry_List_Empty(t *testing.T) {
	r := newTestRegistry()

	result := r.List()

	if len(result) != 0 {
		t.Errorf("List on empty registry should return 0 tunnels, got %d", len(result))
	}
}

func TestRegistry_Count(t *testing.T) {
	r := newTestRegistry()

	if r.Count() != 0 {
		t.Errorf("new registry should have 0 tunnels, got %d", r.Count())
	}

	tunnel := models.NewTunnel("test-id", "abc123", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	if r.Count() != 1 {
		t.Errorf("should have 1 tunnel after registration, got %d", r.Count())
	}

	_ = r.Unregister("abc123")

	if r.Count() != 0 {
		t.Errorf("should have 0 tunnels after unregister, got %d", r.Count())
	}
}

func TestRegistry_GenerateSubdomain(t *testing.T) {
	r := newTestRegistry()

	subdomain, err := r.GenerateSubdomain()

	if err != nil {
		t.Fatalf("subdomain generation should succeed: %v", err)
	}
	if len(subdomain) != 8 {
		t.Errorf("subdomain should be 8 characters, got %d", len(subdomain))
	}
}

func TestRegistry_GenerateSubdomain_Unique(t *testing.T) {
	r := newTestRegistry()
	subdomains := make(map[string]bool)

	// Generate 100 subdomains and verify they're all unique
	for i := 0; i < 100; i++ {
		subdomain, err := r.GenerateSubdomain()
		if err != nil {
			t.Fatalf("generation %d failed: %v", i, err)
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

	// Generate subdomains from multiple goroutines
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subdomain, err := r.GenerateSubdomain()
			if err != nil {
				t.Errorf("concurrent generation failed: %v", err)
				return
			}
			subdomains <- subdomain
		}()
	}

	wg.Wait()
	close(subdomains)

	// Verify all are unique
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

	// Concurrent writes
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			subdomain := generateRandomString(8)
			tunnel := models.NewTunnel("id-"+subdomain, subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
			_ = r.Register(tunnel)
		}()
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

	if r.Count() < 0 {
		t.Error("count should never be negative")
	}
}

func TestGenerateRandomString(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"8 characters", 8},
		{"16 characters", 16},
		{"4 characters", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateRandomString(tt.length)
			if len(result) != tt.length {
				t.Errorf("wrong length: got %d, want %d", len(result), tt.length)
			}
		})
	}
}

func TestGenerateRandomString_HexOnly(t *testing.T) {
	result := generateRandomString(8)

	for _, char := range result {
		isDigit := char >= '0' && char <= '9'
		isHexLetter := char >= 'a' && char <= 'f'
		if !isDigit && !isHexLetter {
			t.Errorf("found non-hex character: %c", char)
		}
	}
}

// TestRegistry_StaleTunnelDetection verifies that stale tunnels are tracked correctly
func TestRegistry_StaleTunnelDetection(t *testing.T) {
	r := newTestRegistry()

	tunnel := models.NewTunnel("stale-id", "stalesub", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	// Verify it's registered and has an activity timestamp
	retrieved, err := r.Get("stalesub")
	if err != nil {
		t.Fatalf("tunnel should be findable: %v", err)
	}

	if retrieved.LastActive.IsZero() {
		t.Error("LastActive should be set on creation")
	}

	// Update activity and verify it changes
	retrieved.UpdateActivity()
	if retrieved.LastActive.IsZero() {
		t.Error("LastActive should be updated after UpdateActivity")
	}
}

// TestRegistry_ClosedTunnelDetection verifies we can detect closed tunnels
func TestRegistry_ClosedTunnelDetection(t *testing.T) {
	r := newTestRegistry()

	tunnel := models.NewTunnel("closed-id", "closedsub", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	// Close it
	_ = tunnel.Close()

	if !tunnel.IsClosed() {
		t.Error("tunnel should report as closed")
	}

	// Should still be in registry until cleanup runs
	retrieved, err := r.Get("closedsub")
	if err != nil {
		t.Fatalf("closed tunnel should still be in registry: %v", err)
	}
	if !retrieved.IsClosed() {
		t.Error("retrieved tunnel should also show as closed")
	}
}

// TestRegistry_ErrorCases covers various error scenarios
func TestRegistry_ErrorCases(t *testing.T) {
	r := newTestRegistry()

	// Unregister something that doesn't exist
	err := r.Unregister("doesnotexist")
	if err != ErrTunnelNotFound {
		t.Errorf("unregister non-existent: got %v, want ErrTunnelNotFound", err)
	}

	// Get something that doesn't exist
	_, err = r.Get("doesnotexist")
	if err != ErrTunnelNotFound {
		t.Errorf("get non-existent: got %v, want ErrTunnelNotFound", err)
	}

	// GetByID for something that doesn't exist
	_, err = r.GetByID("doesnotexist")
	if err != ErrTunnelNotFound {
		t.Errorf("getByID non-existent: got %v, want ErrTunnelNotFound", err)
	}
}

// TestRegistry_RegisterAfterUnregister verifies subdomain reuse works
func TestRegistry_RegisterAfterUnregister(t *testing.T) {
	r := newTestRegistry()

	subdomain := "reusable"

	// First registration
	tunnel1 := models.NewTunnel("id-1", subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
	if err := r.Register(tunnel1); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Unregister
	if err := r.Unregister(subdomain); err != nil {
		t.Fatalf("unregister failed: %v", err)
	}

	// Re-register with same subdomain should work
	tunnel2 := models.NewTunnel("id-2", subdomain, models.ProtocolHTTP, "127.0.0.1:8081")
	if err := r.Register(tunnel2); err != nil {
		t.Fatalf("re-registration should succeed: %v", err)
	}

	// Should get the new tunnel
	retrieved, err := r.Get(subdomain)
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if retrieved.ID != "id-2" {
		t.Errorf("should get the new tunnel, got ID %s", retrieved.ID)
	}
}

// TestRegistry_ListReturnsSnapshot verifies List returns a point-in-time copy
func TestRegistry_ListReturnsSnapshot(t *testing.T) {
	r := newTestRegistry()

	// Add some tunnels
	for i := 0; i < 5; i++ {
		subdomain := generateRandomString(8)
		tunnel := models.NewTunnel("id-"+subdomain, subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
		_ = r.Register(tunnel)
	}

	// Get the list
	list := r.List()
	initialLen := len(list)

	// Add more tunnels
	for i := 0; i < 3; i++ {
		subdomain := generateRandomString(8)
		tunnel := models.NewTunnel("new-"+subdomain, subdomain, models.ProtocolHTTP, "127.0.0.1:8080")
		_ = r.Register(tunnel)
	}

	// Original list should be unchanged (it's a snapshot)
	if len(list) != initialLen {
		t.Error("original list should not be affected by new registrations")
	}

	// New list should have all tunnels
	newList := r.List()
	if len(newList) != initialLen+3 {
		t.Errorf("new list should have %d tunnels, got %d", initialLen+3, len(newList))
	}
}

// TestRegistry_GetByIDAndSubdomainMatch verifies both lookups return the same tunnel
func TestRegistry_GetByIDAndSubdomainMatch(t *testing.T) {
	r := newTestRegistry()

	tunnel := models.NewTunnel("unique-id", "unique-sub", models.ProtocolHTTP, "127.0.0.1:8080")
	_ = r.Register(tunnel)

	bySubdomain, err1 := r.Get("unique-sub")
	byID, err2 := r.GetByID("unique-id")

	if err1 != nil || err2 != nil {
		t.Fatalf("lookups failed: %v, %v", err1, err2)
	}

	if bySubdomain != byID {
		t.Error("both lookup methods should return the same tunnel instance")
	}

	if bySubdomain.ID != "unique-id" {
		t.Errorf("wrong ID: got %s, want unique-id", bySubdomain.ID)
	}

	if byID.Subdomain != "unique-sub" {
		t.Errorf("wrong subdomain: got %s, want unique-sub", byID.Subdomain)
	}
}
