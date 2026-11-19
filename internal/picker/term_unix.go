//go:build unix

package picker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// ErrCancelled is returned when the user leaves the selector without choosing.
var ErrCancelled = errors.New("cancelled")

// ErrNoSessions is returned when there is nothing to select.
var ErrNoSessions = errors.New("no sessions")

// Run reads the session selector from a terminal. in and out are the
// terminal's stdin and stdout. The terminal is restored, including the
// alternate screen used for the menu, before Run returns.
func Run(in, out *os.File, items []Item) (int, error) {
	if in == nil || out == nil || !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return -1, fmt.Errorf("session selector requires a terminal")
	}
	if len(items) == 0 {
		return -1, ErrNoSessions
	}

	fd := int(in.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return -1, fmt.Errorf("set raw mode: %w", err)
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			_, _ = io.WriteString(out, "\033[?25h\033[?1049l")
			_ = term.Restore(fd, state)
		})
	}
	defer cleanup()

	sigCh := make(chan os.Signal, 1)
	signalNotify(sigCh)
	defer signalStop(sigCh)
	go func() {
		<-sigCh
		cleanup()
		os.Exit(1)
	}()

	_, _ = io.WriteString(out, "\033[?1049h\033[?25l")

	width, height := 80, 24
	if w, h, err := term.GetSize(int(out.Fd())); err == nil {
		if w > 0 {
			width = w
		}
		if h > 0 {
			height = h
		}
	}

	keys := &stream{r: in, fd: fd}
	return selectIndex(keys.ReadKey, len(items), func(selected int) error {
		return writeFrame(out, items, selected, width, height)
	})
}

// stream reads keys from a terminal. fd is used to poll after an ESC so a
// bare Esc is not confused with an arrow sequence; fd < 0 skips the poll.
type stream struct {
	r   io.Reader
	fd  int
	buf []byte
}

func (s *stream) ReadKey() (Key, error) {
	b, err := s.take()
	if err != nil {
		return KeyNone, err
	}
	if b != 0x1b {
		return DecodeKey([]byte{b}), nil
	}
	// Arrow keys often arrive split across reads (ESC, then "[B"). Hold an
	// incomplete CSI/SS3 sequence and wait briefly for its final byte.
	for {
		if len(s.buf) == 0 {
			ok, err := s.readMore()
			if !ok {
				if err != nil && !errors.Is(err, io.EOF) {
					return KeyNone, err
				}
				// Esc won the wait. If the rest of an arrow shows up late,
				// read it now so cooked mode does not echo it after cancel.
				s.discardEscapeTail()
				return KeyCancel, nil
			}
		}
		seq, rest, complete := splitEscape(s.buf)
		if !complete {
			if len(s.buf) >= maxEscapeBody {
				s.buf = nil
				return KeyNone, nil
			}
			ok, err := s.readMore()
			if err != nil && !errors.Is(err, io.EOF) {
				return KeyNone, err
			}
			if !ok {
				// The rest never arrived. Drop the fragment so the next
				// byte is a new key, not the tail of this sequence.
				s.buf = nil
				return KeyNone, nil
			}
			continue
		}
		s.buf = rest
		if len(seq) == 0 {
			return KeyNone, nil
		}
		return DecodeKey(append([]byte{0x1b}, seq...)), nil
	}
}

func (s *stream) take() (byte, error) {
	for len(s.buf) == 0 {
		tmp := make([]byte, 32)
		n, err := s.r.Read(tmp)
		if n > 0 {
			s.buf = append(s.buf, tmp[:n]...)
			break
		}
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}
	b := s.buf[0]
	s.buf = s.buf[1:]
	return b, nil
}

const (
	// escapeWaitMS bounds how long an incomplete escape sequence waits for
	// its final byte. A bare Esc uses the same wait so it is not confused
	// with an arrow key. 100ms covers an ESC, "[", "B" split about 50ms apart.
	escapeWaitMS = 100
	// maxEscapeBody is the longest CSI/SS3 body we will assemble before
	// giving up. Arrow sequences are only a few bytes.
	maxEscapeBody = 32
)

// readMore waits up to escapeWaitMS for more input when fd is pollable,
// then appends one read to the buffer. fd < 0 blocks in Read instead.
func (s *stream) readMore() (bool, error) {
	for {
		if s.fd >= 0 {
			pfd := []unix.PollFd{{Fd: int32(s.fd), Events: unix.POLLIN}}
			n, err := unix.Poll(pfd, escapeWaitMS)
			if err != nil {
				if errors.Is(err, syscall.EINTR) {
					continue
				}
				return false, err
			}
			if n == 0 {
				return false, nil
			}
		}
		tmp := make([]byte, 32)
		n, err := s.r.Read(tmp)
		if n > 0 {
			s.buf = append(s.buf, tmp[:n]...)
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, io.EOF
	}
}

// discardEscapeTail runs after a bare Esc has been classified as cancel.
// A late CSI/SS3 tail is still sitting in the kernel input queue; reading it
// while the terminal is raw drops it instead of leaving it for the shell to
// echo. A tty cannot be peeked, so bytes read during this extra wait are
// discarded even when they are not an escape tail. The wait is one more
// escapeWaitMS, and then at most one further wait if the tail is incomplete.
func (s *stream) discardEscapeTail() {
	if s.fd < 0 {
		return
	}
	ok, err := s.readMore()
	if !ok || err != nil {
		s.buf = nil
		return
	}
	for {
		_, _, complete := splitEscape(s.buf)
		if complete || len(s.buf) >= maxEscapeBody {
			s.buf = nil
			return
		}
		ok, err = s.readMore()
		if !ok || err != nil {
			s.buf = nil
			return
		}
	}
}
