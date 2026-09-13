# Repository Guidelines

## Project Structure & Module Organization

`mywt` is a Go CLI for managing groups of Git worktrees. The entry point is
`main.go`; Cobra command definitions live in `cmd/`. Reusable implementation
packages are under `internal/`: `session` manages sessions and projects,
`git` wraps Git operations, `config` resolves `~/.mywt`, and `version` owns
build-time version data. Tests are colocated with their packages as
`*_test.go`. `scripts/` contains the macOS installer and release automation.
Update `README.md` when changing user-facing commands or workflows.

## Build, Test, and Development Commands

Use Go 1.25 (see `go.mod`):

```sh
go build -o mywt .       # build the local CLI
go test ./...             # run all package tests
go test -race ./...      # run tests with race detection
gofmt -w $(rg --files -g '*.go')  # format Go sources
```

Run the local binary directly, for example `go run . list`. The release flow
is macOS-specific and requires an authenticated GitHub CLI:
`scripts/release.sh 0.1.0` builds both macOS architectures, creates checksums,
tags the repository, and publishes a GitHub release.

## Coding Style & Naming Conventions

Follow standard `gofmt` formatting and idiomatic Go naming: exported names use
PascalCase, local variables and unexported helpers use camelCase, and errors
are wrapped with useful context. Keep Cobra command wiring in `cmd/` and
domain logic in `internal/`; avoid introducing global state unless required
by Cobra. Shell scripts use Bash strict mode (`set -euo pipefail`) and should
preserve that behavior.

## Testing Guidelines

Use Go's built-in `testing` package. Name tests `Test<Subject>` and use
subtests for input variations. Add focused unit tests beside changed logic;
there is no documented coverage threshold, but every behavior change should
be covered where practical. Run `go test ./...` before submitting changes.

## Commit & Pull Request Guidelines

Use short, imperative Conventional-Commit-style subjects such as
`test: add unit tests` or `dist: update installer`. Keep commits focused.
Pull requests should explain the user impact, summarize implementation,
mention validation commands (for example, `go test ./...`), and update
`README.md` for CLI or installation changes. Include reproduction steps for
bug fixes and screenshots only when they clarify a user-facing change.

## Security & Configuration Tips

Normal operation reads and writes under `~/.mywt` and invokes Git commands.
Do not commit generated binaries, release artifacts, credentials, or local
session data. Review changes to `scripts/install.sh` carefully because it
downloads and installs executable releases.
