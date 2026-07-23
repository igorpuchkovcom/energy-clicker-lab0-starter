#!/usr/bin/env bash
set -euo pipefail

shopt -s nullglob
for pid_file in /tmp/mirrorquest-lab1/*.pid; do
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" 2>/dev/null; then
    kill -TERM "$pid"
    echo "Stopped pid $pid"
  fi
  rm -f "$pid_file"
done
