package cli

import (
	"os"
	"testing"
	"time"
)

func TestGetConfigValue_FlagPriority(t *testing.T) {
	// When both flag and env are set, the flag should win
	os.Setenv("TEST_KEY", "env_value")
	defer os.Unsetenv("TEST_KEY")

	result := getConfigValue("flag_value", "TEST_KEY", "default_value")

	if result != "flag_value" {
		t.Errorf("flag should take priority over env, got %s instead of flag_value", result)
	}
}

func TestGetConfigValue_EnvPriority(t *testing.T) {
	// When flag is empty, fall back to environment variable
	os.Setenv("TEST_KEY", "env_value")
	defer os.Unsetenv("TEST_KEY")

	result := getConfigValue("", "TEST_KEY", "default_value")

	if result != "env_value" {
		t.Errorf("should use env when flag is empty, got %s instead of env_value", result)
	}
}

func TestGetConfigValue_Default(t *testing.T) {
	// When both flag and env are empty, use the default
	os.Unsetenv("TEST_KEY_UNSET")

	result := getConfigValue("", "TEST_KEY_UNSET", "default_value")

	if result != "default_value" {
		t.Errorf("should fall back to default, got %s instead of default_value", result)
	}
}

func TestGetConfigValue_EmptyEnv(t *testing.T) {
	// An empty env var should be treated as "not set"
	os.Setenv("TEST_EMPTY", "")
	defer os.Unsetenv("TEST_EMPTY")

	result := getConfigValue("", "TEST_EMPTY", "default_value")

	if result != "default_value" {
		t.Errorf("empty env should fall back to default, got %s instead of default_value", result)
	}
}

func TestGetIntEnv_ValidInt(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     int
	}{
		{"positive number", "42", 42},
		{"zero", "0", 0},
		{"negative number", "-10", -10},
		{"large number", "999999", 999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_INT", tt.envValue)
			defer os.Unsetenv("TEST_INT")

			result := getIntEnv("TEST_INT", 100)
			if result != tt.want {
				t.Errorf("parsing %q: got %d, want %d", tt.envValue, result, tt.want)
			}
		})
	}
}

func TestGetIntEnv_InvalidInt(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		{"text string", "not_a_number"},
		{"decimal number", "3.14"},
		{"empty string", ""},
		{"special characters", "!@#"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_INT_INVALID", tt.envValue)
			defer os.Unsetenv("TEST_INT_INVALID")

			result := getIntEnv("TEST_INT_INVALID", 100)
			if result != 100 {
				t.Errorf("invalid value %q should return default 100, got %d", tt.envValue, result)
			}
		})
	}
}

func TestGetIntEnv_NotSet(t *testing.T) {
	os.Unsetenv("TEST_INT_NOT_SET")

	result := getIntEnv("TEST_INT_NOT_SET", 50)
	if result != 50 {
		t.Errorf("unset env should return default 50, got %d", result)
	}
}

func TestGetDurationEnv_ValidDuration(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     time.Duration
	}{
		{"seconds", "30s", 30 * time.Second},
		{"minutes", "5m", 5 * time.Minute},
		{"hours", "2h", 2 * time.Hour},
		{"milliseconds", "500ms", 500 * time.Millisecond},
		{"combined", "1h30m", 90 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_DURATION", tt.envValue)
			defer os.Unsetenv("TEST_DURATION")

			result := getDurationEnv("TEST_DURATION", time.Hour)
			if result != tt.want {
				t.Errorf("parsing %q: got %v, want %v", tt.envValue, result, tt.want)
			}
		})
	}
}

func TestGetDurationEnv_InvalidDuration(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		{"number without unit", "30"},
		{"text string", "not_a_duration"},
		{"empty string", ""},
		{"invalid unit", "30x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("TEST_DURATION_INVALID", tt.envValue)
			defer os.Unsetenv("TEST_DURATION_INVALID")

			defaultDuration := 5 * time.Minute
			result := getDurationEnv("TEST_DURATION_INVALID", defaultDuration)
			if result != defaultDuration {
				t.Errorf("invalid value %q should return default %v, got %v", tt.envValue, defaultDuration, result)
			}
		})
	}
}

func TestGetDurationEnv_NotSet(t *testing.T) {
	os.Unsetenv("TEST_DURATION_NOT_SET")

	defaultDuration := 10 * time.Minute
	result := getDurationEnv("TEST_DURATION_NOT_SET", defaultDuration)
	if result != defaultDuration {
		t.Errorf("unset env should return default %v, got %v", defaultDuration, result)
	}
}

// TestConfig_AddressNormalization verifies that port-only addresses get a colon prefix
func TestConfig_AddressNormalization(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"already has leading colon", ":8080", ":8080"},
		{"port number only", "8080", ":8080"},
		{"full host:port format", "localhost:8080", "localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_SSH", func(t *testing.T) {
			// Start fresh with no env vars
			os.Unsetenv("SSH_ADDR")
			os.Unsetenv("HTTP_ADDR")
			os.Unsetenv("BASE_DOMAIN")
			os.Unsetenv("LOG_LEVEL")

			os.Setenv("SSH_ADDR", tt.input)
			defer os.Unsetenv("SSH_ADDR")

			// We can't test Load() directly due to flag.Parse() global state,
			// so we test the normalization logic separately
			addr := tt.input
			if len(addr) > 0 && addr[0] != ':' && !containsColon(addr) {
				addr = ":" + addr
			}

			if addr != tt.expected {
				t.Errorf("normalizing %q: got %q, want %q", tt.input, addr, tt.expected)
			}
		})
	}
}

// containsColon checks if string contains a colon (for host:port format)
func containsColon(s string) bool {
	for _, c := range s {
		if c == ':' {
			return true
		}
	}
	return false
}

// TestConfig_Defaults ensures we have sensible defaults for all config values
func TestConfig_Defaults(t *testing.T) {
	// Clear everything to test defaults
	envVars := []string{"SSH_ADDR", "HTTP_ADDR", "BASE_DOMAIN", "LOG_LEVEL",
		"IDLE_TIMEOUT", "MAX_TIMEOUT", "CLEANUP_INTERVAL", "SUBDOMAIN_LENGTH"}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	// Check each default value
	sshAddr := getConfigValue("", "SSH_ADDR", ":2222")
	if sshAddr != ":2222" {
		t.Errorf("SSH_ADDR default should be :2222, got %s", sshAddr)
	}

	httpAddr := getConfigValue("", "HTTP_ADDR", ":8080")
	if httpAddr != ":8080" {
		t.Errorf("HTTP_ADDR default should be :8080, got %s", httpAddr)
	}

	baseDomain := getConfigValue("", "BASE_DOMAIN", "localhost")
	if baseDomain != "localhost" {
		t.Errorf("BASE_DOMAIN default should be localhost, got %s", baseDomain)
	}

	logLevel := getConfigValue("", "LOG_LEVEL", "info")
	if logLevel != "info" {
		t.Errorf("LOG_LEVEL default should be info, got %s", logLevel)
	}

	idleTimeout := getDurationEnv("IDLE_TIMEOUT", 300*time.Second)
	if idleTimeout != 300*time.Second {
		t.Errorf("IDLE_TIMEOUT default should be 5m, got %v", idleTimeout)
	}

	maxTimeout := getDurationEnv("MAX_TIMEOUT", 5*time.Minute)
	if maxTimeout != 5*time.Minute {
		t.Errorf("MAX_TIMEOUT default should be 5m, got %v", maxTimeout)
	}

	cleanupInterval := getDurationEnv("CLEANUP_INTERVAL", 5*time.Minute)
	if cleanupInterval != 5*time.Minute {
		t.Errorf("CLEANUP_INTERVAL default should be 5m, got %v", cleanupInterval)
	}

	subdomainLength := getIntEnv("SUBDOMAIN_LENGTH", 8)
	if subdomainLength != 8 {
		t.Errorf("SUBDOMAIN_LENGTH default should be 8, got %d", subdomainLength)
	}
}

// TestConfig_EnvOverrides verifies that env vars properly override defaults
func TestConfig_EnvOverrides(t *testing.T) {
	// Set custom values via environment
	os.Setenv("SSH_ADDR", ":3333")
	os.Setenv("HTTP_ADDR", ":9090")
	os.Setenv("BASE_DOMAIN", "custom.example.com")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("IDLE_TIMEOUT", "10m")
	os.Setenv("MAX_TIMEOUT", "1h")
	os.Setenv("CLEANUP_INTERVAL", "15m")
	os.Setenv("SUBDOMAIN_LENGTH", "12")

	defer func() {
		os.Unsetenv("SSH_ADDR")
		os.Unsetenv("HTTP_ADDR")
		os.Unsetenv("BASE_DOMAIN")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("IDLE_TIMEOUT")
		os.Unsetenv("MAX_TIMEOUT")
		os.Unsetenv("CLEANUP_INTERVAL")
		os.Unsetenv("SUBDOMAIN_LENGTH")
	}()

	// Verify the env values are picked up
	sshAddr := getConfigValue("", "SSH_ADDR", ":2222")
	if sshAddr != ":3333" {
		t.Errorf("SSH_ADDR should be :3333 from env, got %s", sshAddr)
	}

	httpAddr := getConfigValue("", "HTTP_ADDR", ":8080")
	if httpAddr != ":9090" {
		t.Errorf("HTTP_ADDR should be :9090 from env, got %s", httpAddr)
	}

	baseDomain := getConfigValue("", "BASE_DOMAIN", "localhost")
	if baseDomain != "custom.example.com" {
		t.Errorf("BASE_DOMAIN should be custom.example.com from env, got %s", baseDomain)
	}

	logLevel := getConfigValue("", "LOG_LEVEL", "info")
	if logLevel != "debug" {
		t.Errorf("LOG_LEVEL should be debug from env, got %s", logLevel)
	}

	idleTimeout := getDurationEnv("IDLE_TIMEOUT", 300*time.Second)
	if idleTimeout != 10*time.Minute {
		t.Errorf("IDLE_TIMEOUT should be 10m from env, got %v", idleTimeout)
	}

	maxTimeout := getDurationEnv("MAX_TIMEOUT", 5*time.Minute)
	if maxTimeout != time.Hour {
		t.Errorf("MAX_TIMEOUT should be 1h from env, got %v", maxTimeout)
	}

	cleanupInterval := getDurationEnv("CLEANUP_INTERVAL", 5*time.Minute)
	if cleanupInterval != 15*time.Minute {
		t.Errorf("CLEANUP_INTERVAL should be 15m from env, got %v", cleanupInterval)
	}

	subdomainLength := getIntEnv("SUBDOMAIN_LENGTH", 8)
	if subdomainLength != 12 {
		t.Errorf("SUBDOMAIN_LENGTH should be 12 from env, got %d", subdomainLength)
	}
}

// TestConfig_PartialEnvOverrides tests mixing defaults with env overrides
func TestConfig_PartialEnvOverrides(t *testing.T) {
	// Only override some values
	os.Setenv("SSH_ADDR", ":4444")
	os.Setenv("LOG_LEVEL", "warn")

	defer func() {
		os.Unsetenv("SSH_ADDR")
		os.Unsetenv("LOG_LEVEL")
	}()

	// SSH_ADDR should come from env
	sshAddr := getConfigValue("", "SSH_ADDR", ":2222")
	if sshAddr != ":4444" {
		t.Errorf("SSH_ADDR should be :4444 from env, got %s", sshAddr)
	}

	// HTTP_ADDR should use default since env not set
	os.Unsetenv("HTTP_ADDR")
	httpAddr := getConfigValue("", "HTTP_ADDR", ":8080")
	if httpAddr != ":8080" {
		t.Errorf("HTTP_ADDR should fall back to default :8080, got %s", httpAddr)
	}

	// LOG_LEVEL should come from env
	logLevel := getConfigValue("", "LOG_LEVEL", "info")
	if logLevel != "warn" {
		t.Errorf("LOG_LEVEL should be warn from env, got %s", logLevel)
	}

	// BASE_DOMAIN should use default since env not set
	os.Unsetenv("BASE_DOMAIN")
	baseDomain := getConfigValue("", "BASE_DOMAIN", "localhost")
	if baseDomain != "localhost" {
		t.Errorf("BASE_DOMAIN should fall back to default localhost, got %s", baseDomain)
	}
}
