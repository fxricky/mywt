package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fxricky/mywt/internal/picker"
	"github.com/fxricky/mywt/internal/session"
	"github.com/fxricky/mywt/internal/shell"
)

func runInteractiveSelector() {
	if !shell.IsInteractive(os.Stdin, os.Stdout) {
		fmt.Fprint(os.Stderr, nonInteractiveMessage())
		os.Exit(1)
	}

	m := loadManager()
	sessions, err := m.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if len(sessions) == 0 {
		fmt.Fprint(os.Stderr, emptySessionsMessage())
		return
	}

	ordered := sessionsNewestFirst(sessions)
	idx, err := picker.Run(os.Stdin, os.Stdout, picker.ItemsFromSessions(ordered))
	if err != nil {
		if errors.Is(err, picker.ErrCancelled) {
			fmt.Fprintln(os.Stderr, "cancelled")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if idx < 0 || idx >= len(ordered) {
		fmt.Fprintln(os.Stderr, "error: invalid session selection")
		os.Exit(1)
	}

	s := ordered[idx]
	label := strings.TrimSpace(s.Name)
	if label == "" {
		label = s.ID
	}
	err = shell.Launch(shell.Options{
		Dir:       s.Dir(),
		Label:     label,
		SessionID: s.ID,
		NoFrame:   noFrame,
	})
	var exitErr *shell.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// sessionsNewestFirst returns a copy of sessions with the newest id first.
// List returns oldest-first, which matches `mywt list`; the selector leads
// with the session most recently created.
func sessionsNewestFirst(sessions []*session.Session) []*session.Session {
	if len(sessions) == 0 {
		return nil
	}
	out := make([]*session.Session, len(sessions))
	for i, s := range sessions {
		out[len(sessions)-1-i] = s
	}
	return out
}

func nonInteractiveMessage() string {
	return "mywt: interactive session selector needs a terminal.\n" +
		"List sessions with:  mywt list\n" +
		"Open a shell at a session root with:  mywt open <session> --shell\n"
}

func emptySessionsMessage() string {
	return "No sessions yet. Create one with `mywt create` or `mywt create --name <name>`.\n"
}
