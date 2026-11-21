package shell

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fxricky/mywt/internal/termtext"
)

//go:embed shims/bashrc shims/zshenv shims/zshrc shims/env.sh shims/fish.fish
var promptShims embed.FS

const promptTagMax = 32

// PromptTag is the text added to the front of a picker-shell prompt,
// including the trailing space. label is the session label the frame uses.
// parent is the MYWT_SESSION value already set, or empty when this session
// is not nested. An empty label becomes "session", matching the frame's
// last fallback. The caller passes the frame's resolved label, so an empty
// name that fell back to the session id is already that id.
//
// Every rune outside [A-Za-z0-9._+@/-] becomes "_", and a name longer than
// 32 runes is cut to 31 plus "~". The result is ASCII.
func PromptTag(label, parent string) string {
	name := narrowTag(termtext.Sanitize(label))
	if name == "" {
		name = "session"
	}
	if parent == "" {
		return "(mywt:" + name + ") "
	}
	outer := narrowTag(termtext.Sanitize(parent))
	if outer == "" {
		outer = "session"
	}
	return "(mywt:" + outer + ">" + name + ") "
}

func narrowTag(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if tagRune(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	rs := []rune(b.String())
	if len(rs) > promptTagMax {
		return string(rs[:promptTagMax-1]) + "~"
	}
	return b.String()
}

func tagRune(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z':
		return true
	case r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return true
	default:
		return strings.ContainsRune("._+@/-", r)
	}
}

// shellPlan is how Launch starts one shell. Args and Env are what the child
// sees. Files are static shim scripts written into a private temp directory;
// they read the tag from the environment and never contain the session name.
type shellPlan struct {
	Args  []string
	Env   []string
	Files map[string]string
	// Tagged is false for shells mywt does not know how to tag.
	Tagged bool
}

// planShell chooses argv, extra environment, and shim files from the shell's
// base name. shimDir is the private temp directory. tag and inheritedTag are
// already built; this function does not format them. inherited is the
// environment Launch was given, used for ZDOTDIR, HOME, and ENV.
func planShell(shellPath, shimDir, tag, inheritedTag string, inherited []string) shellPlan {
	plan := shellPlan{Args: shellArgs(shellPath)}
	switch filepath.Base(shellPath) {
	case "bash":
		plan.Tagged = true
		plan.Args = []string{"--rcfile", filepath.Join(shimDir, "bashrc"), "-i"}
		plan.Files = map[string]string{"bashrc": mustShim("shims/bashrc")}
	case "zsh":
		plan.Tagged = true
		plan.Args = []string{"-i"}
		plan.Files = map[string]string{
			".zshenv": mustShim("shims/zshenv"),
			".zshrc":  mustShim("shims/zshrc"),
		}
		plan.Env = append(plan.Env, "ZDOTDIR="+shimDir, "MYWT_ORIG_ZDOTDIR="+origZdotdir(inherited))
	case "fish":
		plan.Tagged = true
		plan.Args = []string{"-i", "--init-command", mustShim("shims/fish.fish")}
	case "sh", "dash", "ash", "ksh", "mksh":
		plan.Tagged = true
		plan.Args = []string{"-i"}
		plan.Files = map[string]string{"env.sh": mustShim("shims/env.sh")}
		orig, _ := lookupEnv(inherited, "ENV")
		plan.Env = append(plan.Env, "ENV="+filepath.Join(shimDir, "env.sh"), "MYWT_ORIG_ENV="+orig)
	default:
		return plan
	}
	plan.Env = append(tagEnv(tag, inheritedTag), plan.Env...)
	return plan
}

func tagEnv(tag, inheritedTag string) []string {
	env := []string{"MYWT_PROMPT_TAG=" + tag}
	if inheritedTag != "" {
		env = append(env, "MYWT_PARENT_PROMPT_TAG="+inheritedTag)
	}
	return env
}

func origZdotdir(inherited []string) string {
	if v, ok := lookupEnv(inherited, "ZDOTDIR"); ok && v != "" {
		return v
	}
	if v, ok := lookupEnv(inherited, "HOME"); ok && v != "" {
		return v
	}
	if v := os.Getenv("HOME"); v != "" {
		return v
	}
	return ""
}

func mustShim(name string) string {
	b, err := promptShims.ReadFile(name)
	if err != nil {
		panic("prompt shim " + name + ": " + err.Error())
	}
	return string(b)
}

func promptFamily(shellPath string) string {
	switch filepath.Base(shellPath) {
	case "bash":
		return "bash"
	case "zsh":
		return "zsh"
	case "fish":
		return "fish"
	case "sh", "dash", "ash", "ksh", "mksh":
		return "posix"
	default:
		return ""
	}
}

func writePromptFiles(dir string, files map[string]string) error {
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("prompt dir mode: %w", err)
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return fmt.Errorf("write prompt shim: %w", err)
		}
	}
	return nil
}

func dropEnvKeys(env []string, keys ...string) []string {
	drop := map[string]bool{}
	for _, k := range keys {
		drop[k] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && drop[key] {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func replaceEnv(env, extra []string) []string {
	repl := map[string]string{}
	var order []string
	for _, entry := range extra {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if _, seen := repl[key]; !seen {
			order = append(order, key)
		}
		repl[key] = entry
	}
	out := dropEnvKeys(env, order...)
	for _, key := range order {
		out = append(out, repl[key])
	}
	return out
}
