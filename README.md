# mywt

`mywt` manages git worktrees for many projects in one place.

Each session lives at `~/.mywt/worktree-<YYYYMMDDHHmmss>/` and can hold one or
more project worktrees (e.g. `backend/` and `frontend/`) so you can work on
several repos together under one timestamped directory — no more scattering
worktrees across each repo's own folder.

## Install (macOS)

### Homebrew (recommended)

```sh
brew install fxricky/tap/mywt
```

This builds from source; Homebrew installs Go temporarily, so you don't need Go
pre-installed.

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

Start an empty session, then add a worktree of each repo to it by running
`mywt add` from inside that repo:

```sh
mywt create --name feat-x                       # -> worktree-20260730123456
cd ~/repos/backend  && mywt add worktree-20260730123456   # subfolder = backend (repo basename)
cd ~/repos/frontend && mywt add worktree-20260730123456 --as fe --base develop
cd $(mywt open worktree-20260730123456 fe)               # jump into a project
mywt list                                                # see all sessions
mywt remove worktree-20260730123456 --delete-branches    # clean up
```

### Commands

| Command | Description |
| --- | --- |
| `mywt create [--name <name>]` | Create a new empty timestamped session. Run from anywhere. |
| `mywt add <session> [--as <name>] [--branch <name>] [--branch-prefix <p>] [--base <ref>]` | Add a worktree of the current repo to a session. Must run inside a git repo. Subfolder defaults to the repo basename; a new branch is created. |
| `mywt list [--json]` | List all sessions and their projects. |
| `mywt remove <session> [--force] [--delete-branches]` | Remove a session and all its project worktrees. Branches are kept unless `--delete-branches` is set. |
| `mywt open <session> [project] [--editor <app>] [--shell]` | Print the path of a session or project (default), open it in an editor, or spawn a shell inside it. |
| `mywt prune [--dry-run]` | Clean up stale worktrees and broken session folders. |
| `mywt config show` | Print the worktree root (`~/.mywt`). |
| `mywt version` | Print the version. |

### How branch names work

By default each project gets a new branch named `wt-<session-timestamp>`
(e.g. `wt-20260730123456`). Override with `--branch`, or change the prefix with
`--branch-prefix`. `--base` defaults to the repo's default branch
(auto-detected: `origin/HEAD` -> `main` -> `master` -> current `HEAD`).

### Session resolution

`<session>` can be the full id (`worktree-20260730123456`), the bare timestamp
(`20260730123456`), or the `--name` you set at create time. Ambiguous matches
error.

## License

MIT
