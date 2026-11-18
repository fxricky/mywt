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

// menuHeaderLines is the title, the key hint, and the blank line under them.
const menuHeaderLines = 3

// FormatMenu renders the selector as plain text. width <= 0 disables trimming.
// height <= 0 shows every row. A positive height keeps the rendered menu,
// including its header, inside that many terminal rows and keeps the selected
// row inside the window. The selected row starts with "> ".
func FormatMenu(items []Item, selected, width, height int) string {
	lines := menuLines(items, selected, width, height)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func menuLines(items []Item, selected, width, height int) []string {
	start, end, header := menuWindow(len(items), selected, height)
	lines := make([]string, 0, menuHeaderLines+end-start)
	if header {
		lines = append(lines,
			"Select a session",
			"Enter opens the session root    ↑/↓ move    Esc or Ctrl+C cancels",
			"",
		)
	}
	maxLabel := 0
	for _, it := range items {
		if l := len(it.Label); l > maxLabel {
			maxLabel = l
		}
	}
	for i := start; i < end; i++ {
		it := items[i]
		marker := "  "
		if i == selected {
			marker = "> "
		}
		line := fmt.Sprintf("%s%-*s  %s", marker, maxLabel, it.Label, it.Detail)
		lines = append(lines, trimWidth(line, width))
	}
	return lines
}

// menuWindow is the half-open item range to draw. header is false only when
// the terminal is too short for the title and a row together; the selected
// row is still included.
func menuWindow(n, selected, height int) (start, end int, header bool) {
	if n <= 0 {
		return 0, 0, true
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= n {
		selected = n - 1
	}
	if height <= 0 {
		return 0, n, true
	}
	header = true
	visible := height - menuHeaderLines
	if visible < 1 {
		header = false
		visible = height
		if visible < 1 {
			visible = 1
		}
	}
	if visible > n {
		visible = n
	}
	start = selected - (visible-1)/2
	if start < 0 {
		start = 0
	}
	if start+visible > n {
		start = n - visible
	}
	if selected < start {
		start = selected
	}
	if selected >= start+visible {
		start = selected - visible + 1
	}
	return start, start + visible, header
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
// an SS3 sequence (ESC O x) is two bytes. complete is false when buf is only
// a prefix of one of those sequences, so the caller can wait for the rest
// instead of treating the fragment as a finished key. Anything else is left
// in rest so the ESC can be handled on its own.
func splitEscape(buf []byte) (seq, rest []byte, complete bool) {
	if len(buf) == 0 {
		return nil, nil, false
	}
	switch buf[0] {
	case '[':
		for i := 1; i < len(buf); i++ {
			if buf[i] >= 0x40 && buf[i] <= 0x7e {
				return buf[:i+1], buf[i+1:], true
			}
			// Parameter bytes are 0x30-0x3F and intermediate bytes are
			// 0x20-0x2F. Anything else is not this CSI sequence.
			if buf[i] < 0x20 || buf[i] > 0x3f {
				return nil, buf, true
			}
		}
		return nil, buf, false
	case 'O':
		if len(buf) >= 2 {
			return buf[:2], buf[2:], true
		}
		return nil, buf, false
	default:
		return nil, buf, true
	}
}

func writeFrame(out io.Writer, items []Item, selected, width, height int) error {
	lines := menuLines(items, selected, width, height)
	var b strings.Builder
	b.WriteString("\033[H\033[2J")
	for i, line := range lines {
		if strings.HasPrefix(line, "> ") {
			line = "\033[7m" + line + "\033[0m"
		}
		b.WriteString(line)
		// A newline on the last row scrolls the alternate screen and pushes
		// the selected row out of view. Stay inside the reported height.
		if height <= 0 || i+1 < height {
			b.WriteString("\r\n")
		}
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
