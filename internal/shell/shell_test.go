package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveShell(t *testing.T) {
	t.Run("explicit wins", func(t *testing.T) {
		t.Setenv("SHELL", "/bin/bash")
		got, err := ResolveShell("/bin/sh")
		if err != nil {
			t.Fatal(err)
		}
		if got != "/bin/sh" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("missing explicit falls through", func(t *testing.T) {
		got, err := ResolveShell(filepath.Join(t.TempDir(), "missing"))
		if err != nil {
			t.Fatal(err)
		}
		if got != "/bin/sh" {
			t.Fatalf("got %q, want /bin/sh", got)
		}
	})

	t.Run("SHELL then fallback", func(t *testing.T) {
		t.Setenv("SHELL", filepath.Join(t.TempDir(), "missing-shell"))
		got, err := ResolveShell("")
		if err != nil {
			t.Fatal(err)
		}
		if got != "/bin/sh" {
			t.Fatalf("got %q, want /bin/sh", got)
		}
	})
}

func TestShellArgs(t *testing.T) {
	if got := strings.Join(shellArgs("/bin/zsh"), " "); got != "-i" {
		t.Fatalf("zsh args = %q", got)
	}
	if got := strings.Join(shellArgs("/usr/local/bin/bash"), " "); got != "-i" {
		t.Fatalf("bash args = %q", got)
	}
	if args := shellArgs("/usr/local/bin/nu"); len(args) != 0 {
		t.Fatalf("unknown shell args = %v", args)
	}
	for _, args := range [][]string{shellArgs("/bin/bash"), shellArgs("/bin/zsh"), shellArgs("/bin/sh")} {
		for _, a := range args {
			if a == "-l" || a == "--login" {
				t.Fatalf("login flag in %v", args)
			}
		}
	}
}

func TestFormatBannerAndDisplayPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".mywt", "worktree-20260730123456")
	pretty := displayPath(dir)
	if pretty != filepath.Join("~", ".mywt", "worktree-20260730123456") {
		t.Fatalf("displayPath = %q", pretty)
	}
	banner := FormatBanner("auth-refactor", pretty)
	for _, want := range []string{
		"[mywt: auth-refactor]",
		pretty,
		"Session root",
		"not a git repository",
		"claude and codex",
		"permissions and trust settings",
		"Type exit",
	} {
		if !strings.Contains(banner, want) {
			t.Errorf("banner missing %q:\n%s", want, banner)
		}
	}
}

func TestLaunchUsesSessionRootAndInteractiveArgs(t *testing.T) {
	root := t.TempDir()
	sessionDir := filepath.Join(root, "worktree-20260730123456")
	if err := os.MkdirAll(filepath.Join(sessionDir, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "frontend"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	shellPath := filepath.Join(root, "bash")
	script := "#!/bin/sh\nprintf 'ARGS:%s\\n' \"$*\"\nprintf 'PWD:%s\\n' \"$PWD\"\nprintf 'SESSION:%s\\n' \"$MYWT_SESSION\"\nprintf 'ROOT:%s\\n' \"$MYWT_SESSION_ROOT\"\nls\n"
	if err := os.WriteFile(shellPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	parent, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = Launch(Options{
		Shell:     shellPath,
		Dir:       sessionDir,
		Label:     "auth-refactor",
		SessionID: "worktree-20260730123456",
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Env:       []string{"PATH=" + os.Getenv("PATH")},
	})
	if err != nil {
		t.Fatalf("Launch: %v\nstderr:\n%s", err, stderr.String())
	}
	got, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got != parent {
		t.Fatalf("parent cwd changed from %s to %s", parent, got)
	}
	out := stdout.String()
	for _, want := range []string{
		"ARGS:--rcfile",
		"-i",
		"PWD:" + sessionDir,
		"SESSION:auth-refactor",
		"ROOT:" + sessionDir,
		"backend",
		"frontend",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\033]0;") || strings.Contains(stderr.String(), "\033") {
		t.Errorf("title or styling leaked into a non-terminal:\nstdout:\n%s\nstderr:\n%s", out, stderr.String())
	}
	if strings.Contains(stderr.String(), "╭") || strings.Contains(stderr.String(), "[mywt:") {
		t.Fatalf("non-TTY stderr drew a box or the legacy banner:\n%s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "mywt: auth-refactor") || !strings.Contains(stderr.String(), "(exit 0)") {
		t.Fatalf("plain banner missing:\n%s", stderr.String())
	}
}

func TestLaunchRealShellInSessionRoot(t *testing.T) {
	sessionDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sessionDir, "backend.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("pwd\nprintf 'SESSION:%s\\n' \"$MYWT_SESSION\"\nls\nexit 0\n")
	err = Launch(Options{
		Shell:     "/bin/sh",
		Dir:       sessionDir,
		Label:     "auth-refactor",
		SessionID: "worktree-1",
		Stdin:     stdin,
		Stdout:    &stdout,
		Stderr:    &stderr,
		Env: []string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + t.TempDir(),
			"TERM=dumb",
			"PS1=$ ",
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	got, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got != parent {
		t.Fatalf("parent cwd = %s, want %s", got, parent)
	}
	out := stdout.String()
	if !strings.Contains(out, sessionDir) {
		t.Fatalf("pwd missing session root:\n%s\nstderr:\n%s", out, stderr.String())
	}
	if !strings.Contains(out, "SESSION:auth-refactor") {
		t.Fatalf("env missing:\n%s", out)
	}
	if !strings.Contains(out, "backend.txt") {
		t.Fatalf("ls missing child entry:\n%s", out)
	}
}

func TestLaunchMissingDirectory(t *testing.T) {
	err := Launch(Options{
		Shell: "/bin/sh",
		Dir:   filepath.Join(t.TempDir(), "missing"),
		Label: "auth-refactor",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("err = %v", err)
	}
}

func TestIsInteractivePipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if IsInteractive(r, w) {
		t.Fatal("a pipe should not be interactive")
	}
}
