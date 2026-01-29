package tui

import (
	"io"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gliderlabs/ssh"
)

type Session struct {
	program    *tea.Program
	model      *Model
	logger     *slog.Logger
	sshSession ssh.Session
}

func NewSession(publicURL, subdomain, httpPort string, logger *slog.Logger) *Session {
	model := NewModel(publicURL, subdomain, httpPort)

	return &Session{
		model:  &model,
		logger: logger,
	}
}

// Start runs the interactive TUI dashboard for the tunnel session.
func (s *Session) Start(sshSession ssh.Session) error {
	s.sshSession = sshSession

	s.program = tea.NewProgram(
		*s.model,
		tea.WithInput(sshSession),
		tea.WithOutput(sshSession),
		tea.WithAltScreen(),
	)

	finalModel, err := s.program.Run()
	if err != nil {
		return err
	}

	// Say goodbye when the user quits
	if m, ok := finalModel.(Model); ok && m.quitting {
		_, _ = io.WriteString(sshSession, "\n👋 Thanks for using Rift! Your tunnel is now closed.\n\n")
	}

	return nil
}

func (s *Session) AddRequest(req RequestInfo) {
	if s.program != nil {
		s.program.Send(requestMsg(req))
	}
}

func (s *Session) UpdateStats(stats Stats) {
	if s.program != nil {
		s.program.Send(statsMsg(stats))
	}
}

func (s *Session) Stop() {
	if s.program != nil {
		s.program.Quit()
	}
}

func (s *Session) Write(p []byte) (n int, err error) {
	return io.Discard.Write(p)
}
