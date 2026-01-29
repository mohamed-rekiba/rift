package tui

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewSession(t *testing.T) {
	logger := newTestLogger()
	publicURL := "http://test.localhost:8080"
	subdomain := "test"
	httpPort := ":8080"

	session := NewSession(publicURL, subdomain, httpPort, logger)

	if session == nil {
		t.Fatal("NewSession returned nil")
	}
	if session.model == nil {
		t.Error("model should be initialized")
	}
	if session.logger != logger {
		t.Error("logger should be set")
	}
	if session.program != nil {
		t.Error("program should be nil until Start() is called")
	}
}

func TestSession_ModelInitialization(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "testsub", ":8080", logger)

	if session.model.publicURL != "http://test.localhost" {
		t.Errorf("publicURL: got %s, want http://test.localhost", session.model.publicURL)
	}
	if session.model.subdomain != "testsub" {
		t.Errorf("subdomain: got %s, want testsub", session.model.subdomain)
	}
	if session.model.httpPort != ":8080" {
		t.Errorf("httpPort: got %s, want :8080", session.model.httpPort)
	}
}

func TestSession_AddRequest_BeforeStart(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Should handle gracefully when program hasn't started
	session.AddRequest(RequestInfo{
		Timestamp:  time.Now(),
		Method:     "GET",
		Path:       "/test",
		StatusCode: 200,
		Size:       1024,
	})

	// Test passes if we get here without panicking
}

func TestSession_UpdateStats_BeforeStart(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Should handle gracefully when program hasn't started
	session.UpdateStats(Stats{
		BytesReceived: 1000,
		BytesSent:     500,
		ActiveConns:   5,
		TotalRequests: 10,
	})

	// Test passes if we get here without panicking
}

func TestSession_Stop_BeforeStart(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Should handle gracefully when program hasn't started
	session.Stop()

	// Test passes if we get here without panicking
}

func TestSession_Write(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	data := []byte("test data")
	n, err := session.Write(data)

	if err != nil {
		t.Errorf("Write should not error: %v", err)
	}
	if n != len(data) {
		t.Errorf("should return %d bytes written, got %d", len(data), n)
	}
}

func TestSession_Write_Empty(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	n, err := session.Write([]byte{})

	if err != nil {
		t.Errorf("Write empty should not error: %v", err)
	}
	if n != 0 {
		t.Errorf("empty write should return 0, got %d", n)
	}
}

func TestSession_Write_Large(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Write 1MB of data
	data := make([]byte, 1024*1024)
	n, err := session.Write(data)

	if err != nil {
		t.Errorf("Write large should not error: %v", err)
	}
	if n != len(data) {
		t.Errorf("should return %d bytes written, got %d", len(data), n)
	}
}

func TestSession_MultipleRequests(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Add a bunch of requests before Start()
	requests := []RequestInfo{
		{Timestamp: time.Now(), Method: "GET", Path: "/1", StatusCode: 200, Size: 100},
		{Timestamp: time.Now(), Method: "POST", Path: "/2", StatusCode: 201, Size: 200},
		{Timestamp: time.Now(), Method: "PUT", Path: "/3", StatusCode: 200, Size: 300},
		{Timestamp: time.Now(), Method: "DELETE", Path: "/4", StatusCode: 204, Size: 0},
	}

	for _, req := range requests {
		session.AddRequest(req)
	}

	// Test passes if we get here without panicking
}

func TestSession_MultipleStatsUpdates(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Send multiple stats updates before Start()
	for i := 0; i < 10; i++ {
		session.UpdateStats(Stats{
			BytesReceived: int64(i * 100),
			BytesSent:     int64(i * 50),
			ActiveConns:   i,
			TotalRequests: i,
		})
	}

	// Test passes if we get here without panicking
}

func TestSession_ConcurrentOperations(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	done := make(chan bool, 3)

	// Concurrent AddRequest calls
	go func() {
		for i := 0; i < 100; i++ {
			session.AddRequest(RequestInfo{
				Timestamp:  time.Now(),
				Method:     "GET",
				Path:       "/concurrent",
				StatusCode: 200,
				Size:       100,
			})
		}
		done <- true
	}()

	// Concurrent UpdateStats calls
	go func() {
		for i := 0; i < 100; i++ {
			session.UpdateStats(Stats{
				BytesReceived: int64(i),
				BytesSent:     int64(i),
				ActiveConns:   i,
				TotalRequests: i,
			})
		}
		done <- true
	}()

	// Concurrent Write calls
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = session.Write([]byte("test"))
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Test passes if we get here without panicking or race conditions
}

func TestRequestInfo_Fields(t *testing.T) {
	now := time.Now()
	req := RequestInfo{
		Timestamp:  now,
		Method:     "POST",
		Path:       "/api/users",
		StatusCode: 201,
		Size:       2048,
	}

	if req.Timestamp != now {
		t.Error("Timestamp mismatch")
	}
	if req.Method != "POST" {
		t.Error("Method mismatch")
	}
	if req.Path != "/api/users" {
		t.Error("Path mismatch")
	}
	if req.StatusCode != 201 {
		t.Error("StatusCode mismatch")
	}
	if req.Size != 2048 {
		t.Error("Size mismatch")
	}
}

func TestStats_Fields(t *testing.T) {
	stats := Stats{
		BytesReceived: 10000,
		BytesSent:     5000,
		ActiveConns:   25,
		TotalRequests: 100,
	}

	if stats.BytesReceived != 10000 {
		t.Error("BytesReceived mismatch")
	}
	if stats.BytesSent != 5000 {
		t.Error("BytesSent mismatch")
	}
	if stats.ActiveConns != 25 {
		t.Error("ActiveConns mismatch")
	}
	if stats.TotalRequests != 100 {
		t.Error("TotalRequests mismatch")
	}
}

func TestStats_ZeroValues(t *testing.T) {
	stats := Stats{}

	if stats.BytesReceived != 0 {
		t.Error("BytesReceived should default to 0")
	}
	if stats.BytesSent != 0 {
		t.Error("BytesSent should default to 0")
	}
	if stats.ActiveConns != 0 {
		t.Error("ActiveConns should default to 0")
	}
	if stats.TotalRequests != 0 {
		t.Error("TotalRequests should default to 0")
	}
}
