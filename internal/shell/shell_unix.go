//go:build unix

package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// runInteractive starts cmd and waits for it. When tty is the controlling
// terminal, the child is placed in its own foreground process group so Ctrl+C
// reaches the shell (and the shell's foreground command) instead of mywt.
// Terminal attributes saved before the child starts are restored after it
// exits. This does not allocate a PTY and does not keep a pinned header.
func runInteractive(cmd *exec.Cmd, tty *os.File) error {
	var (
		fd            int
		parentPgid    int
		useForeground bool
		restoreTerm   func()
	)
	if tty != nil && term.IsTerminal(int(tty.Fd())) {
		fd = int(tty.Fd())
		if pgid, err := unix.IoctlGetInt(fd, unix.TIOCGPGRP); err == nil && pgid > 0 {
			parentPgid = pgid
			useForeground = true
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Foreground: true,
				Ctty:       fd,
			}
			// Catch these in the parent so a background ioctl does not stop
			// mywt. Do not use signal.Ignore: an ignored signal is inherited
			// across exec, and an interactive shell would then leave Ctrl+C
			// ignored for itself and its commands. A caught signal is reset
			// to the default disposition when the shell starts.
			sigCh := make(chan os.Signal, 4)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTTIN, syscall.SIGTTOU)
			done := make(chan struct{})
			go func() {
				for {
					select {
					case <-sigCh:
					case <-done:
						return
					}
				}
			}()
			defer close(done)
			defer signal.Stop(sigCh)
			if state, err := term.GetState(fd); err == nil {
				restoreTerm = func() {
					_ = term.Restore(fd, state)
					_, _ = tty.Write([]byte("\033[?25h"))
				}
				defer func() { restoreTerm() }()
			}
		}
	}

	// SIGHUP is what a closed terminal delivers. Catch it so defers still run,
	// forward it to the shell, and exit 129. SIGTERM is not caught: interactive
	// bash and zsh ignore it, so waiting after a forward would hang. SIGINT is
	// handled above and is not part of this path.
	stopHangup := make(chan struct{})
	hangup := make(chan os.Signal, 2)
	signal.Notify(hangup, syscall.SIGHUP)
	defer signal.Stop(hangup)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	var (
		mu     sync.Mutex
		caught syscall.Signal
		wg     sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case sig := <-hangup:
				s, ok := sig.(syscall.Signal)
				if !ok || cmd.Process == nil {
					continue
				}
				mu.Lock()
				if caught == 0 {
					caught = s
				}
				mu.Unlock()
				forwardToChild(cmd, s, useForeground)
			case <-stopHangup:
				return
			}
		}
	}()
	if useForeground {
		defer func() {
			// The child shell was the foreground process group. This process
			// is only a job of the user's shell, so it is in the background
			// once the child exits. TIOCSPGRP from a background job raises
			// SIGTTOU. With that signal caught, the ioctl fails with EINTR
			// and a retry never finishes, which leaves the user's shell
			// without the terminal. POSIX tcsetpgrp allows the call when
			// SIGTTOU is ignored.
			signal.Ignore(syscall.SIGTTOU)
			_ = unix.IoctlSetPointerInt(fd, unix.TIOCSPGRP, parentPgid)
		}()
	}
	err := waitErr(cmd.Wait())
	close(stopHangup)
	wg.Wait()
	mu.Lock()
	sig := caught
	mu.Unlock()
	if sig != 0 {
		return &ExitError{Code: 128 + int(sig)}
	}
	return err
}

// forwardToChild delivers sig to the shell. When the shell has its own
// foreground process group, the whole group is signaled so a command the
// shell is running sees it too.
func forwardToChild(cmd *exec.Cmd, sig syscall.Signal, group bool) {
	if cmd.Process == nil {
		return
	}
	if group {
		if err := syscall.Kill(-cmd.Process.Pid, sig); err == nil {
			return
		}
	}
	_ = cmd.Process.Signal(sig)
}

func waitErr(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return fmt.Errorf("shell: %w", err)
	}
	if code := exitErr.ExitCode(); code >= 0 {
		if code == 0 {
			return nil
		}
		return &ExitError{Code: code}
	}
	if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return &ExitError{Code: 128 + int(ws.Signal())}
	}
	return &ExitError{Code: 1}
}
