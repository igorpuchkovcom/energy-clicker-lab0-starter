#!/usr/bin/env bash
set -euo pipefail

./scripts/start-lab1-mocks.sh
trap './scripts/stop-lab1-mocks.sh' EXIT

sleep 2

go run ./cmd/mirrorscout scan \
  --file ./configs/mirrors.local.json \
  --concurrency 3 \
  --timeout 1s
