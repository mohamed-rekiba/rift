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
		t.Errorf("publicURL: got %s, want %s", model.publicURL, publicURL)
	}
	if model.subdomain != subdomain {
		t.Errorf("subdomain: got %s, want %s", model.subdomain, subdomain)
	}
	if model.httpPort != httpPort {
		t.Errorf("httpPort: got %s, want %s", model.httpPort, httpPort)
	}
	if len(model.requests) != 0 {
		t.Errorf("new model should have 0 requests, got %d", len(model.requests))
	}
	if model.selectedIndex != 0 {
		t.Errorf("selectedIndex should start at 0, got %d", model.selectedIndex)
	}
	if model.width != 80 {
		t.Errorf("default width should be 80, got %d", model.width)
	}
	if model.height != 24 {
		t.Errorf("default height should be 24, got %d", model.height)
	}
	if model.quitting {
		t.Error("new model should not be quitting")
	}
}

func TestModel_Init(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	cmd := model.Init()

	if cmd == nil {
		t.Error("Init should return a command (for the ticker)")
	}
}

func TestModel_Update_WindowSizeMsg(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, _ := model.Update(msg)

	m := newModel.(Model)
	if m.width != 120 {
		t.Errorf("width should be 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("height should be 40, got %d", m.height)
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
		t.Fatalf("should have 1 request, got %d", len(m.requests))
	}
	if m.requests[0].Method != "GET" {
		t.Errorf("method should be GET, got %s", m.requests[0].Method)
	}
	if m.requests[0].Path != "/api/users" {
		t.Errorf("path should be /api/users, got %s", m.requests[0].Path)
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

	// Should cap at 100 to avoid memory bloat
	if len(model.requests) != 100 {
		t.Errorf("should cap at 100 requests, got %d", len(model.requests))
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
		t.Errorf("BytesReceived: got %d, want 1000", m.stats.BytesReceived)
	}
	if m.stats.BytesSent != 500 {
		t.Errorf("BytesSent: got %d, want 500", m.stats.BytesSent)
	}
	if m.stats.ActiveConns != 5 {
		t.Errorf("ActiveConns: got %d, want 5", m.stats.ActiveConns)
	}
	if m.stats.TotalRequests != 10 {
		t.Errorf("TotalRequests: got %d, want 10", m.stats.TotalRequests)
	}
}

func TestModel_Update_TickMsg(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	newModel, cmd := model.Update(tickMsg(time.Now()))

	if newModel == nil {
		t.Error("model should not be nil")
	}
	if cmd == nil {
		t.Error("tick should schedule another tick")
	}
}

func TestModel_HandleKeyPress_Quit(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"q quits", "q"},
		{"ctrl+c quits", "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel("http://test.localhost", "test", ":8080")

			newModel, cmd := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})

			m := newModel.(Model)
			if !m.quitting {
				t.Error("model should be quitting")
			}
			if cmd == nil {
				t.Error("should return quit command")
			}
		})
	}
}

func TestModel_HandleKeyPress_Help(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})

	m := newModel.(Model)
	if !m.showHelp {
		t.Error("pressing h should show help")
	}
}

func TestModel_HandleKeyPress_HelpClose(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.showHelp = true

	tests := []struct {
		name string
		key  string
	}{
		{"h closes help", "h"},
		{"esc closes help", "esc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model
			m.showHelp = true

			newModel, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})

			result := newModel.(Model)
			if result.showHelp {
				t.Error("help should be closed")
			}
		})
	}
}

func TestModel_HandleKeyPress_Navigation(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	// Add some requests to navigate through
	for i := 0; i < 5; i++ {
		model.requests = append(model.requests, RequestInfo{
			Timestamp:  time.Now(),
			Method:     "GET",
			Path:       "/test",
			StatusCode: 200,
		})
	}

	// Down arrow should move selection
	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m := newModel.(Model)
	if m.selectedIndex != 1 {
		t.Errorf("down arrow: index should be 1, got %d", m.selectedIndex)
	}

	// Up arrow should move back
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(Model)
	if m.selectedIndex != 0 {
		t.Errorf("up arrow: index should be 0, got %d", m.selectedIndex)
	}
}

func TestModel_HandleKeyPress_NavigationBounds(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.requests = []RequestInfo{
		{Method: "GET", Path: "/1"},
		{Method: "GET", Path: "/2"},
		{Method: "GET", Path: "/3"},
	}

	// Can't go above the first item
	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	m := newModel.(Model)
	if m.selectedIndex != 0 {
		t.Errorf("should stay at 0 when pressing up, got %d", m.selectedIndex)
	}

	// Can't go below the last item
	m.selectedIndex = 2
	newModel, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(Model)
	if m.selectedIndex != 2 {
		t.Errorf("should stay at 2 when pressing down at end, got %d", m.selectedIndex)
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
		t.Error("enter should show request details")
	}
}

func TestModel_HandleKeyPress_EnterNoRequests(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})

	m := newModel.(Model)
	if m.showDetails {
		t.Error("enter with no requests should not show details")
	}
}

func TestModel_HandleKeyPress_EscDetails(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.showDetails = true

	newModel, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})

	m := newModel.(Model)
	if m.showDetails {
		t.Error("esc should close details view")
	}
}

func TestModel_HandleKeyPress_Refresh(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	_, cmd := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	if cmd == nil {
		t.Error("r should trigger a screen refresh command")
	}
}

func TestModel_View_Quitting(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.quitting = true

	view := model.View()

	if view == "" {
		t.Error("should show something when quitting")
	}
}

func TestModel_View_Help(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")
	model.showHelp = true

	view := model.View()

	if view == "" {
		t.Error("should show help content")
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
		t.Error("should show request details")
	}
}

func TestModel_View_Main(t *testing.T) {
	model := NewModel("http://test.localhost", "test", ":8080")

	view := model.View()

	if view == "" {
		t.Error("should show main dashboard view")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"small", 100, "100 B"},
		{"just under 1KB", 1023, "1023 B"},
		{"exactly 1KB", 1024, "1.0 KB"},
		{"1.5KB", 1536, "1.5 KB"},
		{"1MB", 1024 * 1024, "1.0 MB"},
		{"1GB", 1024 * 1024 * 1024, "1.0 GB"},
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
			name: "404 response",
			req: RequestInfo{
				Method:     "GET",
				Path:       "/not-found",
				StatusCode: 404,
			},
			selected: false,
		},
		{
			name: "redirect",
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
				t.Error("formatted request should not be empty")
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
		t.Error("timestamp should match")
	}
	if req.Method != "GET" {
		t.Error("method should be GET")
	}
	if req.Path != "/test" {
		t.Error("path should be /test")
	}
	if req.StatusCode != 200 {
		t.Error("status code should be 200")
	}
	if req.Size != 1024 {
		t.Error("size should be 1024")
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
		t.Error("BytesReceived should be 1000")
	}
	if stats.BytesSent != 500 {
		t.Error("BytesSent should be 500")
	}
	if stats.ActiveConns != 5 {
		t.Error("ActiveConns should be 5")
	}
	if stats.TotalRequests != 10 {
		t.Error("TotalRequests should be 10")
	}
}
