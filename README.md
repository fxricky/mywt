# mywt

`mywt` manages git worktrees for many projects in one place.

Each session lives at `~/.mywt/worktree-<YYYYMMDDHHmmss>/` and can hold one or
more project worktrees (e.g. `backend/` and `frontend/`) so you can work on
several repos together under one timestamped directory — no more scattering
worktrees across each repo's own folder.

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
version is semver, the working tree is clean, and `HEAD` is already on
`origin/main`. It does not build the binaries or create the GitHub release.
A commit that is not on `main` is refused before a tag is created.

Pushing `vMAJOR.MINOR.PATCH` runs GitHub Actions
(`.github/workflows/release.yml`). The build job checks that the tagged
commit is on `origin/main`, tests, and runs `scripts/build-release.sh`.
That writes `dist/mywt_darwin_arm64`, `dist/mywt_darwin_amd64`, and
`dist/mywt_checksums.txt`, with the tag injected as `v<version>`. The
publish job (environment `release`) attests those binaries, creates a draft
with `gh release create --verify-tag --draft`, uploads the assets, checks
the uploaded bytes against `mywt_checksums.txt`, and only then publishes
with `gh release edit --draft=false`. A tag that is not semver, or a tag
whose commit is not on `main`, fails before anything is published.

A pre-release tag such as `v1.0.0-rc.1` is published as a GitHub prerelease.
`scripts/install.sh` installs `/releases/latest`, which ignores prereleases,
so a release candidate does not replace the stable install. A stable tag,
including one with build metadata such as `v1.0.0+build`, stays a normal
release.

If a GitHub release for the tag already exists, the publish job fails and
does not change its title, notes, or assets. Create the tag only. Do not
create the release in the GitHub UI first.

`scripts/build-release.sh v0.0.0-dev` writes `dist/` and does not tag, push,
or publish. The checksum file lists `mywt_darwin_arm64` and
`mywt_darwin_amd64`, which are the asset names `scripts/install.sh`
downloads. `mywt version` prints the injected tag.

### Repository settings

The workflow does not change repository administration. Set these yourself:

- A ruleset for `v*` tags that blocks deleting or moving a tag and limits who
  can create one.
- An environment named `release` with required reviewers. The publish job
  uses `environment: release`, so a version tag waits for that approval
  before it writes the GitHub release.
- Branch protection on `main`: require a pull request, and block force pushes
  and deletion. Releases are built only from commits already on `main`.
- Immutable releases (Settings, General, Immutable releases). A published
  release then keeps its tag and assets. The workflow creates its own draft,
  uploads, checks the checksums, and only then publishes.

## License

MIT
