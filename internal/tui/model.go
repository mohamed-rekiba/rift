package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type RequestInfo struct {
	Timestamp  time.Time
	Method     string
	Path       string
	StatusCode int
	Size       int64
}

type Stats struct {
	BytesReceived int64
	BytesSent     int64
	ActiveConns   int
	TotalRequests int
}

type Model struct {
	publicURL     string
	subdomain     string
	httpPort      string
	requests      []RequestInfo
	stats         Stats
	qrCode        string
	showHelp      bool
	selectedIndex int
	showDetails   bool
	width         int
	height        int
	quitting      bool
}

type tickMsg time.Time
type requestMsg RequestInfo
type statsMsg Stats

func NewModel(publicURL, subdomain, httpPort string) Model {
	return Model{
		publicURL:     publicURL,
		subdomain:     subdomain,
		httpPort:      httpPort,
		requests:      []RequestInfo{},
		selectedIndex: 0,
		width:         80,
		height:        24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.EnterAltScreen,
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tickCmd()

	case requestMsg:
		m.requests = append([]RequestInfo{RequestInfo(msg)}, m.requests...)
		if len(m.requests) > 100 {
			m.requests = m.requests[:100]
		}
		return m, nil

	case statsMsg:
		m.stats = Stats(msg)
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		if msg.String() == "h" || msg.String() == "esc" {
			m.showHelp = false
		}
		return m, nil
	}

	if m.showDetails {
		if msg.String() == "esc" {
			m.showDetails = false
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit

	case "h":
		m.showHelp = true
		return m, nil

	case "up":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
		return m, nil

	case "down":
		if m.selectedIndex < len(m.requests)-1 {
			m.selectedIndex++
		}
		return m, nil

	case "enter":
		if len(m.requests) > 0 && m.selectedIndex < len(m.requests) {
			m.showDetails = true
		}
		return m, nil

	case "r":
		return m, tea.ClearScreen
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "\n👋 Thanks for using Rift! Your tunnel is now closed.\n\n"
	}

	if m.showHelp {
		return m.renderHelp()
	}

	if m.showDetails && m.selectedIndex < len(m.requests) {
		return m.renderRequestDetails()
	}

	return m.renderMain()
}

func (m Model) renderMain() string {
	header := m.renderHeader()
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderRequests(),
		m.renderRightPanel(),
	)
	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		content,
		"",
		footer,
	)
}

func (m Model) renderHeader() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00d9ff")).
		PaddingLeft(2)

	urlStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff87"))

	lines := []string{
		headerStyle.Render("🚀 Your tunnel is live! Share this URL:"),
		"",
		urlStyle.Render(m.publicURL),
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m Model) renderRequests() string {
	width := m.width/2 - 4
	height := m.height - 10

	if len(m.requests) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(lipgloss.Color("#666666"))

		return emptyStyle.Render("Waiting for requests...\n\nVisit your public URL to see traffic here!")
	}

	var lines []string
	visibleStart := 0
	visibleEnd := len(m.requests)

	if len(m.requests) > height {
		if m.selectedIndex >= height {
			visibleStart = m.selectedIndex - height + 1
		}
		visibleEnd = visibleStart + height
		if visibleEnd > len(m.requests) {
			visibleEnd = len(m.requests)
		}
	}

	for i := visibleStart; i < visibleEnd; i++ {
		req := m.requests[i]
		line := m.formatRequest(req, i == m.selectedIndex)
		lines = append(lines, line)
	}

	containerStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3a3a3a")).
		PaddingLeft(1).
		PaddingRight(1)

	return containerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m Model) formatRequest(req RequestInfo, selected bool) string {
	var methodColor string
	switch req.Method {
	case "GET":
		methodColor = "#00ff87"
	case "POST":
		methodColor = "#ffff00"
	case "PUT":
		methodColor = "#ff8700"
	case "DELETE":
		methodColor = "#ff0000"
	default:
		methodColor = "#ffffff"
	}

	var statusColor string
	switch {
	case req.StatusCode >= 200 && req.StatusCode < 300:
		statusColor = "#00ff87"
	case req.StatusCode >= 300 && req.StatusCode < 400:
		statusColor = "#ffff00"
	case req.StatusCode >= 400:
		statusColor = "#ff0000"
	default:
		statusColor = "#666666"
	}

	methodStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(methodColor)).
		Bold(true).
		Width(7)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(statusColor)).
		Width(4)

	pathStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#aaaaaa"))

	line := fmt.Sprintf("%s %s %s",
		methodStyle.Render(req.Method),
		statusStyle.Render(fmt.Sprintf("%d", req.StatusCode)),
		pathStyle.Render(req.Path),
	)

	if selected {
		selectedStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("#3a3a3a")).
			Bold(true)
		line = selectedStyle.Render("> " + line)
	} else {
		line = "  " + line
	}

	return line
}

func (m Model) renderRightPanel() string {
	width := m.width/2 - 4
	qrSection := m.renderQRCode(width)
	statsSection := m.renderStats(width)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		qrSection,
		"",
		statsSection,
	)
}

func (m Model) renderQRCode(width int) string {
	qrStyle := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3a3a3a")).
		PaddingTop(1).
		PaddingBottom(1)

	qrCode := m.qrCode
	if qrCode == "" {
		qrCode = generateQRCode(m.publicURL)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		qrCode,
		"",
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Render(fmt.Sprintf("← %s →", m.publicURL)),
	)

	return qrStyle.Render(content)
}

func (m Model) renderStats(width int) string {
	statsStyle := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3a3a3a")).
		PaddingLeft(2).
		PaddingRight(2).
		PaddingTop(1).
		PaddingBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Width(10)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff87")).
		Bold(true)

	lines := []string{
		fmt.Sprintf("%s  %s", labelStyle.Render("Received:"), valueStyle.Render(formatBytes(m.stats.BytesReceived))),
		fmt.Sprintf("%s  %s", labelStyle.Render("Sent:"), valueStyle.Render(formatBytes(m.stats.BytesSent))),
		fmt.Sprintf("%s  %s", labelStyle.Render("Active:"), valueStyle.Render(fmt.Sprintf("%d", m.stats.ActiveConns))),
		fmt.Sprintf("%s  %s", labelStyle.Render("Total:"), valueStyle.Render(fmt.Sprintf("%d", m.stats.TotalRequests))),
	}

	return statsStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m Model) renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		PaddingLeft(2)

	return footerStyle.Render("Press 'h' for help • 'q' to quit")
}

func (m Model) renderHelp() string {
	helpStyle := lipgloss.NewStyle().
		Width(m.width - 4).
		Height(m.height - 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00d9ff")).
		PaddingLeft(2).
		PaddingRight(2).
		PaddingTop(1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00d9ff"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff87")).
		Width(15)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#aaaaaa"))

	lines := []string{
		titleStyle.Render("⌨️  Keyboard Shortcuts"),
		"",
		fmt.Sprintf("%s  %s", keyStyle.Render("h"), descStyle.Render("Show this help")),
		fmt.Sprintf("%s  %s", keyStyle.Render("q"), descStyle.Render("Close the tunnel and exit")),
		fmt.Sprintf("%s  %s", keyStyle.Render("r"), descStyle.Render("Refresh the screen")),
		"",
		fmt.Sprintf("%s  %s", keyStyle.Render("Enter"), descStyle.Render("View request details")),
		fmt.Sprintf("%s  %s", keyStyle.Render("Esc"), descStyle.Render("Go back")),
		fmt.Sprintf("%s  %s", keyStyle.Render("↑ / ↓"), descStyle.Render("Navigate requests")),
		"",
		fmt.Sprintf("%s  %s", keyStyle.Render("Ctrl+C"), descStyle.Render("Force quit immediately")),
		"",
		"",
		descStyle.Render("Press 'h' or 'Esc' to close"),
	}

	return helpStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m Model) renderRequestDetails() string {
	if m.selectedIndex >= len(m.requests) {
		return ""
	}

	req := m.requests[m.selectedIndex]

	detailStyle := lipgloss.NewStyle().
		Width(m.width - 4).
		Height(m.height - 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00d9ff")).
		PaddingLeft(2).
		PaddingRight(2).
		PaddingTop(1)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00d9ff"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Width(15)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff87"))

	lines := []string{
		titleStyle.Render("Request Details"),
		"",
		fmt.Sprintf("%s  %s", labelStyle.Render("Method:"), valueStyle.Render(req.Method)),
		fmt.Sprintf("%s  %s", labelStyle.Render("Path:"), valueStyle.Render(req.Path)),
		fmt.Sprintf("%s  %s", labelStyle.Render("Status Code:"), valueStyle.Render(fmt.Sprintf("%d", req.StatusCode))),
		fmt.Sprintf("%s  %s", labelStyle.Render("Size:"), valueStyle.Render(formatBytes(req.Size))),
		fmt.Sprintf("%s  %s", labelStyle.Render("Timestamp:"), valueStyle.Render(req.Timestamp.Format("2006-01-02 15:04:05"))),
		"",
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#666666")).Render("Press 'Esc' to return"),
	}

	return detailStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
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
