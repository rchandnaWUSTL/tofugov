#!/usr/bin/env bash
# Record the two terminal sessions the Remotion video replays.
set -euo pipefail
cd "$(dirname "$0")/.."
export TF_PLUGIN_CACHE_DIR="${TF_PLUGIN_CACHE_DIR:-$HOME/.tofu-plugin-cache}"
mkdir -p "$TF_PLUGIN_CACHE_DIR" demo/video/public
rm -f .tofugov/video.db

make -s build reset seed >/dev/null 2>&1
(cd fixtures/workspaces/ws-06-db-credentials &&
  sed -i '' 's/3.5.1/3.9.1/; s/2.4.0/2.9.1/' versions.tf &&
  tofu init -upgrade -input=false >/dev/null)
asciinema rec --headless --window-size 100x34 --overwrite -q \
  -c "scripts/video_session.sh old" demo/video/public/old.cast

make -s reset seed >/dev/null 2>&1
asciinema rec --headless --window-size 132x13 --overwrite -q \
  -c "scripts/video_session.sh new" demo/video/public/new.cast

make -s reset >/dev/null 2>&1
python3 scripts/trim_cast.py demo/video/public/old.cast
python3 scripts/trim_cast.py demo/video/public/new.cast
