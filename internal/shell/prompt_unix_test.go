//go:build unix

package shell

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestPromptBashSourcesRCAndSurvivesClear(t *testing.T) {
	bash := requireShell(t, "bash")
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bashrc"), "PS1='$ '\nexport MYWT_RC_MARKER=sourced\nunset PROMPT_COMMAND\n")
	writeFile(t, filepath.Join(home, ".hushlogin"), "")
	master, done := startTaggedShell(t, bash, home, "auth-refactor", nil, false)
	buf := waitPty(t, master, "(mywt:auth-refactor) $ ", 8*time.Second)
	mustWrite(t, master, "printf 'MARKER:%s\\n' \"$MYWT_RC_MARKER\"\n")
	buf = append(buf, waitPty(t, master, "MARKER:sourced", 5*time.Second)...)
	mustWrite(t, master, "clear\ntrue\n")
	buf = append(buf, waitPty(t, master, "true", 5*time.Second)...)
	buf = append(buf, readPtyUntil(t, master, "(mywt:auth-refactor) $ ", 5*time.Second)...)
	mustWrite(t, master, "\x0cecho AFTER_CTRL_L\nexit\n")
	buf = append(buf, waitPty(t, master, "AFTER_CTRL_L", 5*time.Second)...)
	waitLaunch(t, done, buf)
	if !bytes.Contains(buf, []byte("(mywt:auth-refactor) $ ")) {
		t.Fatalf("tag missing after clear/ctrl-l:\n%s", buf)
	}
}

func TestPromptBashPROMPTCommand(t *testing.T) {
	bash := requireShell(t, "bash")
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bashrc"), "PS1='$ '\nPROMPT_COMMAND='PS1=\"star \\$ \"'\n")
	writeFile(t, filepath.Join(home, ".hushlogin"), "")
	master, done := startTaggedShell(t, bash, home, "auth-refactor", nil, false)
	buf := waitPty(t, master, "(mywt:auth-refactor) star $ ", 8*time.Second)
	var cmds strings.Builder
	for i := 0; i < 10; i++ {
		cmds.WriteString("true\n")
	}
	cmds.WriteString("exit\n")
	mustWrite(t, master, cmds.String())
	waitLaunch(t, done, nil)
	_ = master.SetReadDeadline(time.Now().Add(2 * time.Second))
	rest := readPtyRest(t, master)
	buf = append(buf, rest...)
	if n := bytes.Count(buf, []byte("(mywt:auth-refactor) star $ ")); n < 10 {
		t.Fatalf("tag survived %d prompts, want at least 10\n%s", n, buf)
	}
}

func TestPromptZshSurvivesClear(t *testing.T) {
	zsh := requireShell(t, "zsh")
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".zshrc"), "PROMPT='%% '\nunset RPROMPT\n")
	master, done := startTaggedShell(t, zsh, home, "auth-refactor", nil, false)
	buf := waitPty(t, master, "(mywt:auth-refactor) % ", 8*time.Second)
	mustWrite(t, master, "clear\ntrue\nexit\n")
	buf = append(buf, waitPty(t, master, "true", 5*time.Second)...)
	waitLaunch(t, done, buf)
	rest := readPtyRest(t, master)
	buf = append(buf, rest...)
	if !bytes.Contains(buf, []byte("(mywt:auth-refactor) % ")) {
		t.Fatalf("zsh tag missing:\n%s", buf)
	}
}

func TestPromptZshHonorsUserZDOTDIR(t *testing.T) {
	zsh := requireShell(t, "zsh")
	home := t.TempDir()
	alt := filepath.Join(home, "alt")
	if err := os.MkdirAll(alt, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(home, ".zshenv"), "ZDOTDIR=\"$HOME/alt\"\n")
	writeFile(t, filepath.Join(alt, ".zshrc"), "PROMPT='%% '\nexport ZSH_MARKER=alt\n")
	master, done := startTaggedShell(t, zsh, home, "auth-refactor", nil, false)
	_ = waitPty(t, master, "(mywt:auth-refactor) % ", 8*time.Second)
	mustWrite(t, master, "printf 'ZDOT:%s MARKER:%s\\n' \"$ZDOTDIR\" \"$ZSH_MARKER\"\nexit\n")
	buf := waitPty(t, master, "MARKER:alt", 5*time.Second)
	waitLaunch(t, done, buf)
	if !bytes.Contains(buf, []byte("ZDOT:"+alt+" ")) {
		t.Fatalf("ZDOTDIR was not the user's directory:\n%s", buf)
	}
	if bytes.Contains(buf, []byte("mywt-prompt-")) && bytes.Contains(buf, []byte("ZDOT:")) {
		line := lineWith(buf, "ZDOT:")
		if strings.Contains(line, "mywt-prompt-") {
			t.Fatalf("session ZDOTDIR is the shim dir: %s", line)
		}
	}
}

func TestPromptFishSurvivesClear(t *testing.T) {
	if os.Getenv("MYWT_FISH_CHILD") == "1" {
		err := Launch(Options{
			Shell:     os.Getenv("MYWT_FISH_BIN"),
			Dir:       os.Getenv("MYWT_FISH_DIR"),
			Label:     "auth-refactor",
			SessionID: "worktree-1",
			Env:       promptEnv(os.Getenv("MYWT_FISH_HOME"), nil),
		})
		if err != nil {
			os.Stderr.WriteString("launch: " + err.Error() + "\n")
			os.Exit(1)
		}
		os.Exit(0)
	}
	fish := requireShell(t, "fish")
	home := t.TempDir()
	dir := t.TempDir()
	beforeFish := promptDirNames()
	writeFile(t, filepath.Join(home, ".config", "fish", "config.fish"), "function fish_prompt\n    printf 'fish> '\nend\n")
	master, slave := openFramePTY(t, 100)
	cmd := exec.Command(os.Args[0], "-test.run=^TestPromptFishSurvivesClear$")
	cmd.Env = append(os.Environ(),
		"MYWT_FISH_CHILD=1",
		"MYWT_FISH_HOME="+home,
		"MYWT_FISH_DIR="+dir,
		"MYWT_FISH_BIN="+fish,
	)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = slave.Close()
	buf := waitPty(t, master, "(mywt:auth-refactor) fish> ", 8*time.Second)
	if extra := newPromptDirs(beforeFish); len(extra) != 0 {
		t.Fatalf("fish created a shim dir: %v", extra)
	}
	mustWrite(t, master, "clear\ntrue\nexit\n")
	buf = append(buf, waitPty(t, master, "true", 5*time.Second)...)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	waitLaunch(t, done, buf)
	buf = append(buf, readPtyRest(t, master)...)
	if !bytes.Contains(buf, []byte("(mywt:auth-refactor) fish> ")) {
		t.Fatalf("fish tag missing:\n%s", buf)
	}
}

func TestPromptPOSIXEnv(t *testing.T) {
	sh := requireShell(t, "sh")
	home := t.TempDir()
	envFile := filepath.Join(home, "myenv")
	writeFile(t, envFile, "MYWT_ENV_MARKER=from-env\nexport MYWT_ENV_MARKER\nPS1='$ '\n")
	master, done := startTaggedShell(t, sh, home, "auth-refactor", []string{"ENV=" + envFile}, false)
	buf := waitPty(t, master, "(mywt:auth-refactor)", 8*time.Second)
	mustWrite(t, master, "printf 'MARKER:%s ENV:%s\\n' \"$MYWT_ENV_MARKER\" \"$ENV\"\nexit\n")
	buf = append(buf, waitPty(t, master, "MARKER:from-env", 5*time.Second)...)
	waitLaunch(t, done, buf)
	if !bytes.Contains(buf, []byte("ENV:"+envFile)) {
		t.Fatalf("ENV was not restored:\n%s", buf)
	}
}

func TestPromptHostileNameRunsNothing(t *testing.T) {
	bash := requireShell(t, "bash")
	home := t.TempDir()
	dir := t.TempDir()
	writeFile(t, filepath.Join(home, ".bashrc"), "PS1='$ '\n")
	writeFile(t, filepath.Join(home, ".hushlogin"), "")
	master, slave := openFramePTY(t, 80)
	done := make(chan error, 1)
	go func() {
		done <- Launch(Options{
			Shell: bash, Dir: dir, Label: "$(touch pwned)", SessionID: "worktree-1",
			Stdin: slave, Stdout: slave, Stderr: slave,
			Env: promptEnv(home, nil),
		})
	}()
	buf := waitPty(t, master, "(mywt:__touch_pwned_) ", 8*time.Second)
	mustWrite(t, master, "exit\n")
	waitLaunch(t, done, buf)
	for _, path := range []string{filepath.Join(dir, "pwned"), filepath.Join(home, "pwned")} {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("hostile name created %s", path)
		}
	}
}

func TestPromptNestedOnce(t *testing.T) {
	bash := requireShell(t, "bash")
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bashrc"), "PS1=\"${MYWT_PARENT_PROMPT_TAG}\"'\\$ '\n")
	writeFile(t, filepath.Join(home, ".hushlogin"), "")
	master, done := startTaggedShell(t, bash, home, "child", []string{
		"MYWT_SESSION=parent",
		"MYWT_PROMPT_TAG=(mywt:parent) ",
	}, false)
	buf := waitPty(t, master, "(mywt:parent>child) ", 8*time.Second)
	mustWrite(t, master, "printf 'PS1:%s\\n' \"$PS1\"\nexit\n")
	buf = append(buf, waitPty(t, master, "PS1:", 5*time.Second)...)
	waitLaunch(t, done, buf)
	if bytes.Contains(buf, []byte("(mywt:parent) (mywt:parent>child)")) {
		t.Fatalf("nested prompt has two tags:\n%s", buf)
	}
	if !bytes.Contains(buf, []byte("PS1:(mywt:parent>child) $ ")) && !bytes.Contains(buf, []byte("PS1:(mywt:parent>child) \\$ ")) {
		line := lineWith(buf, "PS1:")
		if strings.Count(line, "(mywt:") != 1 {
			t.Fatalf("nested PS1 = %q\n%s", line, buf)
		}
	}
}

func TestPromptExitCodeOnTTY(t *testing.T) {
	bash := requireShell(t, "bash")
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bashrc"), "PS1='$ '\n")
	writeFile(t, filepath.Join(home, ".hushlogin"), "")
	master, done := startTaggedShell(t, bash, home, "auth-refactor", nil, false)
	_ = waitPty(t, master, "(mywt:auth-refactor) $ ", 8*time.Second)
	mustWrite(t, master, "exit 3\n")
	var err error
	select {
	case err = <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for exit 3")
	}
	buf := readPtyRest(t, master)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 3 {
		t.Fatalf("err = %v\n%s", err, buf)
	}
	if !bytes.Contains(buf, []byte("(exit 3)")) {
		t.Fatalf("footer missing:\n%s", buf)
	}
}

func TestPromptNoPromptTagKeepsFrame(t *testing.T) {
	bash := requireShell(t, "bash")
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bashrc"), "PS1='$ '\nexport MYWT_RC_MARKER=sourced\n")
	writeFile(t, filepath.Join(home, ".hushlogin"), "")
	master, done := startTaggedShell(t, bash, home, "auth-refactor", nil, true)
	buf := waitPty(t, master, "mywt: auth-refactor", 8*time.Second)
	mustWrite(t, master, "printf 'MARKER:%s\\n' \"$MYWT_RC_MARKER\"\nexit\n")
	buf = append(buf, waitPty(t, master, "MARKER:sourced", 5*time.Second)...)
	waitLaunch(t, done, buf)
	buf = append(buf, readPtyRest(t, master)...)
	if bytes.Contains(buf, []byte("(mywt:")) {
		t.Fatalf("opt-out still tagged the prompt:\n%s", buf)
	}
	if !bytes.Contains(buf, []byte("Type exit to return to")) && !bytes.Contains(buf, []byte("╭")) {
		t.Fatalf("opt-out dropped the frame:\n%s", buf)
	}
}

func requireShell(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skip(name + " is not installed")
	}
	return path
}

func startTaggedShell(t *testing.T, shellPath, home, label string, extra []string, noTag bool) (*os.File, chan error) {
	t.Helper()
	master, slave := openFramePTY(t, 100)
	done := make(chan error, 1)
	go func() {
		done <- Launch(Options{
			Shell: shellPath, Dir: t.TempDir(), Label: label, SessionID: "worktree-1",
			Stdin: slave, Stdout: slave, Stderr: slave,
			Env:         promptEnv(home, extra),
			NoPromptTag: noTag,
		})
	}()
	return master, done
}

func promptEnv(home string, extra []string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"TERM=xterm-256color",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}
	return append(env, extra...)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, master *os.File, s string) {
	t.Helper()
	if _, err := master.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
}

func waitLaunch(t *testing.T, done chan error, buf []byte) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("launch: %v\n%s", err, buf)
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("shell did not exit\n%s", buf)
	}
}

func readPtyRest(t *testing.T, master *os.File) []byte {
	t.Helper()
	var buf bytes.Buffer
	tmp := make([]byte, 1024)
	quiet := 0
	for quiet < 4 {
		pfd := []unix.PollFd{{Fd: int32(master.Fd()), Events: unix.POLLIN}}
		n, err := unix.Poll(pfd, 50)
		if err != nil || n == 0 {
			quiet++
			continue
		}
		nread, err := master.Read(tmp)
		if nread > 0 {
			buf.Write(tmp[:nread])
			quiet = 0
		}
		if err != nil || nread == 0 {
			break
		}
	}
	return buf.Bytes()
}

func lineWith(buf []byte, needle string) string {
	for _, line := range strings.Split(string(buf), "\n") {
		if strings.Contains(line, needle) {
			return strings.TrimRight(line, "\r")
		}
	}
	return ""
}
