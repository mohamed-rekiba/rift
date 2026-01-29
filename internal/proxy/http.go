package proxy

import (
	"bufio"
	"context"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mohamed-rekiba/rift/internal/registry"
	"github.com/mohamed-rekiba/rift/internal/tui"
	"github.com/mohamed-rekiba/rift/pkg/models"
	"github.com/mohamed-rekiba/rift/web"
)

// HTTPProxy routes incoming HTTP requests to the right tunnel based on subdomain.
// It extracts the subdomain from the Host header and forwards the request through
// the corresponding SSH tunnel to the user's local server.
type HTTPProxy struct {
	registry   *registry.Registry
	logger     *slog.Logger
	baseDomain string
	server     *http.Server
}

// Config holds settings for the HTTP proxy.
type Config struct {
	Addr       string
	BaseDomain string
	Registry   *registry.Registry
	Logger     *slog.Logger
}

// NewHTTPProxy creates a new HTTP proxy ready to accept connections.
func NewHTTPProxy(cfg Config) *HTTPProxy {
	p := &HTTPProxy{
		registry:   cfg.Registry,
		logger:     cfg.Logger,
		baseDomain: cfg.BaseDomain,
	}

	p.server = &http.Server{
		Addr:         cfg.Addr,
		Handler:      p,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return p
}

// Start begins accepting HTTP connections.
func (p *HTTPProxy) Start() error {
	p.logger.Info("HTTP proxy is starting", "addr", p.server.Addr)
	return p.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP proxy, waiting for active requests to complete.
func (p *HTTPProxy) Shutdown(ctx context.Context) error {
	p.logger.Info("HTTP proxy is shutting down")
	return p.server.Shutdown(ctx)
}

// ServeHTTP handles each incoming HTTP request by routing it to the right tunnel.
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Figure out which tunnel this request belongs to
	subdomain := p.extractSubdomain(r.Host)

	p.logger.Info("new request",
		"method", r.Method,
		"host", r.Host,
		"subdomain", subdomain,
		"path", r.URL.Path,
		"from", r.RemoteAddr,
	)

	// No subdomain means they're visiting the main domain - show them the status page
	if subdomain == "" || subdomain == p.baseDomain {
		p.handleStatusPage(w, r)
		return
	}

	// Look up the tunnel for this subdomain
	tunnel, err := p.registry.Get(subdomain)
	if err != nil {
		p.logger.Warn("no tunnel found for subdomain",
			"subdomain", subdomain,
			"error", err,
		)
		http.Error(w, "Oops! We couldn't find a tunnel for '"+subdomain+"'. Make sure your tunnel is still running.", http.StatusNotFound)
		return
	}

	// Keep track of when the tunnel was last used (for cleanup)
	tunnel.UpdateActivity()

	// Send the request through the SSH tunnel to the user's local server
	if err := p.forwardRequest(w, r, tunnel); err != nil {
		p.logger.Error("couldn't forward request to local server",
			"subdomain", subdomain,
			"error", err,
		)

		// Send failed request to TUI so user can see it
		p.sendFailedRequestToTUI(tunnel, r, http.StatusBadGateway)

		http.Error(w, "We couldn't reach your local server. Is it running?", http.StatusBadGateway)
	} else {
		duration := time.Since(startTime)
		p.logger.Info("request completed successfully",
			"subdomain", subdomain,
			"path", r.URL.Path,
			"took_ms", duration.Milliseconds(),
		)
	}
}

// forwardRequest sends the HTTP request through the SSH tunnel to the user's local server
// and streams the response back to the original client.
func (p *HTTPProxy) forwardRequest(w http.ResponseWriter, r *http.Request, tunnel interface{}) error {
	tun, ok := tunnel.(*models.Tunnel)
	if !ok {
		return fmt.Errorf("something went wrong with the tunnel connection")
	}

	// This is the local port that SSH is listening on - it forwards to the user's server
	localAddr := tun.LocalAddr

	p.logger.Debug("connecting to local endpoint",
		"local_addr", localAddr,
		"subdomain", tun.Subdomain,
	)

	// Open a connection to the SSH-forwarded port
	conn, err := net.DialTimeout("tcp", localAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("couldn't connect to the tunnel: %w", err)
	}
	defer func() { _ = conn.Close() }()

	p.logger.Debug("connected to tunnel")

	// Calculate how much data we're sending (for stats)
	requestSize := int64(len(r.Method) + len(r.URL.Path) + len(r.Proto))
	for key, values := range r.Header {
		for _, value := range values {
			requestSize += int64(len(key) + len(value) + 4)
		}
	}
	if r.ContentLength > 0 {
		requestSize += r.ContentLength
	}

	// Send the request through the tunnel
	if writeErr := r.Write(conn); writeErr != nil {
		return fmt.Errorf("couldn't send request through tunnel: %w", writeErr)
	}

	tun.AddBytesSent(requestSize)

	p.logger.Debug("request sent to local server")

	// Wait for the response from the user's local server
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, r)
	if err != nil {
		return fmt.Errorf("your local server didn't respond - is it running?: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	p.logger.Debug("got response from local server",
		"status", resp.StatusCode,
	)

	// Pass through all response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Send the status code
	w.WriteHeader(resp.StatusCode)

	// Stream the response body back to the client
	bytesReceived, err := io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("connection interrupted while sending response: %w", err)
	}

	tun.AddBytesReceived(bytesReceived)
	tun.IncrementRequests()

	// Update the TUI dashboard if the user has one running
	p.sendRequestToTUI(tun, r, resp.StatusCode, bytesReceived)
	p.sendStatsToTUI(tun)

	return nil
}

// extractSubdomain pulls out the subdomain from a Host header like "abc123.example.com"
func (p *HTTPProxy) extractSubdomain(host string) string {
	// Strip the port if there is one (e.g., "abc123.localhost:8080" -> "abc123.localhost")
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Check if it ends with our base domain
	if strings.HasSuffix(host, "."+p.baseDomain) {
		subdomain := strings.TrimSuffix(host, "."+p.baseDomain)
		return subdomain
	}

	// If they're hitting the base domain directly, there's no subdomain
	if host == p.baseDomain {
		return ""
	}

	return host
}

// handleStatusPage shows a simple landing page when users visit the root domain.
func (p *HTTPProxy) handleStatusPage(w http.ResponseWriter, _ *http.Request) {
	tmpl, err := template.New("status").Parse(web.StatusPageHTML)
	if err != nil {
		p.logger.Error("couldn't render status page", "error", err)
		http.Error(w, "Something went wrong on our end. Please try again.", http.StatusInternalServerError)
		return
	}

	data := struct {
		RiftCount  int
		BaseDomain string
	}{
		RiftCount:  p.registry.Count(),
		BaseDomain: p.baseDomain,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err := tmpl.Execute(w, data); err != nil {
		p.logger.Error("couldn't render status page", "error", err)
	}
}

func (p *HTTPProxy) sendRequestToTUI(tunnel *models.Tunnel, r *http.Request, statusCode int, size int64) {
	tuiSession := tunnel.GetTUISession()
	if tuiSession == nil {
		return
	}

	session, ok := tuiSession.(*tui.Session)
	if !ok {
		return
	}

	session.AddRequest(tui.RequestInfo{
		Timestamp:  time.Now(),
		Method:     r.Method,
		Path:       r.URL.Path,
		StatusCode: statusCode,
		Size:       size,
	})
}

// sendFailedRequestToTUI sends a failed request to the TUI when the local server is unreachable.
func (p *HTTPProxy) sendFailedRequestToTUI(tunnel interface{}, r *http.Request, statusCode int) {
	tun, ok := tunnel.(*models.Tunnel)
	if !ok {
		return
	}

	tuiSession := tun.GetTUISession()
	if tuiSession == nil {
		return
	}

	session, ok := tuiSession.(*tui.Session)
	if !ok {
		return
	}

	session.AddRequest(tui.RequestInfo{
		Timestamp:  time.Now(),
		Method:     r.Method,
		Path:       r.URL.Path,
		StatusCode: statusCode,
		Size:       0,
	})
}

func (p *HTTPProxy) sendStatsToTUI(tunnel *models.Tunnel) {
	tuiSession := tunnel.GetTUISession()
	if tuiSession == nil {
		return
	}

	session, ok := tuiSession.(*tui.Session)
	if !ok {
		return
	}

	bytesReceived, bytesSent, totalRequests := tunnel.GetStats()
	activeConns := tunnel.GetActiveConnections()

	session.UpdateStats(tui.Stats{
		BytesReceived: bytesReceived,
		BytesSent:     bytesSent,
		ActiveConns:   activeConns,
		TotalRequests: totalRequests,
	})
}
