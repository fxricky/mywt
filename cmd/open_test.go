package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fxricky/mywt/internal/session"
)

func TestNoFrameFlag(t *testing.T) {
	prev := noFrame
	t.Cleanup(func() {
		noFrame = prev
		_ = rootCmd.Flags().Set("no-frame", "false")
	})
	if err := rootCmd.Flags().Set("no-frame", "true"); err != nil {
		t.Fatal(err)
	}
	if !noFrame {
		t.Fatal("--no-frame did not set the flag")
	}
}

func TestOpenShellUnchanged(t *testing.T) {
	if os.Getenv("MYWT_OPEN_CHILD") == "1" {
		rootCmd.SetArgs([]string{"open", os.Getenv("MYWT_OPEN_ID"), "--shell"})
		if err := rootCmd.Execute(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	home := t.TempDir()
	m := session.New(filepath.Join(home, ".mywt"))
	s, err := m.Create(session.CreateOptions{Name: "auth-refactor"})
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "shell")
	body := "#!/bin/sh\nprintf 'OPEN_SHELL\\n'\nprintf 'ARGS:%s\\n' \"$*\"\nprintf 'ZDOT:%s\\n' \"${ZDOTDIR-}\"\nprintf 'ENVV:%s\\n' \"${ENV-}\"\nprintf 'TAG:%s\\n' \"${MYWT_PROMPT_TAG-}\"\npwd\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestOpenShellUnchanged$")
	cmd.Env = []string{
		"MYWT_OPEN_CHILD=1",
		"MYWT_OPEN_ID=" + s.ID,
		"HOME=" + home,
		"SHELL=" + script,
		"PATH=" + os.Getenv("PATH"),
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("open --shell: %v\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "OPEN_SHELL") || !strings.Contains(text, s.Dir()) {
		t.Fatalf("shell did not run in the session directory:\n%s", text)
	}
	for _, bad := range []string{"╭", "╰", "[mywt:", "left mywt", "Type exit to return", "\033[22;0t", "(mywt:", "--rcfile", "mywt-prompt-"} {
		if strings.Contains(text, bad) {
			t.Fatalf("open --shell changed (%q):\n%s", bad, text)
		}
	}
	if !strings.Contains(text, "ARGS:\n") || !strings.Contains(text, "ZDOT:\n") || !strings.Contains(text, "ENVV:\n") || !strings.Contains(text, "TAG:\n") {
		t.Fatalf("open --shell env was not empty:\n%s", text)
	}
}
