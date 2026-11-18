package picker

import (
	"bytes"
	"io"
	"strings"
	"testing"

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
	got := FormatMenu(items, 1, 0)
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
	seq, rest := splitEscape([]byte("[B\r"))
	if string(seq) != "[B" || string(rest) != "\r" {
		t.Fatalf("split = %q %q", seq, rest)
	}
	seq, rest = splitEscape([]byte("OA"))
	if string(seq) != "OA" || len(rest) != 0 {
		t.Fatalf("ss3 = %q %q", seq, rest)
	}
	seq, rest = splitEscape([]byte("[1;5Axxx"))
	if string(seq) != "[1;5A" || string(rest) != "xxx" {
		t.Fatalf("csi = %q %q", seq, rest)
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

func TestWriteFrameUsesCRLF(t *testing.T) {
	var buf bytes.Buffer
	items := []Item{{Label: "auth-refactor", Detail: "1 repository"}}
	if err := writeFrame(&buf, items, 0, 80); err != nil {
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
