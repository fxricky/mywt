package shell

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClampWidth(t *testing.T) {
	if got := ClampWidth(10); got != 40 {
		t.Fatalf("narrow = %d, want 40", got)
	}
	if got := ClampWidth(40); got != 40 {
		t.Fatalf("min = %d", got)
	}
	if got := ClampWidth(80); got != 80 {
		t.Fatalf("max = %d", got)
	}
	if got := ClampWidth(200); got != 80 {
		t.Fatalf("wide = %d, want 80", got)
	}
}

func TestRenderBoxWidthAndTruncation(t *testing.T) {
	long := strings.Repeat("auth-refactor-", 20)
	path := "/home/" + strings.Repeat("repository/", 30) + "backend"
	cwd := "/home/" + strings.Repeat("projects/", 20)
	f := Frame{Label: long, Root: path, ReturnTo: cwd}
	for _, width := range []int{40, 80} {
		for _, unicode := range []bool{true, false} {
			out := RenderBox(f, Style{Unicode: unicode, Color: false, Width: width})
			lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
			if len(lines) != 4 {
				t.Fatalf("width %d unicode %v lines = %d\n%s", width, unicode, len(lines), out)
			}
			for _, line := range lines {
				if n := len([]rune(line)); n != width {
					t.Fatalf("width %d unicode %v line is %d cols: %q", width, unicode, n, line)
				}
			}
			if strings.Contains(out, long) || strings.Contains(out, path) {
				t.Fatalf("width %d unicode %v did not truncate\n%s", width, unicode, out)
			}
			if strings.Contains(out, "\n\n") {
				t.Fatalf("box wrapped or gained a blank line:\n%s", out)
			}
		}
	}
}

func TestRenderBoxLocaleAndColor(t *testing.T) {
	f := Frame{Label: "auth-refactor", Root: "~/.mywt/worktree-1", ReturnTo: "~/projects"}
	utf := RenderBox(f, Style{Unicode: true, Color: true, Width: 60})
	ascii := RenderBox(f, Style{Unicode: false, Color: false, Width: 60})
	for _, want := range []string{"╭", "│", "╰", "╯", "─", "mywt: auth-refactor", "~/.mywt/worktree-1", "Type exit to return to ~/projects"} {
		if !strings.Contains(utf, want) {
			t.Errorf("utf box missing %q\n%s", want, utf)
		}
	}
	if !strings.Contains(utf, "\033[1m") || !strings.Contains(utf, "\033[0m") {
		t.Fatalf("utf box has no bold:\n%s", utf)
	}
	plain := RenderBox(f, Style{Unicode: true, Color: false, Width: 60})
	if strings.Contains(plain, "\033") {
		t.Fatalf("NO_COLOR box has styling:\n%s", plain)
	}
	if strings.Contains(plain, "╭") && strings.Contains(ascii, "╭") {
		t.Fatal("ascii box used unicode")
	}
	for _, bad := range []string{"╭", "│", "╰", "─", "\033"} {
		if strings.Contains(ascii, bad) {
			t.Fatalf("ascii box contains %q\n%s", bad, ascii)
		}
	}
	for _, want := range []string{"+", "|", "-", "mywt: auth-refactor", "Type exit to return to ~/projects"} {
		if !strings.Contains(ascii, want) {
			t.Errorf("ascii box missing %q\n%s", want, ascii)
		}
	}
}

func TestRenderPlainHasNoBoxOrTitle(t *testing.T) {
	f := Frame{Label: "auth-refactor", Root: "~/.mywt/worktree-1", ReturnTo: "~/projects", Nested: "payment-hotfix"}
	got := RenderPlain(f)
	for _, want := range []string{
		"mywt: auth-refactor (nested in payment-hotfix)",
		"~/.mywt/worktree-1",
		"Type exit to return to ~/projects",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plain missing %q\n%s", want, got)
		}
	}
	for _, bad := range []string{"╭", "│", "╰", "+", "\033", "\033]0;"} {
		if strings.Contains(got, bad) {
			t.Fatalf("plain banner contains %q\n%s", bad, got)
		}
	}
}

func TestRenderFooterExitCode(t *testing.T) {
	f := Frame{Label: "auth-refactor", ReturnTo: "~/projects"}
	utf := RenderFooter(f, 7, true, 80)
	if utf != "╰─ left mywt: auth-refactor (exit 7) · back in ~/projects\n" {
		t.Fatalf("utf footer = %q", utf)
	}
	ascii := RenderFooter(f, 0, false, 80)
	if ascii != "+-- left mywt: auth-refactor (exit 0) - back in ~/projects\n" {
		t.Fatalf("ascii footer = %q", ascii)
	}
	long := RenderFooter(Frame{Label: strings.Repeat("n", 100), ReturnTo: strings.Repeat("p", 100)}, 3, true, 40)
	if !strings.Contains(long, "(exit 3)") {
		t.Fatalf("truncated footer dropped the exit code: %q", long)
	}
	if n := len([]rune(strings.TrimSuffix(long, "\n"))); n > 40 {
		t.Fatalf("footer width = %d\n%s", n, long)
	}
}

func TestNestedWarning(t *testing.T) {
	got := NestedWarning("payment-hotfix")
	if got != "mywt: warning: already in session payment-hotfix; continuing nested\n" {
		t.Fatalf("warning = %q", got)
	}
}

func TestTitleSequences(t *testing.T) {
	on := TitleSaveAndSet("auth-refactor")
	if on != "\033[22;0t\033]0;mywt: auth-refactor\a" {
		t.Fatalf("title on = %q", on)
	}
	if TitleRestore() != "\033[23;0t" {
		t.Fatalf("title off = %q", TitleRestore())
	}
}

func TestUTF8Locale(t *testing.T) {
	cases := []struct {
		env  []string
		want bool
	}{
		{[]string{"LANG=C.UTF-8"}, true},
		{[]string{"LANG=en_US.utf8"}, true},
		{[]string{"LC_ALL=C", "LANG=C.UTF-8"}, false},
		{[]string{"LC_CTYPE=en_US.UTF-8", "LANG=C"}, true},
		{[]string{"LANG=C"}, false},
		{[]string{}, false},
	}
	for _, tc := range cases {
		if got := utf8Locale(tc.env); got != tc.want {
			t.Errorf("utf8Locale(%v) = %v, want %v", tc.env, got, tc.want)
		}
	}
}

func TestLaunchPlainBannerWhenStderrIsNotATTY(t *testing.T) {
	dir := t.TempDir()
	shellPath := writeShell(t, dir, "exit 0\n")
	var stdout, stderr bytes.Buffer
	err := Launch(Options{
		Shell:     shellPath,
		Dir:       dir,
		Label:     "auth-refactor",
		SessionID: "worktree-1",
		Stdin:     strings.NewReader(""),
		Stdout:    &stdout,
		Stderr:    &stderr,
		Env:       []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := stderr.String()
	for _, want := range []string{"mywt: auth-refactor", "Type exit to return to", "(exit 0)", "╰─ left mywt: auth-refactor"} {
		if !strings.Contains(got, want) {
			t.Errorf("stderr missing %q\n%s", want, got)
		}
	}
	for _, bad := range []string{"╭", "\033", "[mywt:"} {
		if strings.Contains(got, bad) {
			t.Fatalf("non-TTY stderr contains %q\n%s", bad, got)
		}
	}
}

func TestLaunchNoFrameFallbacks(t *testing.T) {
	dir := t.TempDir()
	shellPath := writeShell(t, dir, "exit 0\n")
	want := FormatBanner("auth-refactor", displayPath(dir))

	t.Run("flag", func(t *testing.T) {
		var stderr bytes.Buffer
		err := Launch(Options{
			Shell: shellPath, Dir: dir, Label: "auth-refactor", SessionID: "worktree-1",
			Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &stderr,
			Env: []string{"PATH=" + os.Getenv("PATH")}, NoFrame: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if stderr.String() != want {
			t.Fatalf("stderr = %q\nwant %q", stderr.String(), want)
		}
	})

	t.Run("env", func(t *testing.T) {
		var stderr bytes.Buffer
		err := Launch(Options{
			Shell: shellPath, Dir: dir, Label: "auth-refactor", SessionID: "worktree-1",
			Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &stderr,
			Env: []string{"PATH=" + os.Getenv("PATH"), "MYWT_NO_FRAME=1"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if stderr.String() != want {
			t.Fatalf("stderr = %q\nwant %q", stderr.String(), want)
		}
	})
}

func TestLaunchNestedWarning(t *testing.T) {
	dir := t.TempDir()
	shellPath := writeShell(t, dir, "exit 0\n")
	var stderr bytes.Buffer
	err := Launch(Options{
		Shell: shellPath, Dir: dir, Label: "auth-refactor", SessionID: "worktree-2",
		Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &stderr,
		Env: []string{"PATH=" + os.Getenv("PATH"), "MYWT_SESSION=payment-hotfix", "LANG=C"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := stderr.String()
	if !strings.Contains(got, NestedWarning("payment-hotfix")) {
		t.Fatalf("missing warning:\n%s", got)
	}
	if !strings.Contains(got, "mywt: auth-refactor (nested in payment-hotfix)") {
		t.Fatalf("missing nested label:\n%s", got)
	}
}

func TestLaunchFooterUsesRealExitCode(t *testing.T) {
	dir := t.TempDir()
	shellPath := writeShell(t, dir, "exit 7\n")
	var stderr bytes.Buffer
	err := Launch(Options{
		Shell: shellPath, Dir: dir, Label: "auth-refactor", SessionID: "worktree-1",
		Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &stderr,
		Env: []string{"PATH=" + os.Getenv("PATH"), "LANG=C"},
	})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 7 {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stderr.String(), "(exit 7)") {
		t.Fatalf("footer missing exit 7:\n%s", stderr.String())
	}
}

func writeShell(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "bash")
	script := "#!/bin/sh\n" + body
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
