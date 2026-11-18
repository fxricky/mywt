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

	width := 80
	if w, _, err := term.GetSize(int(out.Fd())); err == nil && w > 0 {
		width = w
	}

	keys := &stream{r: in, fd: fd}
	return selectIndex(keys.ReadKey, len(items), func(selected int) error {
		return writeFrame(out, items, selected, width)
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
	if len(s.buf) == 0 {
		ok, err := s.fillEscape()
		if !ok {
			if err != nil && !errors.Is(err, io.EOF) {
				return KeyNone, err
			}
			return KeyCancel, nil
		}
	}
	seq, rest := splitEscape(s.buf)
	s.buf = rest
	if len(seq) == 0 {
		return KeyNone, nil
	}
	return DecodeKey(append([]byte{0x1b}, seq...)), nil
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

func (s *stream) fillEscape() (bool, error) {
	if len(s.buf) > 0 {
		return true, nil
	}
	if s.fd >= 0 {
		pfd := []unix.PollFd{{Fd: int32(s.fd), Events: unix.POLLIN}}
		n, err := unix.Poll(pfd, 40)
		if err != nil {
			if errors.Is(err, syscall.EINTR) {
				return s.fillEscape()
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
