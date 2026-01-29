package config

import (
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	SSHAddr         string
	HTTPAddr        string
	BaseDomain      string
	LogLevel        string
	IdleTimeout     time.Duration
	MaxTimeout      time.Duration
	CleanupInterval time.Duration
	SubdomainLength int
}

func Load() *Config {
	_ = godotenv.Load()

	sshAddr := flag.String("ssh-addr", "", "SSH server address")
	httpAddr := flag.String("http-addr", "", "HTTP proxy address")
	baseDomain := flag.String("domain", "", "Base domain for rift")
	logLevel := flag.String("log-level", "", "Log level (debug, info, warn, error)")

	flag.Parse()

	cfg := &Config{
		SSHAddr:         getConfigValue(*sshAddr, "SSH_ADDR", ":2222"),
		HTTPAddr:        getConfigValue(*httpAddr, "HTTP_ADDR", ":8080"),
		BaseDomain:      getConfigValue(*baseDomain, "BASE_DOMAIN", "localhost"),
		LogLevel:        getConfigValue(*logLevel, "LOG_LEVEL", "info"),
		IdleTimeout:     getDurationEnv("IDLE_TIMEOUT", 300*time.Second),
		MaxTimeout:      getDurationEnv("MAX_TIMEOUT", 5*time.Minute),
		CleanupInterval: getDurationEnv("CLEANUP_INTERVAL", 5*time.Minute),
		SubdomainLength: getIntEnv("SUBDOMAIN_LENGTH", 8),
	}

	// Make sure addresses have the colon prefix
	if !strings.HasPrefix(cfg.SSHAddr, ":") {
		cfg.SSHAddr = ":" + cfg.SSHAddr
	}
	if !strings.HasPrefix(cfg.HTTPAddr, ":") {
		cfg.HTTPAddr = ":" + cfg.HTTPAddr
	}

	log.Printf("Config loaded: SSH on %s, HTTP on %s, domain %s",
		cfg.SSHAddr, cfg.HTTPAddr, cfg.BaseDomain)

	return cfg
}

// getConfigValue returns the first non-empty value from: flag, env var, or default.
func getConfigValue(flagValue, envKey, defaultValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := os.Getenv(envKey); envValue != "" {
		return envValue
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
