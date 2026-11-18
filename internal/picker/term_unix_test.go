//go:build unix

package picker

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestStreamCombinedArrowAndEnter(t *testing.T) {
	s := &stream{r: bytes.NewReader([]byte("\x1b[B\r")), fd: -1}
	k, err := s.ReadKey()
	if err != nil || k != KeyDown {
		t.Fatalf("first key = %v, %v", k, err)
	}
	k, err = s.ReadKey()
	if err != nil || k != KeyEnter {
		t.Fatalf("second key = %v, %v", k, err)
	}
}

func TestStreamSplitArrowThenEnter(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	errCh := make(chan error, 1)
	go func() {
		if _, err := w.Write([]byte{0x1b, '['}); err != nil {
			errCh <- err
			return
		}
		time.Sleep(15 * time.Millisecond)
		_, err := w.Write([]byte{'B', '\r'})
		errCh <- err
	}()

	s := &stream{r: r, fd: int(r.Fd())}
	k, err := s.ReadKey()
	if err != nil || k != KeyDown {
		t.Fatalf("first key = %v, %v", k, err)
	}
	k, err = s.ReadKey()
	if err != nil || k != KeyEnter {
		t.Fatalf("second key = %v, %v", k, err)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}
}

func TestRunKeepsSelectionOnScreen(t *testing.T) {
	master, slave := openPTY(t)
	items := make([]Item, 30)
	for i := range items {
		items[i] = Item{Label: fmt.Sprintf("session-%02d", i), Detail: "1 repository"}
	}
	done := make(chan struct{})
	go func() {
		_, _ = Run(slave, slave, items)
		close(done)
	}()

	buf, err := readUntil(master, "> session-00", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf, "Select a session") {
		t.Fatalf("missing header: %q", buf)
	}
	if strings.Contains(buf, "session-29") {
		t.Fatalf("last session was drawn outside the terminal:\n%s", buf)
	}
	if _, err := master.Write([]byte{3}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("selector did not return")
	}
}

func TestStreamBareEscCancels(t *testing.T) {
	s := &stream{r: bytes.NewReader([]byte{0x1b}), fd: -1}
	k, err := s.ReadKey()
	if err != nil || k != KeyCancel {
		t.Fatalf("key = %v, %v", k, err)
	}
}

func TestRunSelectsWithArrow(t *testing.T) {
	master, slave := openPTY(t)
	items := []Item{
		{Label: "auth-refactor", Detail: "3 repositories"},
		{Label: "payment-hotfix", Detail: "2 repositories"},
	}
	type result struct {
		idx int
		err error
	}
	ch := make(chan result, 1)
	go func() {
		idx, err := Run(slave, slave, items)
		ch <- result{idx, err}
	}()

	if err := waitFor(master, "Select a session", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := master.Write([]byte("\x1b[B\r")); err != nil {
		t.Fatal(err)
	}
	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatal(res.err)
		}
		if res.idx != 1 {
			t.Fatalf("index = %d, want 1", res.idx)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for selection")
	}
}

func TestRunEscCancels(t *testing.T) {
	master, slave := openPTY(t)
	ch := make(chan error, 1)
	go func() {
		_, err := Run(slave, slave, []Item{{Label: "only", Detail: "1 repository"}})
		ch <- err
	}()
	if err := waitFor(master, "Select a session", 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := master.Write([]byte{0x1b}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-ch:
		if !errors.Is(err, ErrCancelled) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cancel")
	}
}

func openPTY(t *testing.T) (master, slave *os.File) {
	t.Helper()
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open ptmx: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	// TIOCSPTLCK with 0 unlocks the slave, the same operation as unlockpt(3).
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

func waitFor(master *os.File, needle string, d time.Duration) error {
	_, err := readUntil(master, needle, d)
	return err
}

func readUntil(master *os.File, needle string, d time.Duration) (string, error) {
	var buf bytes.Buffer
	deadline := time.Now().Add(d)
	tmp := make([]byte, 256)
	for {
		if strings.Contains(buf.String(), needle) {
			return buf.String(), nil
		}
		if time.Now().After(deadline) {
			return buf.String(), fmt.Errorf("timed out waiting for %q; got %q", needle, buf.String())
		}
		_ = master.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := master.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err == nil {
			continue
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			continue
		}
		if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
			continue
		}
		return buf.String(), err
	}
}
