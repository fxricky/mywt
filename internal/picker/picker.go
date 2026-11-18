package picker

import (
	"fmt"
	"io"
	"strings"

	"github.com/fxricky/mywt/internal/session"
)

// Key is a selector input.
type Key int

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyCancel
)

// Item is one row in the session selector.
type Item struct {
	Label  string
	Detail string
	ID     string
}

// ItemsFromSessions builds selector rows. A session with a name is shown by
// that name; an unnamed session is shown by its id (which includes the
// timestamp). Duplicate names are disambiguated with the id. Detail is the
// repository count.
func ItemsFromSessions(sessions []*session.Session) []Item {
	labels := make([]string, len(sessions))
	counts := make(map[string]int, len(sessions))
	for i, s := range sessions {
		labels[i] = sessionLabel(s)
		counts[labels[i]]++
	}
	items := make([]Item, len(sessions))
	for i, s := range sessions {
		label := labels[i]
		if strings.TrimSpace(s.Name) != "" && counts[label] > 1 {
			label = fmt.Sprintf("%s (%s)", s.Name, s.ID)
		}
		items[i] = Item{
			Label:  label,
			Detail: repoDetail(len(s.Projects)),
			ID:     s.ID,
		}
	}
	return items
}

func sessionLabel(s *session.Session) string {
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	return s.ID
}

func repoDetail(n int) string {
	if n == 1 {
		return "1 repository"
	}
	return fmt.Sprintf("%d repositories", n)
}

// FormatMenu renders the selector as plain text. width <= 0 disables trimming.
// The selected row starts with "> ".
func FormatMenu(items []Item, selected, width int) string {
	maxLabel := 0
	for _, it := range items {
		if l := len(it.Label); l > maxLabel {
			maxLabel = l
		}
	}
	var b strings.Builder
	b.WriteString("Select a session\n")
	b.WriteString("Enter opens the session root    ↑/↓ move    Esc or Ctrl+C cancels\n\n")
	for i, it := range items {
		marker := "  "
		if i == selected {
			marker = "> "
		}
		line := fmt.Sprintf("%s%-*s  %s", marker, maxLabel, it.Label, it.Detail)
		b.WriteString(trimWidth(line, width))
		b.WriteByte('\n')
	}
	return b.String()
}

func trimWidth(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width < 4 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

// DecodeKey maps one key or escape sequence to a selector action.
func DecodeKey(seq []byte) Key {
	if len(seq) == 0 {
		return KeyNone
	}
	if seq[0] != 0x1b {
		switch seq[0] {
		case 3, 4: // Ctrl+C, Ctrl+D
			return KeyCancel
		case '\r', '\n':
			return KeyEnter
		default:
			return KeyNone
		}
	}
	if len(seq) == 1 {
		return KeyCancel
	}
	switch seq[1] {
	case '[', 'O':
		switch seq[len(seq)-1] {
		case 'A':
			return KeyUp
		case 'B':
			return KeyDown
		}
	}
	return KeyNone
}

// splitEscape splits one escape body (without the leading ESC) from any
// following bytes. A CSI sequence runs through its final byte (0x40-0x7E);
// an SS3 sequence (ESC O x) is two bytes. Anything else is left in rest so
// the ESC can be handled on its own.
func splitEscape(buf []byte) (seq, rest []byte) {
	if len(buf) == 0 {
		return nil, nil
	}
	switch buf[0] {
	case '[':
		for i := 1; i < len(buf); i++ {
			if buf[i] >= 0x40 && buf[i] <= 0x7e {
				return buf[:i+1], buf[i+1:]
			}
		}
		return buf, nil
	case 'O':
		if len(buf) >= 2 {
			return buf[:2], buf[2:]
		}
		return buf, nil
	default:
		return nil, buf
	}
}

func writeFrame(out io.Writer, items []Item, selected, width int) error {
	text := FormatMenu(items, selected, width)
	text = strings.ReplaceAll(text, "\n", "\r\n")
	lines := strings.Split(strings.TrimSuffix(text, "\r\n"), "\r\n")
	var b strings.Builder
	b.WriteString("\033[H\033[2J")
	for _, line := range lines {
		if strings.HasPrefix(line, "> ") {
			line = "\033[7m" + line + "\033[0m"
		}
		b.WriteString(line)
		b.WriteString("\r\n")
	}
	_, err := io.WriteString(out, b.String())
	return err
}

// selectIndex runs the selector state machine. onDraw, when set, is called
// before each read and may abort the loop.
func selectIndex(next func() (Key, error), n int, onDraw func(selected int) error) (int, error) {
	if n <= 0 {
		return -1, ErrNoSessions
	}
	selected := 0
	for {
		if onDraw != nil {
			if err := onDraw(selected); err != nil {
				return -1, err
			}
		}
		k, err := next()
		if err == io.EOF {
			return -1, ErrCancelled
		}
		if err != nil {
			return -1, err
		}
		switch k {
		case KeyUp:
			if selected > 0 {
				selected--
			}
		case KeyDown:
			if selected+1 < n {
				selected++
			}
		case KeyEnter:
			return selected, nil
		case KeyCancel:
			return -1, ErrCancelled
		}
	}
}
