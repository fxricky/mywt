package picker

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fxricky/mywt/internal/session"
)

func TestItemsFromSessions(t *testing.T) {
	items := ItemsFromSessions([]*session.Session{
		{ID: "worktree-20260730123456", Name: "auth-refactor", Projects: []session.Project{{}, {}, {}}},
		{ID: "worktree-20260730123457", Projects: []session.Project{{}}},
		{ID: "worktree-20260730123458", Name: "same"},
		{ID: "worktree-20260730123459", Name: "same", Projects: []session.Project{{}, {}}},
	})
	if len(items) != 4 {
		t.Fatalf("len = %d", len(items))
	}
	if items[0].Label != "auth-refactor" || items[0].Detail != "3 repositories" {
		t.Fatalf("named = %+v", items[0])
	}
	if items[1].Label != "worktree-20260730123457" || items[1].Detail != "1 repository" {
		t.Fatalf("unnamed = %+v", items[1])
	}
	if items[2].Label != "same (worktree-20260730123458)" || items[2].Detail != "0 repositories" {
		t.Fatalf("dup 0 = %+v", items[2])
	}
	if items[3].Label != "same (worktree-20260730123459)" || items[3].Detail != "2 repositories" {
		t.Fatalf("dup 1 = %+v", items[3])
	}
}

func TestFormatMenu(t *testing.T) {
	items := []Item{
		{Label: "auth-refactor", Detail: "3 repositories"},
		{Label: "payment-hotfix", Detail: "2 repositories"},
	}
	got := FormatMenu(items, 1, 0, 0)
	if !strings.Contains(got, "Select a session") {
		t.Fatalf("missing header:\n%s", got)
	}
	if !strings.Contains(got, "Esc or Ctrl+C cancels") {
		t.Fatalf("missing cancel hint:\n%s", got)
	}
	if !strings.Contains(got, "> payment-hotfix") {
		t.Fatalf("missing selection:\n%s", got)
	}
	if strings.Contains(got, "> auth-refactor") {
		t.Fatalf("first row should not be selected:\n%s", got)
	}
	if !strings.Contains(got, "  auth-refactor") || !strings.Contains(got, "3 repositories") {
		t.Fatalf("missing first row:\n%s", got)
	}
}

func TestDecodeKey(t *testing.T) {
	cases := []struct {
		in   []byte
		want Key
	}{
		{[]byte{3}, KeyCancel},
		{[]byte{4}, KeyCancel},
		{[]byte{'\r'}, KeyEnter},
		{[]byte{'\n'}, KeyEnter},
		{[]byte{0x1b}, KeyCancel},
		{[]byte("\x1b[A"), KeyUp},
		{[]byte("\x1b[B"), KeyDown},
		{[]byte("\x1bOA"), KeyUp},
		{[]byte("\x1bOB"), KeyDown},
		{[]byte("\x1b[1;5A"), KeyUp},
		{[]byte("x"), KeyNone},
	}
	for _, tc := range cases {
		if got := DecodeKey(tc.in); got != tc.want {
			t.Errorf("DecodeKey(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSplitEscapeLeavesFollowingKey(t *testing.T) {
	seq, rest, complete := splitEscape([]byte("[B\r"))
	if !complete || string(seq) != "[B" || string(rest) != "\r" {
		t.Fatalf("split = %q %q complete=%v", seq, rest, complete)
	}
	seq, rest, complete = splitEscape([]byte("OA"))
	if !complete || string(seq) != "OA" || len(rest) != 0 {
		t.Fatalf("ss3 = %q %q complete=%v", seq, rest, complete)
	}
	seq, rest, complete = splitEscape([]byte("[1;5Axxx"))
	if !complete || string(seq) != "[1;5A" || string(rest) != "xxx" {
		t.Fatalf("csi = %q %q complete=%v", seq, rest, complete)
	}
	seq, rest, complete = splitEscape([]byte("["))
	if complete || seq != nil || string(rest) != "[" {
		t.Fatalf("incomplete CSI = %q %q complete=%v", seq, rest, complete)
	}
	seq, rest, complete = splitEscape([]byte("O"))
	if complete || seq != nil || string(rest) != "O" {
		t.Fatalf("incomplete SS3 = %q %q complete=%v", seq, rest, complete)
	}
}

func TestFormatMenuStaysInsideTerminal(t *testing.T) {
	items := make([]Item, 30)
	for i := range items {
		items[i] = Item{Label: fmt.Sprintf("session-%02d", i), Detail: "1 repository"}
	}

	top := FormatMenu(items, 0, 80, 24)
	if !strings.Contains(top, "> session-00") {
		t.Fatalf("selected row scrolled off:\n%s", top)
	}
	if strings.Contains(top, "session-29") || strings.Contains(top, "session-21") {
		t.Fatalf("window included rows below the terminal:\n%s", top)
	}
	if lines := strings.Count(top, "\n"); lines > 24 {
		t.Fatalf("menu is %d lines", lines)
	}

	bottom := FormatMenu(items, 29, 80, 24)
	if !strings.Contains(bottom, "> session-29") {
		t.Fatalf("last row not visible:\n%s", bottom)
	}
	if strings.Contains(bottom, "session-08") {
		t.Fatalf("window included rows above the terminal:\n%s", bottom)
	}
}

func TestSelectIndex(t *testing.T) {
	t.Run("down then enter", func(t *testing.T) {
		keys := []Key{KeyDown, KeyEnter}
		got, err := script(keys, 3)
		if err != nil {
			t.Fatal(err)
		}
		if got != 1 {
			t.Fatalf("index = %d, want 1", got)
		}
	})

	t.Run("does not wrap", func(t *testing.T) {
		keys := []Key{KeyUp, KeyDown, KeyDown, KeyDown, KeyEnter}
		got, err := script(keys, 2)
		if err != nil {
			t.Fatal(err)
		}
		if got != 1 {
			t.Fatalf("index = %d, want 1", got)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		_, err := script([]Key{KeyCancel}, 2)
		if err != ErrCancelled {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("eof cancels", func(t *testing.T) {
		_, err := selectIndex(func() (Key, error) { return KeyNone, io.EOF }, 2, nil)
		if err != ErrCancelled {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		_, err := selectIndex(func() (Key, error) { return KeyEnter, nil }, 0, nil)
		if err != ErrNoSessions {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("draws before each key", func(t *testing.T) {
		var seen []int
		keys := []Key{KeyDown, KeyEnter}
		i := 0
		_, err := selectIndex(func() (Key, error) {
			if i >= len(keys) {
				return KeyNone, io.EOF
			}
			k := keys[i]
			i++
			return k, nil
		}, 2, func(selected int) error {
			seen = append(seen, selected)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Trim(strings.Join(ints(seen), ","), "") != "0,1" {
			t.Fatalf("draws = %v", seen)
		}
	})
}

func script(keys []Key, n int) (int, error) {
	i := 0
	return selectIndex(func() (Key, error) {
		if i >= len(keys) {
			return KeyNone, io.EOF
		}
		k := keys[i]
		i++
		return k, nil
	}, n, nil)
}

func ints(v []int) []string {
	out := make([]string, len(v))
	for i, n := range v {
		out[i] = string(rune('0' + n))
	}
	return out
}

func TestFormatMenuStripsControlBytes(t *testing.T) {
	label := "evil" + "\x1b]99;PWNED\x07\r\n\tX" + "\u007f\u009b"
	got := FormatMenu([]Item{{Label: label, Detail: "detail\x1b"}}, 0, 0, 0)
	if !utf8.ValidString(got) {
		t.Fatalf("menu is not valid UTF-8: %q", got)
	}
	var row string
	for _, line := range strings.Split(got, "\n") {
		for i := 0; i < len(line); i++ {
			if line[i] < 0x20 || line[i] == 0x7f {
				t.Fatalf("control byte %#x in %q\nmenu: %q", line[i], line, got)
			}
		}
		for _, r := range line {
			if r >= 0x80 && r <= 0x9f {
				t.Fatalf("C1 control %#U in %q", r, line)
			}
		}
		if strings.Contains(line, "evil") {
			row = line
		}
	}
	if row == "" || !strings.Contains(row, "]99;PWNED") || !strings.Contains(row, "X") {
		t.Fatalf("sanitized row = %q\nmenu: %q", row, got)
	}
	if strings.Contains(got, "\x1b") || strings.Contains(got, "\x07") {
		t.Fatalf("escape payload leaked: %q", got)
	}
}

func TestTrimWidthKeepsCJKRunes(t *testing.T) {
	name := strings.Repeat("测", 40)
	items := []Item{{Label: name, Detail: "1 repository"}}
	for _, width := range []int{1, 2, 3, 4, 5, 8, 10, 39, 40, 41, 80} {
		got := FormatMenu(items, 0, width, 24)
		if !utf8.ValidString(got) || splitHan(got) {
			t.Fatalf("menu width %d split a rune: %q", width, got)
		}
		trimmed := trimWidth(name, width)
		if !utf8.ValidString(trimmed) || splitHan(trimmed) {
			t.Fatalf("trimWidth %d split a rune: %q", width, trimmed)
		}
	}
}

// splitHan reports a truncated UTF-8 sequence for 测 (e6 b5 8b).
func splitHan(s string) bool {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] != 0xe6 {
			continue
		}
		if i+2 >= len(b) || b[i+1] != 0xb5 || b[i+2] != 0x8b {
			return true
		}
		i += 2
	}
	return false
}

func TestWriteFrameUsesCRLF(t *testing.T) {
	var buf bytes.Buffer
	items := []Item{{Label: "auth-refactor", Detail: "1 repository"}}
	if err := writeFrame(&buf, items, 0, 80, 0); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "\033[H\033[2J") {
		t.Fatalf("missing clear: %q", got)
	}
	if !strings.Contains(got, "> auth-refactor") || !strings.Contains(got, "\r\n") {
		t.Fatalf("frame = %q", got)
	}
	if strings.Contains(got, "\n") && !strings.Contains(got, "\r\n") {
		t.Fatalf("bare newline in raw frame: %q", got)
	}
}
