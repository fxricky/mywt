//go:build unix

package shell

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLaunchTTYFrameTitleAndWidth(t *testing.T) {
	t.Run("clamps narrow and wide terminals", func(t *testing.T) {
		for _, tc := range []struct {
			cols int
			want int
		}{
			{cols: 20, want: 40},
			{cols: 120, want: 80},
		} {
			out := launchOnPTY(t, tc.cols, []string{"LANG=C.UTF-8"})
			if !strings.Contains(out, "\033[22;0t") || !strings.Contains(out, "\033[23;0t") || !strings.Contains(out, "\033]0;mywt: auth-refactor\a") {
				t.Fatalf("cols %d missing title save/set/restore:\n%q", tc.cols, out)
			}
			width := boxWidth(t, out)
			if width != tc.want {
				t.Fatalf("cols %d box width = %d, want %d\n%s", tc.cols, width, tc.want, visible(out))
			}
			if !strings.Contains(out, "(exit 0)") {
				t.Fatalf("cols %d missing footer:\n%s", tc.cols, visible(out))
			}
		}
	})

	t.Run("NO_COLOR omits styling and keeps the title", func(t *testing.T) {
		out := launchOnPTY(t, 80, []string{"LANG=C.UTF-8", "NO_COLOR=1"})
		if strings.Contains(out, "\033[1m") {
			t.Fatalf("NO_COLOR frame is bold:\n%q", out)
		}
		if !strings.Contains(out, "\033[22;0t") || !strings.Contains(out, "\033[23;0t") {
			t.Fatalf("NO_COLOR dropped the title sequences:\n%q", out)
		}
	})
}

func TestLaunchPipeHasNoTitle(t *testing.T) {
	dir := t.TempDir()
	shellPath := writeShell(t, dir, "exit 0\n")
	var stdout, stderr bytes.Buffer
	err := Launch(Options{
		Shell: shellPath, Dir: dir, Label: "auth-refactor", SessionID: "worktree-1",
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
		Env: []string{"PATH=" + os.Getenv("PATH"), "LANG=C"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "\033") || strings.Contains(stdout.String(), "\033") {
		t.Fatalf("pipe launch wrote a title:\nstderr %q\nstdout %q", stderr.String(), stdout.String())
	}
}

func launchOnPTY(t *testing.T, cols int, extra []string) string {
	t.Helper()
	dir := t.TempDir()
	shellPath := writeShell(t, dir, "exit 0\n")
	master, slave := openFramePTY(t, cols)
	env := append([]string{"PATH=" + os.Getenv("PATH")}, extra...)
	var stdout bytes.Buffer
	err := Launch(Options{
		Shell: shellPath, Dir: dir, Label: "auth-refactor", SessionID: "worktree-1",
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: slave,
		Env: env,
	})
	if err != nil {
		t.Fatal(err)
	}
	return readPTY(t, master)
}

func boxWidth(t *testing.T, raw string) int {
	t.Helper()
	text := visible(raw)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "╭") || strings.HasPrefix(line, "+") {
			return len([]rune(line))
		}
	}
	t.Fatalf("no box in:\n%s", text)
	return 0
}

func visible(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == ';') {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == ']' {
			i += 2
			for i < len(s) && s[i] != 0x07 {
				i++
			}
			if i < len(s) {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func readPTY(t *testing.T, master *os.File) string {
	t.Helper()
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	waited := 0
	for waited < 500 {
		pfd := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(pfd, 50)
		waited += 50
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			if buf.Len() > 0 {
				break
			}
			continue
		}
		nread, err := master.Read(tmp)
		if nread > 0 {
			buf.Write(tmp[:nread])
			waited = 0
		}
		if err != nil {
			break
		}
	}
	return buf.String()
}

func openFramePTY(t *testing.T, cols int) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open ptmx: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	if err := unix.IoctlSetPointerInt(int(m.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(int(m.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	s, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ws := &unix.Winsize{Row: 24, Col: uint16(cols)}
	if err := unix.IoctlSetWinsize(int(s.Fd()), unix.TIOCSWINSZ, ws); err != nil {
		t.Fatal(err)
	}
	return m, s
}
