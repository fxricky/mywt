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
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash
```

Downloads the latest prebuilt macOS binary (Apple Silicon or Intel), verifies its
checksum, and installs it to `/usr/local/bin` (or `~/.local/bin` if that isn't
writable). No Go required.

Install a specific version or directory:

```sh
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash -s -- v0.1.0
curl --proto '=https' --tlsv1.2 -fsSL https://raw.githubusercontent.com/fxricky/mywt/main/scripts/install.sh | bash -s -- --to ~/bin
```

The release binaries are not signed or notarized. A copy downloaded in a
browser is quarantined, and macOS Gatekeeper can refuse to open it.
`scripts/install.sh` uses curl, which does not set that quarantine attribute.
If a browser download will not open, clear the attribute on that file only:

```sh
xattr -d com.apple.quarantine ./mywt
```

After downloading a release asset, confirm it is the binary Actions built:

```sh
gh attestation verify mywt_darwin_arm64 --repo fxricky/mywt
gh attestation verify mywt_darwin_amd64 --repo fxricky/mywt
```

Verification uses the file digest, so the same command works on the installed
`mywt` binary.

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

Picker sessions add `(mywt:<name>) ` to the front of your existing prompt.
mywt does not edit `.bashrc`, `.zshrc`, fish config, or any other file. It
loads your config first and adds the tag on top. The shim files live in a
private temp directory and remove that directory once they have been read,
so nothing is left behind if mywt is killed. fish needs no files.

- bash is started with `--rcfile`. That file sources `~/.bashrc`, then prefixes
  `PS1` and appends the same step to `PROMPT_COMMAND` so a framework that
  rebuilds the prompt still keeps the tag.
- zsh gets a temporary `ZDOTDIR` that sources your `.zshenv` and `.zshrc`
  (including a `ZDOTDIR` your `.zshenv` points at) and then hooks `precmd`.
  Inside the session, `ZDOTDIR` is yours again.
- fish is started with `--init-command`, which wraps `fish_prompt` after
  `config.fish` has run.
- `sh`, `dash`, `ash`, `ksh`, and `mksh` (and bash invoked as `sh`) use the
  POSIX `ENV` file. Your previous `ENV` is sourced and then restored.
- Other shells (nu, xonsh, tcsh, elvish, and anything else) are started exactly
  as before, with no tag and no error. `$MYWT_SESSION` is still set.

`--no-prompt-tag`, or `MYWT_NO_PROMPT_TAG=1`, turns the tag off and leaves the
frame alone. `--no-frame` turns the frame off and leaves the tag alone. The
two switches are independent. With the tag off, `$MYWT_PROMPT_TAG` is not set;
use `$MYWT_SESSION` from your own prompt config (starship, for example).

A nested picker session keeps the warning and the frame's `(nested in X)`, and
the prompt becomes `(mywt:<parent>><name>) `, for example
`(mywt:auth-refactor>spike) `. Only the immediate parent is shown.

`exec bash` or `exec zsh` inside the session starts a fresh shell and drops
the tag. A fish prompt that a framework redefines on a later event (tide on a
variable change, for example) can drop the tag. Powerlevel10k's instant prompt
can appear briefly before the tag. `mywt open --shell` does not add a tag.
SIGHUP, including a closed terminal, is forwarded to the shell and mywt then
exits 129. Interactive bash and zsh ignore SIGTERM, so mywt does not catch it.

`claude` and `codex`, if installed, can be started from the session root
without `cd`-ing into a child repository. They can see the child folders
subject to each tool's own filesystem permissions and trust settings. `mywt`
does not launch them for you.

The environment of that shell includes `MYWT_SESSION` (the name, or the id),
`MYWT_SESSION_ID`, `MYWT_SESSION_ROOT`, and, when the tag is on,
`MYWT_PROMPT_TAG`.

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

## Releasing

Merging to `main` does not publish a release. Every release starts from the
Release workflow on `main`. Pushing a tag does not publish one.

From the Actions tab, open Release and choose Run workflow. The branch must
be `main`. Enter the version as `1.1.0` or `v1.1.0`. Or:

```sh
scripts/release.sh 1.1.0
```

`scripts/release.sh` checks that the version is semver, then runs
`gh workflow run release.yml --ref main -f version=v<version>`. It does not
create a tag, push a tag, or create a release. A semver pre-release tag such
as `v1.1.0-rc.1` is marked as a prerelease when Actions creates the release.
`scripts/install.sh` installs `/releases/latest`, which ignores prereleases.

Do not create the release on the Releases page, and do not push a `v*` tag
by hand. Immutable releases stay on. A release created on that page is
published before the binaries exist, and an immutable release then refuses
the asset upload.

The workflow (`.github/workflows/release.yml`) runs only from Run workflow.
The build job requires `refs/heads/main`, checks that the version is semver,
and fails when that tag or a release for it already exists. The commit must
be on `origin/main`. The job tests and runs `scripts/build-release.sh`. That
writes `dist/mywt_darwin_arm64`, `dist/mywt_darwin_amd64`, and
`dist/mywt_checksums.txt`, with the tag injected as `v<version>`.

The publish job uses the environment `release`. After that approval it
attests the binaries, creates the tag at this run's `GITHUB_SHA`, and pushes
the tag with the workflow `GITHUB_TOKEN`. It creates a draft with
`gh release create --draft --verify-tag --generate-notes`, and adds
`--prerelease` for a pre-release tag. It uploads the three files without
replacing an existing name, downloads them, and compares them to `dist/`.
When they match, it publishes the draft with `gh release edit --draft=false`.
If a release for the tag already exists, the job fails and leaves that
release as it is. The release remains a draft until the uploaded files
match, so it is not listed as the latest release while the binaries are
missing.

If the upload or the comparison fails, the job stops before publishing. The
tag, and a draft when one was created, stay in place. Delete that draft and
the tag, then run the workflow again. A later run will not change a release
that already exists.

`scripts/build-release.sh v0.0.0-dev` writes `dist/` and does not tag, push,
or publish. The checksum file lists `mywt_darwin_arm64` and
`mywt_darwin_amd64`, which are the asset names `scripts/install.sh`
downloads. `mywt version` prints the injected tag.

### Repository settings

The workflow does not change repository administration. Set these yourself:

- An environment named `release`. Allow deployments from `main` only, and
  require Ricky as a reviewer. The publish job uses `environment: release`,
  so it waits for that approval before it creates a tag, opens a draft, or
  uploads binaries.
- A ruleset for `v*` tags that restricts updates and deletions. Ricky is the
  only bypass actor, so only Ricky can move or delete one of these tags.
  Leave Restrict creations off so CI can create the tag. The publish job
  pushes it with the workflow `GITHUB_TOKEN`. GitHub does not accept the
  built-in GitHub Actions app as a ruleset bypass actor, so that token
  cannot be added to the bypass list. With creations left open, the
  workflow can create the tag, and only Ricky can move or delete it.
- Branch protection on `main`: require a pull request, and block force
  pushes and deletion. Releases are built only from commits already on
  `main`.
- Leave immutable releases on (Settings, General). The workflow uploads the
  binaries to a draft and publishes that draft only after the files match.
  A published release cannot have assets added or replaced.

## License

MIT
