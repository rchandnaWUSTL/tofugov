#!/usr/bin/env bash
# One terminal session for the Remotion video, run inside `asciinema rec`.
# Usage: video_session.sh old|new   (fixtures must already be seeded)
set -euo pipefail
cd "$(dirname "$0")/.."
if [ -f .env ]; then set -a; . ./.env; set +a; fi
export PATH="$PWD/bin:$PATH"
export TOFUGOV_DB="$PWD/.tofugov/video.db"
cd fixtures/workspaces

prompt() {
  printf '\e[32m$\e[0m '
  local s="$1"
  sleep 0.4
  for ((i = 0; i < ${#s}; i++)); do printf '%s' "${s:i:1}"; sleep 0.035; done
  sleep 0.3
  echo
}

case "$1" in
  old)
    cd ws-06-db-credentials
    prompt "tofu plan"
    tofu plan -input=false
    ;;
  new)
    prompt "tofugov upgrade --provider hashicorp/random=3.9.1 ws-*"
    tofugov upgrade --provider hashicorp/random=3.9.1 ws-* 2>/dev/null | sed '/^Shadow mode/,$d'
    ;;
esac
sleep 1
