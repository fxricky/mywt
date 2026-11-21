package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fxricky/mywt/internal/termtext"
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
	// NoFrame keeps the plain startup banner. MYWT_NO_FRAME=1 does the same.
	NoFrame bool
	// NoPromptTag leaves the shell prompt unchanged. MYWT_NO_PROMPT_TAG=1 does the same.
	NoPromptTag bool
}

// Launch starts an interactive child shell whose working directory is the
// session root. The parent process's working directory is not changed.
//
// On a terminal, stderr gets a box frame and, after the shell exits, a footer
// with the real exit status. The window title is saved and restored. A pipe
// gets a plain banner and the footer, with no box and no title sequences.
// NoFrame, or MYWT_NO_FRAME=1, keeps the original banner and does not print a
// footer. Unless NoPromptTag or MYWT_NO_PROMPT_TAG=1 is set, a known shell
// gets "(mywt:<name>) " in front of the prompt it builds from the user's rc
// files. Those files are only sourced. The shims live in a private temp
// directory. Each shim removes that directory after it has been read, and
// Launch also removes it when it returns, including when the shell fails
// to start.
//
// The title is not a pinned header; mywt does not stay in front of the shell
// to redraw one, and it does not allocate a PTY or run tmux. The child is a
// normal interactive shell, so programs such as vim, less, claude, and codex
// take the terminal as usual.
func Launch(opts Options) error {
	shellPath, err := ResolveShell(opts.Shell)
	if err != nil {
		return err
	}
	dir, err := absDir(opts.Dir)
	if err != nil {
		return err
	}
	label := termtext.Sanitize(opts.Label)
	if label == "" {
		label = termtext.Sanitize(opts.SessionID)
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

	env := opts.Env
	parent, parentSet := lookupEnv(env, "MYWT_SESSION")
	nested := parentSet && parent != ""
	if nested {
		parent = termtext.Sanitize(parent)
		if _, err := io.WriteString(stderr, NestedWarning(parent)); err != nil {
			return fmt.Errorf("write nested warning: %w", err)
		}
	}
	noFrame := opts.NoFrame || envEquals(env, "MYWT_NO_FRAME", "1")
	unicode := utf8Locale(env)
	frame := Frame{
		Label:    label,
		Root:     termtext.Sanitize(displayPath(dir)),
		ReturnTo: termtext.Sanitize(displayPath(workingDir())),
		Nested:   termtext.Sanitize(parent),
	}
	if !nested {
		frame.Nested = ""
	}

	tty := isTerminalWriter(stderr)
	width := ClampWidth(80)
	restoreTitle := func() {}
	switch {
	case noFrame:
		writeTitle(label, stdout, stderr)
		if _, err := io.WriteString(stderr, FormatBanner(label, frame.Root)); err != nil {
			return fmt.Errorf("write session banner: %w", err)
		}
	case !tty:
		if _, err := io.WriteString(stderr, RenderPlain(frame)); err != nil {
			return fmt.Errorf("write session banner: %w", err)
		}
	default:
		width = terminalColumns(stderr)
		if _, err := io.WriteString(stderr, TitleSaveAndSet(label)); err != nil {
			return fmt.Errorf("write session title: %w", err)
		}
		restored := false
		restoreTitle = func() {
			if restored {
				return
			}
			restored = true
			_, _ = io.WriteString(stderr, TitleRestore())
		}
		defer restoreTitle()
		box := RenderBox(frame, Style{Unicode: unicode, Color: colorEnabled(env), Width: width})
		if _, err := io.WriteString(stderr, box); err != nil {
			return fmt.Errorf("write session banner: %w", err)
		}
	}

	childEnv := dropEnvKeys(sessionEnv(env, label, dir, opts.SessionID), "MYWT_PROMPT_TAG", "MYWT_PARENT_PROMPT_TAG", "MYWT_SHIM_DIR")
	args := shellArgs(shellPath)
	tagOn := !opts.NoPromptTag && !envEquals(env, "MYWT_NO_PROMPT_TAG", "1")
	if tagOn && promptFamily(shellPath) != "" {
		shimDir := ""
		if promptFamily(shellPath) != "fish" {
			var err error
			shimDir, err = os.MkdirTemp("", "mywt-prompt-*")
			if err != nil {
				return fmt.Errorf("create prompt dir: %w", err)
			}
			defer os.RemoveAll(shimDir)
		}
		parentName := ""
		if nested {
			parentName = parent
		}
		inheritedTag, _ := lookupEnv(env, "MYWT_PROMPT_TAG")
		plan := planShell(shellPath, shimDir, PromptTag(label, parentName), inheritedTag, env)
		if len(plan.Files) > 0 {
			if err := writePromptFiles(shimDir, plan.Files); err != nil {
				return err
			}
		}
		args = plan.Args
		childEnv = replaceEnv(childEnv, plan.Env)
	}
	cmd, err := shellCommand(shellPath, dir, args, childEnv)
	if err != nil {
		return err
	}
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = runInteractive(cmd, stdinFile(stdin))
	if !noFrame {
		restoreTitle()
		if code, ran := exitStatus(err); ran {
			footer := RenderFooter(frame, code, unicode, width)
			if !tty {
				footer = RenderPlainFooter(frame, code, unicode, width)
			}
			if _, werr := io.WriteString(stderr, footer); werr != nil && err == nil {
				return fmt.Errorf("write session footer: %w", werr)
			}
		}
	}
	return err
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

func writeTitle(label string, stdout, stderr io.Writer) {
	title := "\033]0;mywt: " + termtext.Sanitize(label) + "\a"
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

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil || dir == "" {
		return "."
	}
	return dir
}

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func terminalColumns(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return ClampWidth(80)
	}
	cols, _, err := term.GetSize(int(f.Fd()))
	if err != nil || cols <= 0 {
		return ClampWidth(80)
	}
	return ClampWidth(cols)
}

func exitStatus(err error) (code int, ran bool) {
	if err == nil {
		return 0, true
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code, true
	}
	return 0, false
}

func lookupEnv(env []string, key string) (string, bool) {
	if env == nil {
		return os.LookupEnv(key)
	}
	for i := len(env) - 1; i >= 0; i-- {
		k, v, ok := strings.Cut(env[i], "=")
		if ok && k == key {
			return v, true
		}
	}
	return "", false
}

func envEquals(env []string, key, want string) bool {
	v, ok := lookupEnv(env, key)
	return ok && v == want
}

func utf8Locale(env []string) bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v, ok := lookupEnv(env, key)
		if !ok || v == "" {
			continue
		}
		u := strings.ToUpper(v)
		return strings.Contains(u, "UTF-8") || strings.Contains(u, "UTF8")
	}
	return false
}

func colorEnabled(env []string) bool {
	_, set := lookupEnv(env, "NO_COLOR")
	return !set
}
