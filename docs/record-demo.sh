#!/usr/bin/env bash
# Rebuild docs/demo.cast and docs/demo.gif from a real mywt binary.
#
# Requires: Go 1.25, git, python3, and agg (https://github.com/asciinema/agg)
# on PATH. The GIF is what the binary and the shell print; the script does
# not draw frames itself.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
demo=/tmp/mywt-demo
real_home=${HOME}

rm -rf "$demo"
mkdir -p "$demo/home/projects" "$demo/bin" \
  "$demo/repos/backend" "$demo/repos/frontend" "$demo/repos/payments"

GOTOOLCHAIN=${GOTOOLCHAIN:-go1.25.0} go build -C "$root" -o "$demo/bin/mywt" .

export HOME="$demo/home"
export PATH="$demo/bin:$PATH"
# The inner shell is the user's real interactive bash, which reads this home.
# A short prompt and no first-login sudo hint keep the recording on mywt.
printf "PS1='\$ '\nunset PROMPT_COMMAND\n" >"$demo/home/.bashrc"
touch "$demo/home/.hushlogin"
export DEMO_HOME="$demo/home"
export DEMO_PROJECTS="$demo/home/projects"
export CAST_PATH="$root/docs/demo.cast"

init_repo() {
  local dir=$1
  git -C "$dir" init -q -b main
  git -C "$dir" config user.email demo@example.com
  git -C "$dir" config user.name "mywt demo"
  git -C "$dir" config commit.gpgsign false
  printf 'demo\n' >"$dir/README"
  git -C "$dir" add README
  git -C "$dir" commit -q -m init
}

init_repo "$demo/repos/backend"
init_repo "$demo/repos/frontend"
init_repo "$demo/repos/payments"

# Older session, so the picker leaves auth-refactor highlighted.
(
  cd "$demo/repos/payments"
  "$demo/bin/mywt" create --name payment-hotfix >/dev/null
  "$demo/bin/mywt" add payment-hotfix --as payments >/dev/null
)
(
  cd "$demo/repos/backend"
  "$demo/bin/mywt" create --name auth-refactor >/dev/null
  "$demo/bin/mywt" add auth-refactor --as backend >/dev/null
)
(
  cd "$demo/repos/frontend"
  "$demo/bin/mywt" add auth-refactor --as frontend >/dev/null
)

python3 "$root/docs/record-demo.py"

if ! command -v agg >/dev/null 2>&1; then
  echo "agg is not on PATH. Install it from https://github.com/asciinema/agg/releases and re-run." >&2
  exit 1
fi

agg \
  --font-family "JetBrains Mono,DejaVu Sans Mono,Noto Sans Mono" \
  --font-size 14 \
  --line-height 1.3 \
  --theme github-dark \
  --fps-cap 12 \
  --idle-time-limit 2 \
  --last-frame-duration 2 \
  "$root/docs/demo.cast" "$root/docs/demo.gif"

bytes=$(wc -c <"$root/docs/demo.gif")
echo "wrote $root/docs/demo.gif ($bytes bytes)"
if [ "$bytes" -gt 2000000 ]; then
  echo "demo.gif is over 2MB; lower --fps-cap or --font-size in this script" >&2
  exit 1
fi

# The demo HOME is disposable. Restore nothing: this script is a subprocess.
unset HOME
export HOME="$real_home"
