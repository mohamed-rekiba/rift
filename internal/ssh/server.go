package ssh

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/mohamed-rekiba/rift/internal/registry"
	"github.com/mohamed-rekiba/rift/internal/tui"
	"github.com/mohamed-rekiba/rift/pkg/models"
)

var ErrServerClosed = ssh.ErrServerClosed

type forwardingConfig struct {
	requestedAddr string
	allocatedPort uint32
	listener      net.Listener
	sshConn       *gossh.ServerConn
	tunnel        *models.Tunnel
}

type Server struct {
	addr       string
	registry   *registry.Registry
	logger     *slog.Logger
	sshServer  *ssh.Server
	baseDomain string
	httpAddr   string

	sessions   map[string]ssh.Session
	sshConns   map[string]*gossh.ServerConn
	sessionsMu sync.RWMutex
}

type Config struct {
	Addr        string
	BaseDomain  string
	Registry    *registry.Registry
	Logger      *slog.Logger
	IdleTimeout time.Duration
	MaxTimeout  time.Duration
	HTTPAddr    string
}

func NewServer(cfg Config) (*Server, error) {
	s := &Server{
		addr:       cfg.Addr,
		registry:   cfg.Registry,
		logger:     cfg.Logger,
		baseDomain: cfg.BaseDomain,
		httpAddr:   cfg.HTTPAddr,
		sessions:   make(map[string]ssh.Session),
		sshConns:   make(map[string]*gossh.ServerConn),
	}

	hostKey, err := generateHostKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate host key: %w", err)
	}

	s.sshServer = &ssh.Server{
		Addr:    cfg.Addr,
		Handler: s.sessionHandler,
		ReversePortForwardingCallback: ssh.ReversePortForwardingCallback(
			s.reversePortForwardingCallback,
		),
		RequestHandlers: map[string]ssh.RequestHandler{
			"tcpip-forward":        s.handleTCPIPForward,
			"cancel-tcpip-forward": s.handleCancelTCPIPForward,
		},
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"session":      s.customSessionHandler,
			"direct-tcpip": ssh.DirectTCPIPHandler,
		},
		LocalPortForwardingCallback: ssh.LocalPortForwardingCallback(func(ctx ssh.Context, dhost string, dport uint32) bool {
			return true
		}),
		ConnCallback: func(ctx ssh.Context, conn net.Conn) net.Conn {
			return conn
		},
		IdleTimeout: cfg.IdleTimeout,
		MaxTimeout:  cfg.MaxTimeout,
	}

	s.sshServer.AddHostKey(hostKey)

	return s, nil
}

func (s *Server) Start() error {
	s.logger.Info("SSH server is listening for tunnel connections", "addr", s.addr)
	return s.sshServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("SSH server is shutting down")
	s.CloseAllSessions()
	return s.sshServer.Shutdown(ctx)
}

func (s *Server) customSessionHandler(srv *ssh.Server, conn *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	sessionID := ctx.SessionID()

	s.sessionsMu.Lock()
	s.sshConns[sessionID] = conn
	s.sessionsMu.Unlock()

	s.logger.Debug("new SSH connection established",
		"session_id", sessionID,
		"user", ctx.User(),
	)

	// Link this SSH connection to an existing tunnel if one was already created
	tunnelID := fmt.Sprintf("%s:%s", ctx.User(), sessionID)
	if tunnel, err := s.registry.GetByID(tunnelID); err == nil {
		tunnel.SetSSHConn(conn)
		s.logger.Info("connected SSH session to tunnel",
			"tunnel_id", tunnelID,
			"subdomain", tunnel.Subdomain,
		)
	}

	ssh.DefaultSessionHandler(srv, conn, newChan, ctx)

	s.sessionsMu.Lock()
	delete(s.sshConns, sessionID)
	s.sessionsMu.Unlock()
}

func (s *Server) registerSession(sess ssh.Session) {
	sessionID := sess.Context().SessionID()

	s.sessionsMu.Lock()
	s.sessions[sessionID] = sess
	s.sessionsMu.Unlock()

	s.logger.Info("new session started",
		"session_id", sessionID,
		"user", sess.User(),
		"active_sessions", len(s.sessions),
	)
}

func (s *Server) unregisterSession(sess ssh.Session) {
	sessionID := sess.Context().SessionID()

	s.sessionsMu.Lock()
	delete(s.sessions, sessionID)
	remaining := len(s.sessions)
	s.sessionsMu.Unlock()

	s.logger.Info("session ended",
		"session_id", sessionID,
		"user", sess.User(),
		"active_sessions", remaining,
	)
}

func (s *Server) GetSSHConnection(sessionID string) (gossh.Conn, bool) {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	conn, ok := s.sshConns[sessionID]
	if !ok {
		return nil, false
	}

	return conn, true
}

func resolveBindAddress(requestedAddr string) (requested, actual string) {
	if requestedAddr == "" {
		requestedAddr = "localhost"
	}

	actualBindAddr := requestedAddr
	if requestedAddr == "localhost" || requestedAddr == "" {
		actualBindAddr = "127.0.0.1"
	}

	return requestedAddr, actualBindAddr
}

// setupForwarding creates a new tunnel with a random subdomain and prepares the port forwarding.
func (s *Server) setupForwarding(ctx ssh.Context, requestedAddr string) (*forwardingConfig, error) {
	subdomain, err := s.registry.GenerateSubdomain()
	if err != nil {
		return nil, fmt.Errorf("couldn't generate a unique subdomain: %w", err)
	}

	requestedAddr, bindAddr := resolveBindAddress(requestedAddr)

	// Grab a random available port to listen on locally
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:0", bindAddr))
	if err != nil {
		return nil, fmt.Errorf("couldn't find an available port: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return nil, fmt.Errorf("couldn't get TCP address from listener")
	}
	allocatedPort := tcpAddr.Port
	localAddr := fmt.Sprintf("%s:%d", tcpAddr.IP.String(), allocatedPort)

	tunnelID := fmt.Sprintf("%s:%s", ctx.User(), ctx.SessionID())
	tunnel := models.NewTunnel(tunnelID, subdomain, models.ProtocolHTTP, localAddr)
	tunnel.SetListener(listener)

	if err := s.registry.Register(tunnel); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("couldn't register the tunnel: %w", err)
	}

	ctx.SetValue("tunnel", tunnel)

	publicURL := fmt.Sprintf("http://%s.%s", subdomain, s.baseDomain)
	s.logger.Info("new tunnel is live!",
		"tunnel_id", tunnelID,
		"subdomain", subdomain,
		"local_addr", localAddr,
		"public_url", publicURL,
	)

	return &forwardingConfig{
		requestedAddr: requestedAddr,
		allocatedPort: uint32(allocatedPort),
		listener:      listener,
		tunnel:        tunnel,
	}, nil
}

func (s *Server) startForwardingWhenReady(ctx ssh.Context, config *forwardingConfig) {
	sessionID := ctx.SessionID()
	subdomain := config.tunnel.Subdomain

	sshConn := s.waitForSSHConnection(sessionID, subdomain)
	if sshConn == nil {
		_ = config.listener.Close()
		_ = s.registry.Unregister(subdomain)
		return
	}

	config.sshConn = sshConn
	config.tunnel.SetSSHConn(sshConn)

	s.handleReversePortForwarding(config)
}

// waitForSSHConnection waits for the SSH connection to be fully established before we can
// start forwarding traffic through it. SSH sessions are set up asynchronously, so we need
// to poll until it's ready.
func (s *Server) waitForSSHConnection(sessionID, subdomain string) *gossh.ServerConn {
	const maxAttempts = 50
	const waitInterval = 100 * time.Millisecond

	for attempt := range maxAttempts {
		s.sessionsMu.RLock()
		sshConn := s.sshConns[sessionID]
		s.sessionsMu.RUnlock()

		if sshConn != nil {
			s.logger.Debug("SSH connection is ready",
				"subdomain", subdomain,
				"waited_ms", attempt*100,
			)
			return sshConn
		}

		time.Sleep(waitInterval)
	}

	s.logger.Error("timed out waiting for SSH connection",
		"subdomain", subdomain,
		"session_id", sessionID,
		"timeout_ms", maxAttempts*100,
	)
	return nil
}

// CloseAllSessions gracefully closes all active tunnel sessions during shutdown.
func (s *Server) CloseAllSessions() {
	s.sessionsMu.Lock()
	sessions := make([]ssh.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	s.sessionsMu.Unlock()

	if len(sessions) == 0 {
		s.logger.Info("no active tunnels to close")
		return
	}

	s.logger.Info("closing all active tunnels", "count", len(sessions))

	for _, sess := range sessions {
		s.logger.Info("closing tunnel",
			"session_id", sess.Context().SessionID(),
			"user", sess.User(),
		)

		_, _ = io.WriteString(sess, "\n👋 Server is shutting down. Thanks for using Rift!\n\n")
		_ = sess.Close()
	}

	s.logger.Info("all tunnels closed")
}

// monitorClientDisconnection watches for Ctrl+C or connection loss so we can clean up properly.
func (s *Server) monitorClientDisconnection(sess ssh.Session) <-chan struct{} {
	doneChan := make(chan struct{})

	go func() {
		defer close(doneChan)

		s.logger.Debug("watching for client disconnect",
			"user", sess.User(),
			"session_id", sess.Context().SessionID(),
		)

		buf := make([]byte, 1)
		readDone := make(chan struct{})
		ctrlCPressed := false

		// Watch for Ctrl+C (byte 0x03) or connection close
		go func() {
			for {
				n, err := sess.Read(buf)

				if err != nil {
					close(readDone)
					return
				}

				if n > 0 && buf[0] == 0x03 {
					s.logger.Info("user pressed Ctrl+C",
						"user", sess.User(),
					)
					ctrlCPressed = true
					close(readDone)
					return
				}
			}
		}()

		select {
		case <-readDone:
			if ctrlCPressed {
				_, _ = io.WriteString(sess, "\n\n👋 Goodbye! Your tunnel is now closed.\n")
				time.Sleep(100 * time.Millisecond)
			}
			s.logger.Info("client disconnected",
				"user", sess.User(),
				"reason", "user initiated",
			)

		case <-sess.Context().Done():
			s.logger.Info("client disconnected",
				"user", sess.User(),
				"reason", "connection lost",
			)
		}
	}()

	return doneChan
}

// sessionHandler manages the lifecycle of an SSH session from connection to cleanup.
func (s *Server) sessionHandler(sess ssh.Session) {
	cmd := sess.Command()
	sessionID := sess.Context().SessionID()

	s.logger.Info("user connected",
		"user", sess.User(),
		"from", sess.RemoteAddr(),
		"command", strings.Join(cmd, " "),
		"session_id", sessionID,
	)

	s.registerSession(sess)

	cleanup := func() {
		s.logger.Debug("cleaning up session",
			"user", sess.User(),
			"session_id", sessionID,
		)

		s.unregisterSession(sess)

		// Close the tunnel and clean up resources
		if tunnel, ok := sess.Context().Value("tunnel").(*models.Tunnel); ok {
			if tuiSession := tunnel.GetTUISession(); tuiSession != nil {
				if session, ok := tuiSession.(*tui.Session); ok {
					session.Stop()
				}
			}

			_ = tunnel.Close()
			_ = s.registry.Unregister(tunnel.Subdomain)
			s.logger.Info("tunnel removed",
				"subdomain", tunnel.Subdomain,
			)
		}

		_ = sess.Exit(0)

		s.logger.Info("user disconnected",
			"user", sess.User(),
			"from", sess.RemoteAddr(),
		)
	}
	defer cleanup()

	_, _ = io.WriteString(sess, "Setting up your tunnel...\n\n")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sess.Context().Done():
			return

		case <-ticker.C:
			tunnel, ok := sess.Context().Value("tunnel").(*models.Tunnel)
			if !ok {
				continue
			}

			publicURL := fmt.Sprintf("http://%s.%s", tunnel.Subdomain, s.baseDomain)
			if s.httpAddr != "" && s.httpAddr != ":80" && s.httpAddr != ":443" {
				publicURL = fmt.Sprintf("http://%s.%s%s", tunnel.Subdomain, s.baseDomain, s.httpAddr)
			}

			// Check if the user's terminal supports the interactive dashboard
			_, _, hasPty := sess.Pty()
			if hasPty {
				tuiSession := tui.NewSession(publicURL, tunnel.Subdomain, s.httpAddr, s.logger)
				tunnel.SetTUISession(tuiSession)

				s.logger.Info("showing interactive dashboard",
					"user", sess.User(),
					"subdomain", tunnel.Subdomain,
				)

				if err := tuiSession.Start(sess); err != nil {
					s.logger.Warn("couldn't start dashboard, using simple mode", "error", err)
					s.showSimpleMode(sess, tunnel, publicURL)
					return
				}

				return
			}

			s.logger.Info("terminal doesn't support dashboard, using simple mode",
				"user", sess.User(),
				"subdomain", tunnel.Subdomain,
			)
			s.showSimpleMode(sess, tunnel, publicURL)
			return
		}
	}
}

// showSimpleMode displays a minimal text interface for terminals that don't support the full TUI.
func (s *Server) showSimpleMode(sess ssh.Session, tunnel *models.Tunnel, publicURL string) {
	message := "\n🚀 Your tunnel is ready!\n\n"
	message += fmt.Sprintf("   Public URL:  %s\n", publicURL)
	message += fmt.Sprintf("   Subdomain:   %s\n", tunnel.Subdomain)
	message += "\nKeep this window open to maintain your tunnel.\n"
	message += "Press Ctrl+C to close it when you're done.\n\n"
	message += "💡 Tip: Run with 'ssh -t' for a live dashboard with request logs!\n\n"

	_, _ = io.WriteString(sess, message)

	doneChan := s.monitorClientDisconnection(sess)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-doneChan:
			return
		case <-sess.Context().Done():
			return
		case <-ticker.C:
			// Show periodic stats updates in simple mode
			bytesReceived, bytesSent, totalRequests := tunnel.GetStats()
			if totalRequests > 0 {
				statusMsg := fmt.Sprintf("\r📊 %d requests | ↓ %s received | ↑ %s sent",
					totalRequests,
					formatBytes(bytesReceived),
					formatBytes(bytesSent),
				)
				_, _ = io.WriteString(sess, statusMsg)
			}
		}
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (s *Server) reversePortForwardingCallback(ctx ssh.Context, bindHost string, bindPort uint32) bool {
	s.logger.Info("tunnel request received",
		"user", ctx.User(),
		"bind_host", bindHost,
		"bind_port", bindPort,
		"from", ctx.RemoteAddr(),
	)

	return true
}

// handleTCPIPForward processes SSH -R (reverse port forwarding) requests.
// This is the core of how tunnels work - SSH sends us this request when the user runs:
// ssh -R 8080:localhost:3000 ...
func (s *Server) handleTCPIPForward(ctx ssh.Context, _ *ssh.Server, req *gossh.Request) (ok bool, payload []byte) {
	var reqPayload struct {
		Addr string
		Port uint32
	}

	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		s.logger.Error("couldn't parse tunnel request", "error", err)
		return false, nil
	}

	s.logger.Info("setting up tunnel",
		"user", ctx.User(),
		"requested_addr", reqPayload.Addr,
		"requested_port", reqPayload.Port,
	)

	config, err := s.setupForwarding(ctx, reqPayload.Addr)
	if err != nil {
		s.logger.Error("couldn't create tunnel", "error", err)
		return false, nil
	}

	go s.startForwardingWhenReady(ctx, config)

	s.logger.Info("tunnel is now accessible",
		"url", fmt.Sprintf("http://%s.%s", config.tunnel.Subdomain, s.baseDomain),
		"subdomain", config.tunnel.Subdomain,
		"port", config.allocatedPort,
	)

	response := struct {
		Port uint32
	}{
		Port: config.allocatedPort,
	}

	return true, gossh.Marshal(response)
}

// handleCancelTCPIPForward handles requests to close a tunnel (when SSH connection ends).
func (s *Server) handleCancelTCPIPForward(ctx ssh.Context, _ *ssh.Server, req *gossh.Request) (ok bool, payload []byte) {
	var reqPayload struct {
		Addr string
		Port uint32
	}

	if err := gossh.Unmarshal(req.Payload, &reqPayload); err != nil {
		s.logger.Error("couldn't parse cancel request", "error", err)
		return false, nil
	}

	s.logger.Info("closing tunnel",
		"user", ctx.User(),
		"addr", reqPayload.Addr,
		"port", reqPayload.Port,
	)

	if tunnel, ok := ctx.Value("tunnel").(*models.Tunnel); ok {
		_ = tunnel.Close()
		_ = s.registry.Unregister(tunnel.Subdomain)
		s.logger.Info("tunnel closed successfully", "subdomain", tunnel.Subdomain)
	}

	return true, nil
}

// handleReversePortForwarding runs the main loop that accepts incoming HTTP connections
// and forwards them through the SSH tunnel to the user's local server.
func (s *Server) handleReversePortForwarding(config *forwardingConfig) {
	defer func() { _ = config.listener.Close() }()

	s.logger.Info("tunnel is accepting connections",
		"subdomain", config.tunnel.Subdomain,
		"listening_on", config.listener.Addr().String(),
	)

	for {
		conn, err := config.listener.Accept()
		if err != nil {
			if config.tunnel.IsClosed() {
				s.logger.Debug("tunnel listener stopped", "subdomain", config.tunnel.Subdomain)
				return
			}
			s.logger.Error("couldn't accept connection",
				"error", err,
				"subdomain", config.tunnel.Subdomain,
			)
			continue
		}

		s.logger.Debug("new incoming connection",
			"subdomain", config.tunnel.Subdomain,
			"from", conn.RemoteAddr(),
		)

		go s.forwardConnection(conn, config)
	}
}

// forwardConnection pipes data between an incoming HTTP connection and the SSH tunnel.
// This is where the magic happens - bytes flow from the internet, through SSH, to localhost.
func (s *Server) forwardConnection(conn net.Conn, config *forwardingConfig) {
	defer func() { _ = conn.Close() }()

	originAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		s.logger.Error("couldn't get TCP address from connection", "subdomain", config.tunnel.Subdomain)
		return
	}

	// Tell the SSH client where this connection came from so it knows where to forward it
	payload := struct {
		ConnectedAddr string
		ConnectedPort uint32
		OriginAddr    string
		OriginPort    uint32
	}{
		ConnectedAddr: config.requestedAddr,
		ConnectedPort: config.allocatedPort,
		OriginAddr:    originAddr.IP.String(),
		OriginPort:    uint32(originAddr.Port),
	}

	s.logger.Debug("opening SSH channel for forwarding",
		"to", fmt.Sprintf("%s:%d", payload.ConnectedAddr, payload.ConnectedPort),
		"from", fmt.Sprintf("%s:%d", payload.OriginAddr, payload.OriginPort),
		"subdomain", config.tunnel.Subdomain,
	)

	channel, requests, err := config.sshConn.OpenChannel("forwarded-tcpip", gossh.Marshal(payload))
	if err != nil {
		s.logger.Error("couldn't open SSH forwarding channel",
			"error", err,
			"subdomain", config.tunnel.Subdomain,
		)
		return
	}
	defer func() { _ = channel.Close() }()

	// We don't care about any requests on this channel
	go gossh.DiscardRequests(requests)

	s.logger.Debug("SSH channel opened, forwarding data",
		"subdomain", config.tunnel.Subdomain,
	)

	// Pipe data in both directions until one side closes
	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(channel, conn)
		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(conn, channel)
		done <- struct{}{}
	}()

	<-done

	s.logger.Debug("connection forwarding finished",
		"subdomain", config.tunnel.Subdomain,
	)
}

func generateHostKey() (ssh.Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	signer, err := gossh.ParsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}

	return signer, nil
}
