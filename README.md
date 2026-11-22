# mywt

`mywt` manages git worktrees for many projects in one place.

Each session lives at `~/.mywt/worktree-<YYYYMMDDHHmmss>/` and can hold one or
more project worktrees (e.g. `backend/` and `frontend/`) so you can work on
several repos together under one timestamped directory — no more scattering
worktrees across each repo's own folder.

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

Start an empty session, then add a worktree of each repo to it by running
`mywt add` from inside that repo:

```sh
mywt create --name feat-x                       # -> worktree-20260730123456
cd ~/repos/backend  && mywt add worktree-20260730123456   # subfolder = backend (repo basename)
cd ~/repos/frontend && mywt add worktree-20260730123456 --as fe --base develop
cd $(mywt open worktree-20260730123456 fe)               # jump into a project
mywt here                                                # see this session and its project branches
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
| `mywt here` | Show the session containing the current directory, including every project and its branch. |
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

## Releasing

Merging to `main` does not publish a release. After the commit you want is on
`main`, tag it and push the tag:

```sh
git checkout main
git pull
git tag v1.0.0
git push origin v1.0.0
```

`scripts/release.sh 1.0.0` does that same tag and push after checking the
version is semver and the working tree is clean. It does not build the
binaries or create the GitHub release.

Pushing `vMAJOR.MINOR.PATCH` runs GitHub Actions
(`.github/workflows/release.yml`). The workflow tests, cross-compiles the
macOS arm64 and amd64 binaries, writes `mywt_checksums.txt`, and publishes
them. A tag that is not semver fails the job before anything is published.

A pre-release tag such as `v1.0.0-rc.1` is published as a GitHub prerelease.
`scripts/install.sh` installs `/releases/latest`, which ignores prereleases,
so a release candidate does not replace the stable install. A stable tag,
including one with build metadata such as `v1.0.0+build`, stays a normal
release. If the release already exists, the workflow only uploads artifacts
and does not change whether it is a prerelease.

Two ways to do the same thing from the GitHub website:

- **Tag only.** Create the tag in the GitHub UI. That is a tag push, so the
  workflow runs and creates the release.
- **Draft a new release.** On the Releases page, choose "Draft a new release",
  enter a new tag, and publish. GitHub creates the tag and the release
  together. The tag push starts one build. The release already exists, so the
  workflow uploads the binaries and checksums onto it and does not change the
  title or notes you wrote.

`scripts/release.sh --build-only v1.0.0` only writes `dist/`. It does not tag,
push, or publish. That is the command Actions runs, and it is safe to dry-run
locally.

## License

MIT
