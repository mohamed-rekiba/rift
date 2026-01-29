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
		t.Fatal("expected non-nil session")
	}
	if session.model == nil {
		t.Error("expected model to be initialized")
	}
	if session.logger != logger {
		t.Error("expected logger to be set")
	}
	if session.program != nil {
		t.Error("expected program to be nil before Start()")
	}
}

func TestSession_ModelInitialization(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "testsub", ":8080", logger)

	// Check model fields are set correctly
	if session.model.publicURL != "http://test.localhost" {
		t.Errorf("expected publicURL http://test.localhost, got %s", session.model.publicURL)
	}
	if session.model.subdomain != "testsub" {
		t.Errorf("expected subdomain testsub, got %s", session.model.subdomain)
	}
	if session.model.httpPort != ":8080" {
		t.Errorf("expected httpPort :8080, got %s", session.model.httpPort)
	}
}

func TestSession_AddRequest_BeforeStart(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Adding request before Start() should not panic
	// program is nil, so AddRequest should handle this gracefully
	session.AddRequest(RequestInfo{
		Timestamp:  time.Now(),
		Method:     "GET",
		Path:       "/test",
		StatusCode: 200,
		Size:       1024,
	})

	// Should not panic - test passes if we get here
}

func TestSession_UpdateStats_BeforeStart(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Updating stats before Start() should not panic
	// program is nil, so UpdateStats should handle this gracefully
	session.UpdateStats(Stats{
		BytesReceived: 1000,
		BytesSent:     500,
		ActiveConns:   5,
		TotalRequests: 10,
	})

	// Should not panic - test passes if we get here
}

func TestSession_Stop_BeforeStart(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Stop before Start() should not panic
	session.Stop()

	// Should not panic - test passes if we get here
}

func TestSession_Write(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Write should return successfully (discards data)
	data := []byte("test data")
	n, err := session.Write(data)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
}

func TestSession_Write_Empty(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Write empty slice
	n, err := session.Write([]byte{})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes written, got %d", n)
	}
}

func TestSession_Write_Large(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Write large data
	data := make([]byte, 1024*1024) // 1MB
	n, err := session.Write(data)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
}

func TestSession_MultipleRequests(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Add multiple requests before Start() - should all be safe
	requests := []RequestInfo{
		{Timestamp: time.Now(), Method: "GET", Path: "/1", StatusCode: 200, Size: 100},
		{Timestamp: time.Now(), Method: "POST", Path: "/2", StatusCode: 201, Size: 200},
		{Timestamp: time.Now(), Method: "PUT", Path: "/3", StatusCode: 200, Size: 300},
		{Timestamp: time.Now(), Method: "DELETE", Path: "/4", StatusCode: 204, Size: 0},
	}

	for _, req := range requests {
		session.AddRequest(req)
	}

	// Should not panic - test passes if we get here
}

func TestSession_MultipleStatsUpdates(t *testing.T) {
	logger := newTestLogger()
	session := NewSession("http://test.localhost", "test", ":8080", logger)

	// Multiple stats updates before Start() - should all be safe
	for i := 0; i < 10; i++ {
		session.UpdateStats(Stats{
			BytesReceived: int64(i * 100),
			BytesSent:     int64(i * 50),
			ActiveConns:   i,
			TotalRequests: i,
		})
	}

	// Should not panic - test passes if we get here
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

	// Should not panic - test passes if we get here
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
		t.Error("expected zero BytesReceived")
	}
	if stats.BytesSent != 0 {
		t.Error("expected zero BytesSent")
	}
	if stats.ActiveConns != 0 {
		t.Error("expected zero ActiveConns")
	}
	if stats.TotalRequests != 0 {
		t.Error("expected zero TotalRequests")
	}
}
