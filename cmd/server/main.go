package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mohamed-rekiba/rift/internal/config"
	"github.com/mohamed-rekiba/rift/internal/proxy"
	"github.com/mohamed-rekiba/rift/internal/registry"
	"github.com/mohamed-rekiba/rift/internal/ssh"
)

func main() {
	cfg := config.Load()

	logger := setupLogger(cfg.LogLevel)

	logger.Info("Rift is starting up",
		"ssh_addr", cfg.SSHAddr,
		"http_addr", cfg.HTTPAddr,
		"base_domain", cfg.BaseDomain,
	)

	reg := registry.NewRegistry(logger, cfg.CleanupInterval)

	sshServer, err := ssh.NewServer(ssh.Config{
		Addr:        cfg.SSHAddr,
		BaseDomain:  cfg.BaseDomain,
		Registry:    reg,
		Logger:      logger,
		IdleTimeout: cfg.IdleTimeout,
		MaxTimeout:  cfg.MaxTimeout,
		HTTPAddr:    cfg.HTTPAddr,
	})
	if err != nil {
		logger.Error("couldn't create SSH server", "error", err)
		os.Exit(1)
	}

	httpProxy := proxy.NewHTTPProxy(proxy.Config{
		Addr:       cfg.HTTPAddr,
		BaseDomain: cfg.BaseDomain,
		Registry:   reg,
		Logger:     logger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errChan := make(chan error, 2)

	go func() {
		logger.Info("SSH server is now listening for connections", "addr", cfg.SSHAddr)
		if err := sshServer.Start(); err != nil {
			if !errors.Is(err, ssh.ErrServerClosed) {
				errChan <- fmt.Errorf("SSH server crashed: %w", err)
			} else {
				logger.Debug("SSH server closed cleanly")
			}
		}
	}()

	go func() {
		logger.Info("HTTP proxy is ready to forward requests", "addr", cfg.HTTPAddr)
		if err := httpProxy.Start(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				errChan <- fmt.Errorf("HTTP proxy crashed: %w", err)
			} else {
				logger.Debug("HTTP proxy closed cleanly")
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	logger.Info("Rift is ready! Create tunnels by connecting via SSH")

	select {
	case err := <-errChan:
		logger.Error("something went wrong", "error", err)
		stop()
	case <-ctx.Done():
		logger.Info("received shutdown signal, wrapping things up...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	logger.Info("closing all tunnels and connections gracefully")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		logger.Info("stopping SSH server...")
		if err := sshServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("had trouble stopping SSH server", "error", err)
		} else {
			logger.Info("SSH server stopped")
		}
	}()

	go func() {
		defer wg.Done()
		logger.Info("stopping HTTP proxy...")
		if err := httpProxy.Shutdown(shutdownCtx); err != nil {
			logger.Error("had trouble stopping HTTP proxy", "error", err)
		} else {
			logger.Info("HTTP proxy stopped")
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all services stopped cleanly")
	case <-shutdownCtx.Done():
		logger.Warn("shutdown took too long, forcing exit")
	}

	logger.Info("goodbye!")
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	return slog.New(handler)
}
