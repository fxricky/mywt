# mywt

`mywt` manages git worktrees for many projects in one place.

Each session lives at `~/.mywt/worktree-<YYYYMMDDHHmmss>/` and can hold one or
more project worktrees (e.g. `backend/` and `frontend/`) so you can work on
several repos together under one directory. That session folder is the
**session root**. It is not a git repository. The repository worktrees inside
it keep their own branches and history.

Running `mywt` with no arguments in a terminal opens a session selector and
then an interactive shell at that session root. No shell rc changes and no
tmux install are required.

![Entering a mywt session](docs/demo.gif)

Recorded from the built `mywt` binary (`mywt` → picker → Enter → session-root banner → `pwd` / `ls` → `exit` back to the original directory). Regenerate with [`docs/record-demo.sh`](docs/record-demo.sh), which needs [agg](https://github.com/asciinema/agg) on `PATH`.

## Install (macOS)

### curl (recommended)

```sh
curl -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash
```

Downloads the latest prebuilt macOS binary (Apple Silicon or Intel), verifies its
checksum, and installs it to `/usr/local/bin` (or `~/.local/bin` if that isn't
writable). No Go required.

Install a specific version or directory:

```sh
curl -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash -s -- v0.1.0
curl -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash -s -- --to ~/bin
```

### go install

If you have Go installed:

```sh
go install github.com/fxricky/mywt@latest
```

### From source

```sh
git clone https://github.com/fxricky/mywt.git
cd mywt
go build -o mywt .
```

Then move the `mywt` binary onto your `PATH` (e.g. `mv mywt /usr/local/bin/`).

## Usage

Create a named session, add a worktree of each repo, then enter the session
root with `mywt`:

```sh
mywt create --name auth-refactor
cd ~/repos/backend  && mywt add auth-refactor          # subfolder = backend (repo basename)
cd ~/repos/frontend && mywt add auth-refactor --as fe --base develop
mywt                                                   # select auth-refactor
# shell starts in the session root; both worktrees are subfolders
pwd                                                    # ~/.mywt/worktree-<timestamp>
ls                                                     # backend  fe
claude                                                 # or: codex
exit                                                   # back to the previous shell and directory

mywt here                                              # from inside a project: session and branches
mywt list
mywt open auth-refactor fe                             # print one project's path
mywt remove auth-refactor --delete-branches            # clean up
```

### Enter a session

`mywt` with no arguments, in a terminal:

- Lists sessions. A named session is shown by name; an unnamed session is shown
  by its id (`worktree-<timestamp>`). Each row includes the repository count.
- Arrow keys move, Enter starts a shell, Esc or Ctrl+C cancels without
  starting one.
- If there are no sessions, it prints how to create one and exits.
- If stdin or stdout is not a terminal, it exits with an error and tells you
  to use `mywt list` or `mywt open <session> --shell`. It does not wait for
  input.

The shell is `$SHELL` when that program exists, otherwise `/bin/sh` (then
`/bin/bash` or `/bin/zsh`). It is an interactive child process. Its working
directory is the **session root**, so `ls` shows every repository added to the
session. `mywt` does not `cd` your current shell, does not edit `.zshrc` /
`.bashrc` / fish config, and does not use tmux.

On a terminal, startup draws a frame and exit prints a footer. The frame names
the session, the session root, and the directory `exit` returns to. The width
follows the terminal, between 40 and 80 columns. A UTF-8 locale uses box
drawing; any other locale uses ASCII. `NO_COLOR` turns off styling.

```text
╭─ mywt: auth-refactor ──────────────────────────────╮
│ ~/.mywt/worktree-20260730123456                    │
│ Type exit to return to ~/projects                  │
╰────────────────────────────────────────────────────╯
```

```text
╰─ left mywt: auth-refactor (exit 0) · back in ~/projects
```

The window title is saved, set to `mywt: auth-refactor`, and restored when the
shell exits. The frame and title are not a pinned header; a header that stays
on screen while other programs run is follow-up work. The child shell is
ephemeral: exit (or the shell exiting) returns you to the original shell and
directory, and restores the terminal mode. Ctrl+C inside the session
interrupts the foreground command, the same way it does in a normal shell.

`--no-frame`, or `MYWT_NO_FRAME=1`, prints the plain banner instead:

```text
[mywt: auth-refactor]  ~/.mywt/worktree-20260730123456
```

Starting `mywt` from inside a session prints a warning and marks the frame
`(nested in <session>)`. The shell still starts. When stderr is not a
terminal, the same facts are printed as plain text, with no box and no title
sequences.

`claude` and `codex`, if installed, can be started from the session root
without `cd`-ing into a child repository. They can see the child folders
subject to each tool's own filesystem permissions and trust settings. `mywt`
does not launch them for you.

The environment of that shell includes `MYWT_SESSION` (the name, or the id),
`MYWT_SESSION_ID`, and `MYWT_SESSION_ROOT`. `mywt` does not change your prompt.
You can reference `MYWT_SESSION` from your own prompt config if you want it there.

`mywt open` is unchanged. With no flags it prints a path (session root, or one
project). `--shell` still spawns a shell in that path, and `--editor` still
opens an editor. The selector does not replace it.

### Commands

| Command | Description |
| --- | --- |
| `mywt` | Interactive selector. Enter starts `$SHELL` at the session root. |
| `mywt create [--name <name>]` | Create a new empty timestamped session. `--name` must be a valid git branch name; it becomes the default branch for repositories added later. Run from anywhere. |
| `mywt add <session> [--as <name>] [--branch <name>] [--branch-prefix <p>] [--base <ref>]` | Add a worktree of the current repo to a session. Must run inside a git repo. Subfolder defaults to the repo basename. |
| `mywt list [--json]` | List all sessions and their projects. |
| `mywt remove <session> [--force] [--delete-branches]` | Remove a session and all its project worktrees. Branches are kept unless `--delete-branches` is set. |
| `mywt open <session> [project] [--editor <app>] [--shell]` | Print the path of a session or project (default), open it in an editor, or spawn a shell inside it. |
| `mywt here` | Show the session containing the current directory, including every project and its branch. |
| `mywt prune [--dry-run]` | Clean up stale worktrees and broken session folders. |
| `mywt config show` | Print the worktree root (`~/.mywt`). |
| `mywt version` | Print the version. |

### Session root and repository worktrees

```text
~/.mywt/
└── worktree-<timestamp>/          <- session root; the interactive shell starts here
    ├── .session.json
    ├── backend/                   <- git worktree, its own branch
    └── frontend/                  <- git worktree, its own branch
```

The configured root comes from `mywt config show` (always `~/.mywt`). Nothing
here turns the session root into a parent git repository.

### How branch names work

For a **named** session, each new worktree defaults to a branch of that same
name. `auth-refactor` in the backend repo and `auth-refactor` in the frontend
repo are independent branches. `--branch <name>` still wins. For an **unnamed**
session, the default is still `wt-<timestamp>` (for example
`wt-20260730123456`). `--branch-prefix` changes that prefix, and passing it
explicitly also selects the `<prefix>-<timestamp>` form for a named session.

Names are checked with `git check-ref-format --branch`. A session name that is
not a valid branch is rejected at `mywt create` instead of being rewritten.
If the chosen branch already exists or is already checked out in that
repository, `mywt add` stops with an error. It does not reuse, reset, move, or
delete the branch. Pass `--branch <other-name>` to create a different one.
Sessions and branches that already exist are left as they are; changing the
default does not rename them.

`--base` defaults to the repo's default branch (auto-detected: `origin/HEAD`
-> `main` -> `master` -> current `HEAD`).

### Session resolution

`<session>` can be the full id (`worktree-20260730123456`), the bare timestamp
(`20260730123456`), or the `--name` you set at create time. Ambiguous matches
error. The interactive selector shows the id next to a name when two sessions
share it.

## License

MIT
