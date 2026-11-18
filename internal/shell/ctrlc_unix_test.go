//go:build unix

package shell

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestCtrlCInterruptsForegroundCommand starts mywt's shell on a pty and checks
// that Ctrl+C interrupts a foreground command and returns to the shell.
// SIGINT must not be ignored in the child: an ignored signal survives exec
// and would make sleep ignore Ctrl+C.
func TestCtrlCInterruptsForegroundCommand(t *testing.T) {
	if os.Getenv("MYWT_CTRLC_CHILD") == "1" {
		err := Launch(Options{
			Shell:     "/bin/sh",
			Dir:       os.Getenv("MYWT_CTRLC_DIR"),
			Label:     "auth-refactor",
			SessionID: "worktree-test",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "launch: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	dir := t.TempDir()
	master, slave := openPTY(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestCtrlCInterruptsForegroundCommand$", "-test.v")
	cmd.Env = append(os.Environ(), "MYWT_CTRLC_CHILD=1", "MYWT_CTRLC_DIR="+dir)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = slave.Close()

	buf := waitPty(t, master, "[mywt: auth-refactor]", 4*time.Second)
	if _, err := master.Write([]byte("sleep 30\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := master.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := master.Write([]byte("echo STILL_HERE\nexit\n")); err != nil {
		t.Fatal(err)
	}
	buf = append(buf, readPtyUntil(t, master, "STILL_HERE", 4*time.Second)...)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("child: %v\n%s", err, buf)
		}
	case <-time.After(4 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("shell did not exit after Ctrl+C\n%s", buf)
	}
	if !bytes.Contains(buf, []byte("STILL_HERE")) {
		t.Fatalf("Ctrl+C did not return to the shell\n%s", buf)
	}
}

// TestExitReturnsWhenNotSessionLeader runs Launch the way a user shell does:
// mywt is a foreground job, not the session leader. Restoring the terminal
// after the child exits must return, so the caller can prompt again.
func TestExitReturnsWhenNotSessionLeader(t *testing.T) {
	switch {
	case os.Getenv("MYWT_EXIT_CHILD") == "1":
		err := Launch(Options{
			Shell:     "/bin/sh",
			Dir:       os.Getenv("MYWT_EXIT_DIR"),
			Label:     "auth-refactor",
			SessionID: "worktree-test",
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "launch: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	case os.Getenv("MYWT_EXIT_LEADER") == "1":
		runExitLeader()
	}

	dir := t.TempDir()
	master, slave := openPTY(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestExitReturnsWhenNotSessionLeader$", "-test.v")
	cmd.Env = append(os.Environ(), "MYWT_EXIT_LEADER=1", "MYWT_EXIT_DIR="+dir)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = slave.Close()

	_ = waitPty(t, master, "[mywt: auth-refactor]", 4*time.Second)
	if _, err := master.Write([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("session leader: %v", err)
		}
	case <-time.After(4 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("shell exit did not return the terminal to the calling shell")
	}
}

func runExitLeader() {
	dir := os.Getenv("MYWT_EXIT_DIR")
	env := make([]string, 0, len(os.Environ())+2)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "MYWT_EXIT_") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "MYWT_EXIT_CHILD=1", "MYWT_EXIT_DIR="+dir)
	cmd := exec.Command(os.Args[0], "-test.run=^TestExitReturnsWhenNotSessionLeader$", "-test.v")
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start job: %v\n", err)
		os.Exit(1)
	}
	fd := int(os.Stdin.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, cmd.Process.Pid); err != nil {
		fmt.Fprintf(os.Stderr, "foreground: %v\n", err)
		os.Exit(1)
	}
	err := cmd.Wait()
	signal.Ignore(syscall.SIGTTOU)
	_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, os.Getpid())
	if err != nil {
		fmt.Fprintf(os.Stderr, "job: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func openPTY(t *testing.T) (master, slave *os.File) {
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
	_ = unix.IoctlSetWinsize(int(s.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: 24, Col: 80})
	return m, s
}

func waitPty(t *testing.T, master *os.File, needle string, d time.Duration) []byte {
	t.Helper()
	buf := readPtyUntil(t, master, needle, d)
	return buf
}

func readPtyUntil(t *testing.T, master *os.File, needle string, d time.Duration) []byte {
	t.Helper()
	var buf bytes.Buffer
	deadline := time.Now().Add(d)
	tmp := make([]byte, 256)
	for {
		if bytes.Contains(buf.Bytes(), []byte(needle)) {
			return buf.Bytes()
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q; got %q", needle, buf.String())
		}
		_ = master.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := master.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err == nil || errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		}
		if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
			continue
		}
		t.Fatalf("read pty: %v; got %q", err, buf.String())
	}
}
