package registry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mohamed-rekiba/rift/pkg/models"
)

var (
	ErrTunnelNotFound   = errors.New("no tunnel exists with that subdomain")
	ErrSubdomainTaken   = errors.New("that subdomain is already in use")
	ErrInvalidSubdomain = errors.New("the subdomain format is invalid")
)

type Registry struct {
	mu              sync.RWMutex
	tunnels         map[string]*models.Tunnel
	tunnelsById     map[string]*models.Tunnel
	logger          *slog.Logger
	cleanupInterval time.Duration
}

func NewRegistry(logger *slog.Logger, cleanupInterval time.Duration) *Registry {
	r := &Registry{
		tunnels:         make(map[string]*models.Tunnel),
		tunnelsById:     make(map[string]*models.Tunnel),
		logger:          logger,
		cleanupInterval: cleanupInterval,
	}

	go r.cleanupStale()

	return r
}

// Register adds a new tunnel to the registry so it can receive traffic.
func (r *Registry) Register(tunnel *models.Tunnel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tunnels[tunnel.Subdomain]; exists {
		return ErrSubdomainTaken
	}

	r.tunnels[tunnel.Subdomain] = tunnel
	r.tunnelsById[tunnel.ID] = tunnel

	r.logger.Info("tunnel added to registry",
		"id", tunnel.ID,
		"subdomain", tunnel.Subdomain,
		"protocol", tunnel.Protocol,
		"local_addr", tunnel.LocalAddr,
	)

	return nil
}

// Unregister removes a tunnel from the registry (usually when the user disconnects).
func (r *Registry) Unregister(subdomain string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tunnel, exists := r.tunnels[subdomain]
	if !exists {
		return ErrTunnelNotFound
	}

	delete(r.tunnels, subdomain)
	delete(r.tunnelsById, tunnel.ID)

	r.logger.Info("tunnel removed from registry",
		"id", tunnel.ID,
		"subdomain", subdomain,
	)

	return nil
}

func (r *Registry) Get(subdomain string) (*models.Tunnel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tunnel, exists := r.tunnels[subdomain]
	if !exists {
		return nil, ErrTunnelNotFound
	}

	return tunnel, nil
}

func (r *Registry) GetByID(id string) (*models.Tunnel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tunnel, exists := r.tunnelsById[id]
	if !exists {
		return nil, ErrTunnelNotFound
	}

	return tunnel, nil
}

func (r *Registry) List() []*models.Tunnel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tunnels := make([]*models.Tunnel, 0, len(r.tunnels))
	for _, tunnel := range r.tunnels {
		tunnels = append(tunnels, tunnel)
	}

	return tunnels
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tunnels)
}

// GenerateSubdomain creates a unique random subdomain for a new tunnel.
// It tries up to 10 times to avoid collisions (though collisions are extremely rare).
func (r *Registry) GenerateSubdomain() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for range 10 {
		subdomain := generateRandomString(8)
		if _, exists := r.tunnels[subdomain]; !exists {
			return subdomain, nil
		}
	}

	return "", fmt.Errorf("couldn't find a unique subdomain after several attempts")
}

// cleanupStale runs periodically to remove tunnels that have been inactive for too long.
// This prevents ghost tunnels from hanging around when connections are dropped unexpectedly.
func (r *Registry) cleanupStale() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		now := time.Now()
		staleTimeout := 10 * time.Minute

		for subdomain, tunnel := range r.tunnels {
			if tunnel.IsClosed() || now.Sub(tunnel.LastActive) > staleTimeout {
				r.logger.Info("removing inactive tunnel",
					"subdomain", subdomain,
					"last_seen", tunnel.LastActive,
				)

				_ = tunnel.Close()
				delete(r.tunnels, subdomain)
				delete(r.tunnelsById, tunnel.ID)
			}
		}
		r.mu.Unlock()
	}
}

func generateRandomString(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}
