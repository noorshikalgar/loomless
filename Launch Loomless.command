#!/bin/bash
# Double-click this file in Finder to start Loomless — no terminal
# knowledge needed. macOS opens a Terminal window automatically to run
# any executable .command file; this one just cd's next to itself (so
# it works regardless of where the folder is on disk) and starts the
# dashboard, which opens your browser to it on its own.
#
# Requires the compiled "loomless" binary in this same folder — see
# INSTALL.md for how to build one, or grab a prebuilt release.
cd "$(dirname "$0")"

if [ ! -x ./loomless ]; then
  echo "Can't find a 'loomless' binary next to this launcher."
  echo "Build one first: go build -o loomless ./cmd/loomless"
  echo "(see INSTALL.md), then double-click this file again."
  read -n 1 -s -r -p "Press any key to close..."
  exit 1
fi

./loomless serve
