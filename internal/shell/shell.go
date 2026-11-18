package shell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// ExitError is a shell that ran and exited with a non-zero status. Callers
// should propagate Code without wrapping it in an extra "error:" line.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("shell exited with status %d", e.Code)
}

// Options controls Launch.
type Options struct {
	// Shell is the program to run. When empty, $SHELL is used, then a fallback.
	Shell string
	// Dir is the session root. It is resolved to an absolute path and must exist.
	Dir string
	// Label is the session name, or the session id when the session is unnamed.
	Label string
	// SessionID is the worktree-<timestamp> id stored in the environment.
	SessionID string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	// Env replaces the child environment when non-nil. Nil inherits os.Environ().
	Env []string
}

// Launch starts an interactive child shell whose working directory is the
// session root. The parent process's working directory is not changed.
//
// A startup banner names the session. When stdout or stderr is a terminal, the
// window title is set to "mywt: <session>" as well. That title is not a pinned
// header; mywt does not stay in front of the shell to redraw one, and it does
// not allocate a PTY or run tmux. The child is a normal interactive shell, so
// programs such as vim, less, claude, and codex take the terminal as usual.
func Launch(opts Options) error {
	shellPath, err := ResolveShell(opts.Shell)
	if err != nil {
		return err
	}
	dir, err := absDir(opts.Dir)
	if err != nil {
		return err
	}
	label := sanitizeTerminal(opts.Label)
	if label == "" {
		label = sanitizeTerminal(opts.SessionID)
	}
	if label == "" {
		label = "session"
	}

	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	writeTitle(label, stdout, stderr)
	if _, err := io.WriteString(stderr, FormatBanner(label, displayPath(dir))); err != nil {
		return fmt.Errorf("write session banner: %w", err)
	}

	cmd, err := shellCommand(shellPath, dir, sessionEnv(opts.Env, label, dir, opts.SessionID))
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return runInteractive(cmd, stdinFile(stdin))
}

// ResolveShell picks an executable shell. explicit wins when it is usable;
// otherwise $SHELL is used, then /bin/sh, /bin/bash, and /bin/zsh.
func ResolveShell(explicit string) (string, error) {
	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	} else if s := os.Getenv("SHELL"); s != "" {
		candidates = append(candidates, s)
	}
	candidates = append(candidates, "/bin/sh", "/bin/bash", "/bin/zsh")

	seen := map[string]bool{}
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		info, err := os.Stat(c)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			continue
		}
		return c, nil
	}
	return "", fmt.Errorf("no usable shell found; set $SHELL to an interactive shell")
}

// IsInteractive reports whether both files are terminals.
func IsInteractive(in, out *os.File) bool {
	if in == nil || out == nil {
		return false
	}
	return term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

// FormatBanner is the startup banner printed before the child shell.
func FormatBanner(label, prettyPath string) string {
	return fmt.Sprintf(
		"[mywt: %s]  %s\n\nSession root. Repository worktrees are the folders in this directory (this folder is not a git repository).\nclaude and codex can be started here, subject to each tool's own filesystem permissions and trust settings.\nType exit to return to your previous shell and directory.\n\n",
		label,
		prettyPath,
	)
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	prefix := home + string(os.PathSeparator)
	if strings.HasPrefix(path, prefix) {
		return "~" + string(os.PathSeparator) + path[len(prefix):]
	}
	return path
}

func sanitizeTerminal(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

func writeTitle(label string, stdout, stderr io.Writer) {
	title := "\033]0;mywt: " + label + "\a"
	for _, w := range []io.Writer{stdout, stderr} {
		f, ok := w.(*os.File)
		if ok && term.IsTerminal(int(f.Fd())) {
			_, _ = io.WriteString(f, title)
			return
		}
	}
}

func absDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("session directory is empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve session directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("session directory %s does not exist", abs)
		}
		return "", fmt.Errorf("session directory %s: %w", abs, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("session directory %s is not a directory", abs)
	}
	return abs, nil
}

// shellArgs forces interactive mode for shells that need an explicit flag.
// Unknown shells are started with no arguments so a TTY still makes them
// interactive, without passing a flag they may reject. Login mode (-l) is
// never used, because a login shell may change directory.
func shellArgs(shellPath string) []string {
	switch filepath.Base(shellPath) {
	case "bash", "zsh", "sh", "dash", "ksh", "mksh", "ash", "fish":
		return []string{"-i"}
	default:
		return nil
	}
}

func sessionEnv(base []string, label, root, id string) []string {
	if base == nil {
		base = os.Environ()
	}
	vals := map[string]string{
		"MYWT_SESSION":      label,
		"MYWT_SESSION_ROOT": root,
	}
	if id != "" {
		vals["MYWT_SESSION_ID"] = id
	}
	out := make([]string, 0, len(base)+len(vals))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, replaced := vals[key]; replaced {
			continue
		}
		out = append(out, entry)
	}
	// Stable order for the keys we add.
	for _, key := range []string{"MYWT_SESSION", "MYWT_SESSION_ID", "MYWT_SESSION_ROOT"} {
		if v, ok := vals[key]; ok {
			out = append(out, key+"="+v)
		}
	}
	return out
}

func stdinFile(r io.Reader) *os.File {
	f, _ := r.(*os.File)
	return f
}
