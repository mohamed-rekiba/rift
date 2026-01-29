package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	publicURL := "http://abc123.example.com"
	subdomain := "abc123"
	httpPort := ":8080"

	model := NewModel(publicURL, subdomain, httpPort)

	if model.publicURL != publicURL {
		t.Errorf("expected publicURL %s, got %s", publicURL, model.publicURL)
	}
	if model.subdomain != subdomain {
		t.Errorf("expected subdomain %s, got %s", subdomain, model.subdomain)
	}
	if model.httpPort != httpPort {
		t.Errorf("expected httpPort %s, got %s", httpPort, model.httpPort)
	}
	if len(model.requests) != 0 {
		t.Errorf("expected 0 requests, got %d", len(model.requests))
	}
	if model.selectedIndex != 0 {
		t.Errorf("expected selectedIndex 0, got %d", model.selectedIndex)
	}
	if model.width != 80 {
		t.Errorf("expected default width 80, got %d", model.width)
	}
	if model.height != 24 {
		t.Errorf("expected default height 24, got %d", model.height)
	}
	if model.quitting {
		t.Error("expected quitting to be false initially")
	}
}

func TestModel_Init(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	cmd := model.Init()

	if cmd == nil {
		t.Error("expected non-nil command from Init")
	}
}

func TestModel_Update_WindowSizeMsg(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, _ := model.Update(msg)

	m := newModel.(Model)
	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestModel_Update_RequestMsg(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	req := RequestInfo{
		Timestamp:  time.Now(),
		Method:     "GET",
		Path:       "/api/users",
		StatusCode: 200,
		Size:       1024,
	}

	newModel, _ := model.Update(requestMsg(req))

	m := newModel.(Model)
	if len(m.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(m.requests))
	}
	if m.requests[0].Method != "GET" {
		t.Errorf("expected method GET, got %s", m.requests[0].Method)
	}
	if m.requests[0].Path != "/api/users" {
		t.Errorf("expected path /api/users, got %s", m.requests[0].Path)
	}
}

func TestModel_Update_RequestMsg_MaxRequests(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	// Add 105 requests
	for i := 0; i < 105; i++ {
		req := RequestInfo{
			Timestamp:  time.Now(),
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
		}
		newModel, _ := model.Update(requestMsg(req))
		model = newModel.(Model)
	}

	// Should cap at 100 requests
	if len(model.requests) != 100 {
		t.Errorf("expected max 100 requests, got %d", len(model.requests))
	}
}

func TestModel_Update_StatsMsg(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	stats := Stats{
		BytesReceived: 1000,
		BytesSent:     500,
		ActiveConns:   5,
		TotalRequests: 10,
	}

	newModel, _ := model.Update(statsMsg(stats))

	m := newModel.(Model)
	if m.stats.BytesReceived != 1000 {
		t.Errorf("expected BytesReceived 1000, got %d", m.stats.BytesReceived)
	}
	if m.stats.BytesSent != 500 {
		t.Errorf("expected BytesSent 500, got %d", m.stats.BytesSent)
	}
	if m.stats.ActiveConns != 5 {
		t.Errorf("expected ActiveConns 5, got %d", m.stats.ActiveConns)
	}
	if m.stats.TotalRequests != 10 {
		t.Errorf("expected TotalRequests 10, got %d", m.stats.TotalRequests)
	}
}

func TestModel_Update_TickMsg(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	newModel, cmd := model.Update(tickMsg(time.Now()))

	if newModel == nil {
		t.Error("expected non-nil model")
	}
	if cmd == nil {
		t.Error("expected non-nil command (should schedule next tick)")
	}
}

func TestModel_HandleKeyPress_Quit(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"q key", "q"},
		{"ctrl+c", "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("http://test.localhost", "test", ":8080")

			newModel, cmd := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})

			m := newModel.(Model)
			if !m.quitting {
				t.Error("expected quitting to be true")
			}
			if cmd == nil {
				t.Error("expected quit command")
			}
		})
	}
}

func TestModel_HandleKeyPress_Help(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})

	m := newModel.(Model)
	if !m.showHelp {
		t.Error("expected showHelp to be true")
	}
}

func TestModel_HandleKeyPress_HelpClose(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.showHelp = true

	tests := []struct {
		name string
		key  string
	}{
		{"h key", "h"},
		{"esc key", "esc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model
			m.showHelp = true

			newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})

			result := newModel.(Model)
			if result.showHelp {
				t.Error("expected showHelp to be false")
			}
		})
	}
}

func TestModel_HandleKeyPress_Navigation(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	// Add some requests
	for i := 0; i < 5; i++ {
		model.requests = append(model.requests, RequestInfo{
			Timestamp:  time.Now(),
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
		})
	}

	// Test down navigation
	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m := newModel.(Model)
	if m.selectedIndex != 1 {
		t.Errorf("expected selectedIndex 1 after down, got %d", m.selectedIndex)
	}

	// Test up navigation
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)
	if m.selectedIndex != 0 {
		t.Errorf("expected selectedIndex 0 after up, got %d", m.selectedIndex)
	}
}

func TestModel_HandleKeyPress_NavigationBounds(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.requests = []RequestInfo{
		{Method: "GET", Path: "/1"},
		{Method: "GET", Path: "/2"},
		{Method: "GET", Path: "/3"},
	}

	// Can't go below 0
	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	m := newModel.(Model)
	if m.selectedIndex != 0 {
		t.Errorf("expected selectedIndex 0 (can't go below), got %d", m.selectedIndex)
	}

	// Move to end
	m.selectedIndex = 2
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)
	if m.selectedIndex != 2 {
		t.Errorf("expected selectedIndex 2 (can't exceed), got %d", m.selectedIndex)
	}
}

func TestModel_HandleKeyPress_Enter(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.requests = []RequestInfo{
		{Method: "GET", Path: "/test"},
	}

	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	m := newModel.(Model)
	if !m.showDetails {
		t.Error("expected showDetails to be true")
	}
}

func TestModel_HandleKeyPress_EnterNoRequests(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	m := newModel.(Model)
	if m.showDetails {
		t.Error("expected showDetails to be false when no requests")
	}
}

func TestModel_HandleKeyPress_EscDetails(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.showDetails = true

	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})

	m := newModel.(Model)
	if m.showDetails {
		t.Error("expected showDetails to be false after esc")
	}
}

func TestModel_HandleKeyPress_Refresh(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	_, cmd := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	// Should return ClearScreen command
	if cmd == nil {
		t.Error("expected non-nil command for refresh")
	}
}

func TestModel_View_Quitting(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.quitting = true

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
	if len(view) == 0 {
		t.Error("expected goodbye message")
	}
}

func TestModel_View_Help(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.showHelp = true

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_View_Details(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.showDetails = true
	model.requests = []RequestInfo{
		{
			Timestamp:  time.Now(),
			Method:     "POST",
			Path:       "/api/data",
			StatusCode: 201,
			Size:       2048,
		},
	}

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestModel_View_Main(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	view := model.View()

	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero bytes", 0, "0 B"},
		{"small bytes", 100, "100 B"},
		{"1023 bytes", 1023, "1023 B"},
		{"1 KB", 1024, "1.0 KB"},
		{"1.5 KB", 1536, "1.5 KB"},
		{"1 MB", 1024 * 1024, "1.0 MB"},
		{"1 GB", 1024 * 1024 * 1024, "1.0 GB"},
		{"mixed KB", 2500, "2.4 KB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.want {
				t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.want)
			}
		})
	}
}

func TestModel_FormatRequest(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	tests := []struct {
		name     string
		req      RequestInfo
		selected bool
	}{
		{
			name: "GET request",
			req: RequestInfo{
				Method:     "GET",
				Path:       "/api/users",
				StatusCode: 200,
			},
			selected: false,
		},
		{
			name: "POST request selected",
			req: RequestInfo{
				Method:     "POST",
				Path:       "/api/data",
				StatusCode: 201,
			},
			selected: true,
		},
		{
			name: "PUT request",
			req: RequestInfo{
				Method:     "PUT",
				Path:       "/api/update",
				StatusCode: 200,
			},
			selected: false,
		},
		{
			name: "DELETE request",
			req: RequestInfo{
				Method:     "DELETE",
				Path:       "/api/item",
				StatusCode: 204,
			},
			selected: false,
		},
		{
			name: "error response",
			req: RequestInfo{
				Method:     "GET",
				Path:       "/not-found",
				StatusCode: 404,
			},
			selected: false,
		},
		{
			name: "redirect response",
			req: RequestInfo{
				Method:     "GET",
				Path:       "/redirect",
				StatusCode: 301,
			},
			selected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := model.formatRequest(tt.req, tt.selected)
			if result == "" {
				t.Error("expected non-empty formatted request")
			}
		})
	}
}

func TestRequestInfo(t *testing.T) {
	now := time.Now()
	req := RequestInfo{
		Timestamp:  now,
		Method:     "GET",
		Path:       "/test",
		StatusCode: 200,
		Size:       1024,
	}

	if req.Timestamp != now {
		t.Error("expected timestamp to match")
	}
	if req.Method != "GET" {
		t.Error("expected method GET")
	}
	if req.Path != "/test" {
		t.Error("expected path /test")
	}
	if req.StatusCode != 200 {
		t.Error("expected status code 200")
	}
	if req.Size != 1024 {
		t.Error("expected size 1024")
	}
}

func TestStats(t *testing.T) {
	stats := Stats{
		BytesReceived: 1000,
		BytesSent:     500,
		ActiveConns:   5,
		TotalRequests: 10,
	}

	if stats.BytesReceived != 1000 {
		t.Error("expected BytesReceived 1000")
	}
	if stats.BytesSent != 500 {
		t.Error("expected BytesSent 500")
	}
	if stats.ActiveConns != 5 {
		t.Error("expected ActiveConns 5")
	}
	if stats.TotalRequests != 10 {
		t.Error("expected TotalRequests 10")
	}
}
