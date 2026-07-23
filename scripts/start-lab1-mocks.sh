#!/usr/bin/env bash
set -euo pipefail

mkdir -p /tmp/mirrorquest-lab1

start() {
  local name="$1"
  local port="$2"
  local mode="$3"
  local delay="${4:-800ms}"

  MIRROR_ADDR=":$port" \
  MIRROR_MODE="$mode" \
  MIRROR_DELAY="$delay" \
    go run ./cmd/mockmirror \
      >"/tmp/mirrorquest-lab1/$name.log" 2>&1 &

  echo $! >"/tmp/mirrorquest-lab1/$name.pid"
  echo "Started $name on :$port, mode=$mode, pid=$!"
}

start good       18081 good
start health500  18082 health500
start notready   18083 notready
start hanging    18084 hanging
start slow       18085 slow 800ms

echo
echo "Logs: /tmp/mirrorquest-lab1/*.log"
echo "Stop: ./scripts/stop-lab1-mocks.sh"
