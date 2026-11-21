package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fxricky/mywt/internal/session"
)

func TestSessionsNewestFirst(t *testing.T) {
	in := []*session.Session{{ID: "worktree-1"}, {ID: "worktree-2"}, {ID: "worktree-3"}}
	got := sessionsNewestFirst(in)
	if len(got) != 3 || got[0].ID != "worktree-3" || got[2].ID != "worktree-1" {
		t.Fatalf("got %#v", got)
	}
	if in[0].ID != "worktree-1" {
		t.Fatal("input slice was reversed in place")
	}
	if sessionsNewestFirst(nil) != nil {
		t.Fatal("nil should stay nil")
	}
}

func TestSelectorMessages(t *testing.T) {
	non := nonInteractiveMessage()
	for _, want := range []string{"needs a terminal", "mywt list", "mywt open <session> --shell"} {
		if !strings.Contains(non, want) {
			t.Errorf("non-interactive message missing %q:\n%s", want, non)
		}
	}
	empty := emptySessionsMessage()
	if !strings.Contains(empty, "mywt create") || !strings.Contains(empty, "--name") {
		t.Fatalf("empty message = %q", empty)
	}
}

func TestRootHelpDocumentsSelector(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	t.Cleanup(func() { rootCmd.SetOut(nil) })
	if err := rootCmd.Help(); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	for _, want := range []string{
		"session root",
		"exit",
		"tmux",
		"auth-refactor",
		"wt-<timestamp>",
		"does not edit shell rc files",
		"--no-frame",
		"--no-prompt-tag",
		"(mywt:<name>) ",
		"does not edit",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("root help missing %q\n%s", want, text)
		}
	}
}

func TestAddHelpDocumentsBranchDefault(t *testing.T) {
	var buf bytes.Buffer
	addCmd.SetOut(&buf)
	t.Cleanup(func() { addCmd.SetOut(nil) })
	if err := addCmd.Help(); err != nil {
		t.Fatal(err)
	}
	text := strings.Join(strings.Fields(buf.String()), " ")
	for _, want := range []string{"session name", "--branch", "unnamed", "will not reuse, reset, move, or delete"} {
		if !strings.Contains(text, want) {
			t.Errorf("add help missing %q\n%s", want, text)
		}
	}
}
