package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Build info - set at compile time via -ldflags
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Config holds all application configuration
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

// Parse handles CLI flags and returns configuration.
func Parse() *Config {
	// Check for version flag first (before flag.Parse)
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "-version" {
			printVersion()
			return nil
		}
	}

	// Load .env file if present
	_ = godotenv.Load()

	// Define flags
	sshAddr := flag.String("ssh-addr", "", "SSH server address (env: SSH_ADDR)")
	httpAddr := flag.String("http-addr", "", "HTTP proxy address (env: HTTP_ADDR)")
	baseDomain := flag.String("domain", "", "Base domain for tunnels (env: BASE_DOMAIN)")
	logLevel := flag.String("log-level", "", "Log level: debug, info, warn, error (env: LOG_LEVEL)")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: rift-server [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -v, -version      print version and exit\n")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(os.Stderr, "  -%s string\n    \t%s\n", f.Name, f.Usage)
		})
	}

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

	// Normalize addresses to have colon prefix
	if !strings.HasPrefix(cfg.SSHAddr, ":") && !strings.Contains(cfg.SSHAddr, ":") {
		cfg.SSHAddr = ":" + cfg.SSHAddr
	}
	if !strings.HasPrefix(cfg.HTTPAddr, ":") && !strings.Contains(cfg.HTTPAddr, ":") {
		cfg.HTTPAddr = ":" + cfg.HTTPAddr
	}

	return cfg
}

func printVersion() {
	banner := `
       _  __ _   
  _ __(_)/ _| |_ 
 | '__| | |_| __|
 | |  | |  _| |_ 
 |_|  |_|_|  \__|`

	fmt.Println(banner, Version+", built with Go", runtime.Version())
	fmt.Println()
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
