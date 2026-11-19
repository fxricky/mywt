package shell

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Frame is the text shown around a picker subshell. Rendering never starts
// a process; Launch decides when to print it.
type Frame struct {
	Label    string
	Root     string
	ReturnTo string
	// Nested is the MYWT_SESSION value already set in the parent, if any.
	Nested string
}

// Style controls one rendering of a frame.
type Style struct {
	// Unicode selects box-drawing characters. False selects ASCII.
	Unicode bool
	// Color allows SGR styling. False (NO_COLOR) emits no styling escapes.
	Color bool
	// Width is the box width in columns, already clamped.
	Width int
}

const (
	minFrameWidth = 40
	maxFrameWidth = 80
)

// ClampWidth limits a terminal width to the frame's column range.
func ClampWidth(cols int) int {
	if cols < minFrameWidth {
		return minFrameWidth
	}
	if cols > maxFrameWidth {
		return maxFrameWidth
	}
	return cols
}

// NestedWarning is the stderr line printed when a session is started from
// inside another mywt session.
func NestedWarning(parent string) string {
	return "mywt: warning: already in session " + parent + "; continuing nested\n"
}

// RenderPlain is the banner used when stderr is not a terminal: the same
// facts as the box, with no border and no styling.
func RenderPlain(f Frame) string {
	return frameTitle(f) + "\n" + f.Root + "\n" + exitInstruction(f.ReturnTo) + "\n\n"
}

// RenderBox draws the entry frame. Every line has Style.Width columns,
// ignoring SGR sequences, so the box does not wrap.
func RenderBox(f Frame, s Style) string {
	width := s.Width
	if width < 8 {
		width = minFrameWidth
	}
	g := glyphsFor(s.Unicode)
	title := truncate(frameTitle(f), width-6, s.Unicode)
	path := fitField(f.Root, width-4, s.Unicode)
	back := fitField(exitInstruction(f.ReturnTo), width-4, s.Unicode)
	lines := []string{
		topBorder(title, width, g, s.Color),
		row(path, width, g),
		row(back, width, g),
		g.bl + strings.Repeat(g.h, width-2) + g.br,
	}
	return strings.Join(lines, "\n") + "\n"
}

// RenderFooter is the line printed after the subshell exits.
// Unicode: ╰─ left mywt: <name> (exit N) · back in <cwd>
// ASCII:   +-- left mywt: <name> (exit N) - back in <cwd>
func RenderFooter(f Frame, exitCode int, unicode bool, width int) string {
	head := "╰─"
	sep := "·"
	if !unicode {
		head = "+--"
		sep = "-"
	}
	prefix := head + " left mywt: "
	middle := fmt.Sprintf(" (exit %d) %s back in ", exitCode, sep)
	line := prefix + f.Label + middle + f.ReturnTo
	if width > 0 && utf8.RuneCountInString(line) > width {
		budget := width - utf8.RuneCountInString(prefix) - utf8.RuneCountInString(middle)
		if budget < 2 {
			line = truncate(line, width, unicode)
		} else {
			nameMax := budget / 2
			if utf8.RuneCountInString(f.Label) < nameMax {
				nameMax = utf8.RuneCountInString(f.Label)
			}
			cwdMax := budget - nameMax
			if utf8.RuneCountInString(f.ReturnTo) < cwdMax {
				cwdMax = utf8.RuneCountInString(f.ReturnTo)
				nameMax = budget - cwdMax
				if nameMax > utf8.RuneCountInString(f.Label) {
					nameMax = utf8.RuneCountInString(f.Label)
				}
				cwdMax = budget - nameMax
			}
			line = prefix + truncate(f.Label, nameMax, unicode) + middle + truncate(f.ReturnTo, cwdMax, unicode)
		}
	}
	return line + "\n"
}

// TitleSaveAndSet saves the window title (CSI 22;0t) and sets mywt's title.
func TitleSaveAndSet(label string) string {
	return "\033[22;0t\033]0;mywt: " + label + "\a"
}

// TitleRestore restores the window title saved by TitleSaveAndSet (CSI 23;0t).
func TitleRestore() string {
	return "\033[23;0t"
}

func frameTitle(f Frame) string {
	title := "mywt: " + f.Label
	if f.Nested != "" {
		title += " (nested in " + f.Nested + ")"
	}
	return title
}

func exitInstruction(cwd string) string {
	return "Type exit to return to " + cwd
}

type glyphs struct {
	tl, tr, bl, br, h, v string
}

func glyphsFor(unicode bool) glyphs {
	if unicode {
		return glyphs{tl: "╭", tr: "╮", bl: "╰", br: "╯", h: "─", v: "│"}
	}
	return glyphs{tl: "+", tr: "+", bl: "+", br: "+", h: "-", v: "|"}
}

func topBorder(title string, width int, g glyphs, color bool) string {
	shown := title
	painted := shown
	if color {
		painted = "\033[1m" + shown + "\033[0m"
	}
	dashes := width - 3 - utf8.RuneCountInString(shown) - 2
	if dashes < 1 {
		dashes = 1
	}
	return g.tl + g.h + " " + painted + " " + strings.Repeat(g.h, dashes) + g.tr
}

func row(text string, width int, g glyphs) string {
	inner := width - 4
	pad := inner - utf8.RuneCountInString(text)
	if pad < 0 {
		pad = 0
	}
	return g.v + " " + text + strings.Repeat(" ", pad) + " " + g.v
}

func fitField(text string, inner int, unicode bool) string {
	if inner < 1 {
		return truncate(text, 1, unicode)
	}
	prefix := "Type exit to return to "
	if strings.HasPrefix(text, prefix) && utf8.RuneCountInString(prefix) < inner {
		room := inner - utf8.RuneCountInString(prefix)
		return prefix + truncate(strings.TrimPrefix(text, prefix), room, unicode)
	}
	return truncate(text, inner, unicode)
}

func truncate(s string, max int, unicode bool) string {
	if max <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	ell := []rune("...")
	if unicode {
		ell = []rune("…")
	}
	if max <= len(ell) {
		return string(ell[:max])
	}
	return string(rs[:max-len(ell)]) + string(ell)
}
