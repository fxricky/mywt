package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptTag(t *testing.T) {
	if got := PromptTag("auth-refactor", ""); got != "(mywt:auth-refactor) " {
		t.Fatalf("plain = %q", got)
	}
	cases := []struct {
		label, parent, want string
	}{
		{"a b", "", "(mywt:a_b) "},
		{"$(touch x)", "", "(mywt:__touch_x_) "},
		{"`id`", "", "(mywt:_id_) "},
		{"%~", "", "(mywt:__) "},
		{`\w`, "", "(mywt:_w) "},
		{"\x1b[31mred\x1b[0m", "", "(mywt:__31mred__0m) "},
		{"\u009b\u0085", "", "(mywt:__) "},
		{"测", "", "(mywt:_) "},
		{"", "", "(mywt:session) "},
		{"spike", "auth-refactor", "(mywt:auth-refactor>spike) "},
		{"b", "a b", "(mywt:a_b>b) "},
		{"", "parent", "(mywt:parent>session) "},
	}
	for _, tc := range cases {
		got := PromptTag(tc.label, tc.parent)
		if got != tc.want {
			t.Errorf("PromptTag(%q, %q) = %q, want %q", tc.label, tc.parent, got, tc.want)
		}
		if strings.ContainsAny(tagName(got), " $`\\%\t\n\r") {
			t.Errorf("tag name is not inert: %q", got)
		}
	}

	exact := strings.Repeat("a", promptTagMax)
	if got := PromptTag(exact, ""); got != "(mywt:"+exact+") " {
		t.Fatalf("32-rune name changed: %q", got)
	}
	long := strings.Repeat("b", promptTagMax+5)
	want := "(mywt:" + strings.Repeat("b", promptTagMax-1) + "~) "
	if got := PromptTag(long, ""); got != want {
		t.Fatalf("truncated = %q, want %q", got, want)
	}
	parent := strings.Repeat("p", promptTagMax+2)
	child := strings.Repeat("c", promptTagMax+2)
	got := PromptTag(child, parent)
	want = "(mywt:" + strings.Repeat("p", promptTagMax-1) + "~>" + strings.Repeat("c", promptTagMax-1) + "~) "
	if got != want {
		t.Fatalf("nested truncation = %q, want %q", got, want)
	}
}

func tagName(tag string) string {
	s := strings.TrimPrefix(tag, "(mywt:")
	s = strings.TrimSuffix(s, ") ")
	return s
}

func TestPlanShellFamilies(t *testing.T) {
	const (
		dir  = "/tmp/mywt-prompt-test"
		tag  = "(mywt:auth-refactor) "
		home = "/home/demo"
	)
	inherited := []string{"HOME=" + home, "ZDOTDIR=/custom/zsh", "ENV=/custom/env", "MYWT_PROMPT_TAG=(mywt:old) "}

	bash := planShell("/usr/local/bin/bash", dir, tag, "", inherited)
	if !bash.Tagged || strings.Join(bash.Args, " ") != "--rcfile "+filepath.Join(dir, "bashrc")+" -i" {
		t.Fatalf("bash plan = %+v", bash.Args)
	}
	if !hasEnv(bash.Env, "MYWT_PROMPT_TAG", tag) || hasKey(bash.Env, "MYWT_PARENT_PROMPT_TAG") || !hasEnv(bash.Env, "MYWT_SHIM_DIR", dir) {
		t.Fatalf("bash env = %v", bash.Env)
	}
	if _, ok := bash.Files["bashrc"]; !ok || strings.Contains(bash.Files["bashrc"], "auth-refactor") {
		t.Fatal("bash shim missing or contains the session name")
	}

	bare := planShell("bash", dir, tag, "(mywt:old) ", nil)
	if bare.Args[0] != "--rcfile" || !hasEnv(bare.Env, "MYWT_PARENT_PROMPT_TAG", "(mywt:old) ") {
		t.Fatalf("bare bash = args %v env %v", bare.Args, bare.Env)
	}

	zsh := planShell("/bin/zsh", dir, tag, "", inherited)
	if strings.Join(zsh.Args, " ") != "-i" || !hasEnv(zsh.Env, "ZDOTDIR", dir) || !hasEnv(zsh.Env, "MYWT_ORIG_ZDOTDIR", "/custom/zsh") || !hasEnv(zsh.Env, "MYWT_SHIM_DIR", dir) {
		t.Fatalf("zsh = args %v env %v", zsh.Args, zsh.Env)
	}
	if _, ok := zsh.Files[".zshenv"]; !ok {
		t.Fatal("zsh shim files missing")
	}
	if _, ok := zsh.Files[".zshrc"]; !ok {
		t.Fatal("zshrc shim missing")
	}

	noZ := planShell("zsh", dir, tag, "", []string{"HOME=" + home})
	if !hasEnv(noZ.Env, "MYWT_ORIG_ZDOTDIR", home) {
		t.Fatalf("default ZDOTDIR = %v", noZ.Env)
	}

	fish := planShell("/opt/homebrew/bin/fish", dir, tag, "", nil)
	if len(fish.Files) != 0 || fish.Args[0] != "-i" || fish.Args[1] != "--init-command" {
		t.Fatalf("fish args = %v files %v", fish.Args, fish.Files)
	}
	if strings.Contains(fish.Args[2], "auth-refactor") || !strings.Contains(fish.Args[2], "MYWT_PROMPT_TAG") {
		t.Fatal("fish init command is not a static shim")
	}
	if hasKey(fish.Env, "MYWT_SHIM_DIR") || len(fish.Files) != 0 {
		t.Fatalf("fish should not get a shim directory: env %v files %v", fish.Env, fish.Files)
	}

	for _, name := range []string{"sh", "dash", "ksh", "mksh", "ash", "/bin/sh"} {
		p := planShell(name, dir, tag, "", inherited)
		if !p.Tagged || strings.Join(p.Args, " ") != "-i" {
			t.Fatalf("%s args = %v", name, p.Args)
		}
		if !hasEnv(p.Env, "ENV", filepath.Join(dir, "env.sh")) || !hasEnv(p.Env, "MYWT_ORIG_ENV", "/custom/env") || !hasEnv(p.Env, "MYWT_SHIM_DIR", dir) {
			t.Fatalf("%s env = %v", name, p.Env)
		}
		if strings.Contains(p.Files["env.sh"], "auth-refactor") {
			t.Fatalf("%s shim contains the session name", name)
		}
	}

	unknown := planShell("/usr/local/bin/nu", dir, tag, "(mywt:old) ", inherited)
	if unknown.Tagged || len(unknown.Args) != 0 || len(unknown.Env) != 0 || len(unknown.Files) != 0 {
		t.Fatalf("unknown plan = %+v", unknown)
	}
	for _, args := range [][]string{bash.Args, zsh.Args, fish.Args} {
		for _, a := range args {
			if a == "-l" || a == "--login" {
				t.Fatalf("login flag in %v", args)
			}
		}
	}
}

func TestPromptTagOptOutAndNestedEnv(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "bash")
	body := "#!/bin/sh\nprintf 'ARGS:%s\\n' \"$*\"\nprintf 'TAG:%s\\n' \"${MYWT_PROMPT_TAG-}\"\nprintf 'PARENT:%s\\n' \"${MYWT_PARENT_PROMPT_TAG-}\"\nprintf 'SHIM:%s\\n' \"${MYWT_SHIM_DIR-}\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	session := t.TempDir()
	base := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()}

	run := func(t *testing.T, extra []string, noTag, noFrame bool) (string, string) {
		t.Helper()
		var stdout, stderr strings.Builder
		err := Launch(Options{
			Shell: script, Dir: session, Label: "auth-refactor", SessionID: "worktree-1",
			Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
			Env: append(append([]string{}, base...), extra...), NoPromptTag: noTag, NoFrame: noFrame,
		})
		if err != nil {
			t.Fatalf("Launch: %v\n%s\n%s", err, stdout.String(), stderr.String())
		}
		return stdout.String(), stderr.String()
	}

	stdout, stderr := run(t, []string{"MYWT_NO_PROMPT_TAG=0"}, false, false)
	if !strings.Contains(stdout, "ARGS:--rcfile") || !strings.Contains(stdout, "TAG:(mywt:auth-refactor) ") || !strings.Contains(stdout, "SHIM:"+os.TempDir()+"/mywt-prompt-") {
		t.Fatalf("MYWT_NO_PROMPT_TAG=0 did not keep the tag:\n%s", stdout)
	}
	if strings.Contains(stderr, "Session root") {
		t.Fatal("tag-on launch used the legacy banner")
	}

	stdout, stderr = run(t, []string{"MYWT_NO_PROMPT_TAG=1", "MYWT_PROMPT_TAG=(mywt:old) ", "MYWT_SHIM_DIR=" + filepath.Join(os.TempDir(), "mywt-prompt-stale")}, false, false)
	if stdout != "ARGS:-i\nTAG:\nPARENT:\nSHIM:\n" {
		t.Fatalf("MYWT_NO_PROMPT_TAG=1 args/env =\n%s", stdout)
	}
	if !strings.Contains(stderr, "Type exit to return to") || strings.Contains(stderr, "Session root") {
		t.Fatalf("opt-out dropped the frame:\n%s", stderr)
	}

	stdout, _ = run(t, []string{"MYWT_PROMPT_TAG=(mywt:old) ", "MYWT_SHIM_DIR=" + filepath.Join(os.TempDir(), "mywt-prompt-stale")}, true, false)
	if stdout != "ARGS:-i\nTAG:\nPARENT:\nSHIM:\n" {
		t.Fatalf("NoPromptTag args/env =\n%s", stdout)
	}

	stdout, stderr = run(t, nil, false, true)
	if !strings.Contains(stdout, "TAG:(mywt:auth-refactor) ") || !strings.Contains(stdout, "--rcfile") {
		t.Fatalf("--no-frame removed the tag:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Session root") || strings.Contains(stderr, "╭") {
		t.Fatalf("--no-frame did not keep the legacy banner:\n%s", stderr)
	}

	stdout, stderr = run(t, []string{"MYWT_SESSION=parent", "MYWT_PROMPT_TAG=(mywt:parent) "}, false, false)
	if !strings.Contains(stdout, "TAG:(mywt:parent>auth-refactor) ") {
		t.Fatalf("nested tag =\n%s", stdout)
	}
	if !strings.Contains(stdout, "PARENT:(mywt:parent) ") {
		t.Fatalf("parent tag was not copied:\n%s", stdout)
	}
	if !strings.Contains(stderr, "nested in parent") {
		t.Fatalf("nested frame missing:\n%s", stderr)
	}

	// Empty label uses the session id, the same value the frame shows.
	var stdoutB, stderrB strings.Builder
	err := Launch(Options{
		Shell: script, Dir: session, Label: "", SessionID: "worktree-9",
		Stdin: strings.NewReader(""), Stdout: &stdoutB, Stderr: &stderrB,
		Env: base,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdoutB.String(), "TAG:(mywt:worktree-9) ") {
		t.Fatalf("empty label tag =\n%s", stdoutB.String())
	}
	if !strings.Contains(stderrB.String(), "mywt: worktree-9") {
		t.Fatalf("empty label frame =\n%s", stderrB.String())
	}
}

func TestPromptShimModesAndCleanup(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "bash")
	body := `#!/bin/sh
rc=$2
shim=$(dirname "$rc")
mode() { python3 -c 'import os,sys; print(format(os.stat(sys.argv[1]).st_mode & 0o777, "o"))' "$1"; }
printf 'DIR:%s\n' "$shim"
printf 'DIRMODE:%s\n' "$(mode "$shim")"
printf 'FILEMODE:%s\n' "$(mode "$rc")"
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr strings.Builder
	err := Launch(Options{
		Shell: script, Dir: t.TempDir(), Label: "auth-refactor", SessionID: "worktree-1",
		Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr,
		Env: []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir()},
	})
	if err != nil {
		t.Fatalf("Launch: %v\n%s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "DIRMODE:700") || !strings.Contains(out, "FILEMODE:600") {
		t.Fatalf("modes =\n%s", out)
	}
	shim := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "DIR:") {
			shim = strings.TrimPrefix(line, "DIR:")
		}
	}
	if shim == "" {
		t.Fatalf("no shim dir in\n%s", out)
	}
	if _, err := os.Stat(shim); !os.IsNotExist(err) {
		t.Fatalf("shim dir %s still exists after Launch (%v)", shim, err)
	}

	bad := filepath.Join(dir, "bash-bad")
	if err := os.WriteFile(bad, []byte("#!/no/such/mywt-interpreter\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := listPromptDirs(t)
	err = Launch(Options{
		Shell: bad, Dir: t.TempDir(), Label: "auth-refactor", SessionID: "worktree-1",
		Env: []string{"PATH=" + os.Getenv("PATH")},
	})
	if err == nil {
		t.Fatal("expected the shell to fail to start")
	}
	after := listPromptDirs(t)
	if len(after) > len(before) {
		t.Fatalf("shim dirs left behind: before %v after %v", before, after)
	}
}

func listPromptDirs(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "mywt-prompt-*"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func hasEnv(env []string, key, want string) bool {
	v, ok := lookupEnv(env, key)
	return ok && v == want
}

func hasKey(env []string, key string) bool {
	_, ok := lookupEnv(env, key)
	return ok
}
