//go:build unix

package shell

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestPromptShimDirGoneAfterPrompt(t *testing.T) {
	for _, name := range []string{"bash", "zsh", "sh"} {
		t.Run(name, func(t *testing.T) {
			assertShimCreatedAndRemoved(t, name, "auth-refactor", nil, "(mywt:auth-refactor)")
		})
	}
}

func TestPromptNestedShimDirGone(t *testing.T) {
	assertShimCreatedAndRemoved(t, "bash", "child", []string{
		"MYWT_SESSION=parent",
		"MYWT_PROMPT_TAG=(mywt:parent) ",
	}, "(mywt:parent>child)")
}

func TestPromptShimDirAfterSignal(t *testing.T) {
	if os.Getenv("MYWT_SIGNAL_CHILD") != "" {
		runSignalChild()
	}
	bash := requireShell(t, "bash")
	t.Run("hup", func(t *testing.T) {
		signalLeavesNoShimDir(t, bash, syscall.SIGHUP, 129)
	})
	t.Run("kill", func(t *testing.T) {
		signalLeavesNoShimDir(t, bash, syscall.SIGKILL, -1)
	})
}

func TestPromptShimRemovalGuard(t *testing.T) {
	tmp := os.TempDir()
	keep := filepath.Join(tmp, "not-mywt-keep")
	if err := os.MkdirAll(keep, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(keep) })
	outside := filepath.Join(t.TempDir(), "mywt-prompt-outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}

	bash := requireShell(t, "bash")
	good, err := os.MkdirTemp(tmp, "mywt-prompt-")
	if err != nil {
		t.Fatal(err)
	}
	runShim(t, bash, mustShim("shims/bashrc"), good, "")
	if _, err := os.Stat(good); !os.IsNotExist(err) {
		t.Fatalf("bash shim left %s (%v)", good, err)
	}
	runShim(t, bash, mustShim("shims/bashrc"), keep, "")
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("bash shim removed a non-prompt dir: %v", err)
	}
	runShim(t, bash, mustShim("shims/bashrc"), outside, "")
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("bash shim removed a prompt dir outside the temp dir: %v", err)
	}

	if zsh, err := exec.LookPath("zsh"); err == nil {
		good, err = os.MkdirTemp(tmp, "mywt-prompt-")
		if err != nil {
			t.Fatal(err)
		}
		runShim(t, zsh, mustShim("shims/zshrc"), good, "__mywt_tmpdir="+strings.TrimRight(tmp, "/"))
		if _, err := os.Stat(good); !os.IsNotExist(err) {
			t.Fatalf("zsh shim left %s (%v)", good, err)
		}
	}
	if sh, err := exec.LookPath("sh"); err == nil {
		good, err = os.MkdirTemp(tmp, "mywt-prompt-")
		if err != nil {
			t.Fatal(err)
		}
		runShim(t, sh, mustShim("shims/env.sh"), good, "")
		if _, err := os.Stat(good); !os.IsNotExist(err) {
			t.Fatalf("sh shim left %s (%v)", good, err)
		}
	}
}

func assertShimCreatedAndRemoved(t *testing.T, shellName, label string, extra []string, prompt string) {
	t.Helper()
	shellPath := requireShell(t, shellName)
	home := t.TempDir()
	rc := "RC_COUNT=$((${RC_COUNT:-0}+1))\nexport RC_COUNT\n"
	switch shellName {
	case "bash":
		writeFile(t, filepath.Join(home, ".bashrc"), rc+"PS1='$ '\nunset PROMPT_COMMAND\n")
		writeFile(t, filepath.Join(home, ".hushlogin"), "")
	case "zsh":
		writeFile(t, filepath.Join(home, ".zshrc"), rc+"PROMPT='%% '\nunset RPROMPT\n")
	default:
		envFile := filepath.Join(home, "env.sh")
		writeFile(t, envFile, rc+"PS1='$ '\n")
		extra = append(append([]string{}, extra...), "ENV="+envFile)
	}
	watch := watchPromptDirs(t)
	master, done := startTaggedShell(t, shellPath, home, label, extra, false)
	buf := waitPty(t, master, prompt, 8*time.Second)
	mustWrite(t, master, "printf 'COUNTED:%s SHIMDIR:%s\\n' \"$RC_COUNT\" \"${MYWT_SHIM_DIR-}\"\nexit\n")
	buf = append(buf, waitPty(t, master, "COUNTED:", 5*time.Second)...)
	waitLaunch(t, done, buf)
	buf = append(buf, readPtyRest(t, master)...)
	if !bytes.Contains(buf, []byte("COUNTED:1")) {
		t.Fatalf("%s rc was not sourced once:\n%s", shellName, buf)
	}
	if bytes.Contains(buf, []byte("COUNTED:2")) || strings.Contains(lineWith(buf, "COUNTED:"), "SHIMDIR:/") {
		t.Fatalf("%s shim dir still visible or rc ran twice:\n%s", shellName, buf)
	}
	created, deleted := watch.waitMatched(t, 2*time.Second)
	if left := newPromptDirs(nil); len(left) != 0 && shares(left, created) {
		t.Fatalf("%s left shim dirs %v (created %v deleted %v)", shellName, left, created, deleted)
	}
}

func signalLeavesNoShimDir(t *testing.T, shellPath string, sig syscall.Signal, wantExit int) {
	t.Helper()
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".bashrc"), "PS1='$ '\nunset PROMPT_COMMAND\n")
	writeFile(t, filepath.Join(home, ".hushlogin"), "")
	dir := t.TempDir()
	watch := watchPromptDirs(t)
	master, slave := openFramePTY(t, 80)
	cmd := exec.Command(os.Args[0], "-test.run=^TestPromptShimDirAfterSignal$")
	cmd.Env = append(os.Environ(),
		"MYWT_SIGNAL_CHILD=1",
		"MYWT_SIGNAL_SHELL="+shellPath,
		"MYWT_SIGNAL_HOME="+home,
		"MYWT_SIGNAL_DIR="+dir,
	)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = slave.Close()
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	buf := waitPty(t, master, "(mywt:auth) $ ", 8*time.Second)
	watch.waitMatched(t, 2*time.Second)
	if err := syscall.Kill(cmd.Process.Pid, sig); err != nil {
		t.Fatal(err)
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()
	var err error
	select {
	case err = <-waitErr:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("mywt did not exit after %s\n%s", sig, buf)
	}
	if wantExit >= 0 {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != wantExit {
			t.Fatalf("exit = %v, want %d\n%s", err, wantExit, buf)
		}
	} else if err == nil {
		t.Fatalf("SIGKILL did not stop mywt\n%s", buf)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("session dir missing after signal: %v", err)
	}
	created, deleted := watch.snapshot()
	if left := newPromptDirs(nil); shares(left, created) {
		t.Fatalf("shim dirs remain after %s: %v created %v deleted %v", sig, left, created, deleted)
	}
	_ = master.Close()
}

func runSignalChild() {
	err := Launch(Options{
		Shell:     os.Getenv("MYWT_SIGNAL_SHELL"),
		Dir:       os.Getenv("MYWT_SIGNAL_DIR"),
		Label:     "auth",
		SessionID: "worktree-1",
		Env:       promptEnv(os.Getenv("MYWT_SIGNAL_HOME"), nil),
	})
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "launch: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runShim(t *testing.T, shellPath, body, shimDir, extra string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "shim")
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	cmd := exec.Command(shellPath, "-c", ". \"$MYWT_TEST_SHIM\"")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"MYWT_TEST_SHIM=" + file,
		"MYWT_SHIM_DIR=" + shimDir,
		"MYWT_PROMPT_TAG=(mywt:auth) ",
		"TMPDIR=" + os.TempDir(),
	}
	if extra != "" {
		cmd.Env = append(cmd.Env, extra)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shim %s: %v\n%s", shellPath, err, out)
	}
}

func promptDirNames() []string {
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "mywt-prompt-*"))
	return matches
}

func newPromptDirs(before []string) []string {
	have := map[string]bool{}
	for _, name := range before {
		have[name] = true
	}
	var extra []string
	for _, name := range promptDirNames() {
		if !have[name] {
			extra = append(extra, name)
		}
	}
	return extra
}

func shares(left []string, names []string) bool {
	want := map[string]bool{}
	for _, name := range names {
		want[filepath.Base(name)] = true
		want[name] = true
	}
	for _, name := range left {
		if want[name] || want[filepath.Base(name)] {
			return true
		}
	}
	return false
}

type dirWatch struct {
	mu      sync.Mutex
	created []string
	deleted []string
	fd      int
	stop    chan struct{}
	done    chan struct{}
}

func watchPromptDirs(t *testing.T) *dirWatch {
	t.Helper()
	fd, err := unix.InotifyInit1(unix.IN_NONBLOCK | unix.IN_CLOEXEC)
	if err != nil {
		t.Fatalf("inotify: %v", err)
	}
	if _, err := unix.InotifyAddWatch(fd, os.TempDir(), unix.IN_CREATE|unix.IN_DELETE); err != nil {
		_ = unix.Close(fd)
		t.Fatal(err)
	}
	w := &dirWatch{fd: fd, stop: make(chan struct{}), done: make(chan struct{})}
	go w.loop()
	t.Cleanup(func() {
		select {
		case <-w.stop:
		default:
			close(w.stop)
		}
		<-w.done
		_ = unix.Close(fd)
	})
	return w
}

func (w *dirWatch) loop() {
	defer close(w.done)
	buf := make([]byte, 4096)
	for {
		select {
		case <-w.stop:
			return
		default:
		}
		pfd := []unix.PollFd{{Fd: int32(w.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(pfd, 50)
		if err != nil || n == 0 {
			continue
		}
		nread, err := unix.Read(w.fd, buf)
		if nread > 0 {
			w.parse(buf[:nread])
		}
		if err != nil && err != unix.EAGAIN && err != unix.EINTR {
			return
		}
	}
}

func (w *dirWatch) parse(buf []byte) {
	for len(buf) >= unix.SizeofInotifyEvent {
		evLen := binary.LittleEndian.Uint32(buf[12:16])
		if int(unix.SizeofInotifyEvent+evLen) > len(buf) {
			return
		}
		mask := binary.LittleEndian.Uint32(buf[4:8])
		name := ""
		if evLen > 0 {
			raw := buf[unix.SizeofInotifyEvent : unix.SizeofInotifyEvent+evLen]
			name = string(bytes.TrimRight(raw, "\x00"))
		}
		if strings.HasPrefix(name, "mywt-prompt-") {
			w.mu.Lock()
			if mask&unix.IN_CREATE != 0 {
				w.created = append(w.created, name)
			}
			if mask&unix.IN_DELETE != 0 {
				w.deleted = append(w.deleted, name)
			}
			w.mu.Unlock()
		}
		buf = buf[unix.SizeofInotifyEvent+evLen:]
	}
}

func (w *dirWatch) snapshot() (created, deleted []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string{}, w.created...), append([]string{}, w.deleted...)
}

func (w *dirWatch) waitMatched(t *testing.T, d time.Duration) (created, deleted []string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		created, deleted = w.snapshot()
		if promptDirsMatched(created, deleted) {
			return created, deleted
		}
		if time.Now().After(deadline) {
			t.Fatalf("shim dir was not created and removed: created %v deleted %v", created, deleted)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func promptDirsMatched(created, deleted []string) bool {
	if len(created) == 0 {
		return false
	}
	left := map[string]int{}
	for _, name := range deleted {
		left[name]++
	}
	for _, name := range created {
		if left[name] == 0 {
			return false
		}
		left[name]--
	}
	return true
}
