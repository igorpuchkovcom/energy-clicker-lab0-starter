#!/usr/bin/env bash
# Runs once after the Dev Container is created (postCreateCommand).
# Safe to re-run: it only syncs dependencies and installs missing tools,
# it does not modify sources and does not leave the app running.
set -euo pipefail

echo "==> marking workspace as a safe git directory"
git config --global --add safe.directory /workspaces/energy-clicker

echo "==> go version"
go version

echo "==> go mod download"
go mod download

echo "==> ensuring Go tools are installed"
command -v dlv >/dev/null 2>&1 || go install github.com/go-delve/delve/cmd/dlv@latest
command -v goimports >/dev/null 2>&1 || go install golang.org/x/tools/cmd/goimports@latest
command -v golangci-lint >/dev/null 2>&1 \
    || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
        | sh -s -- -b "$(go env GOPATH)/bin"

echo "==> go test ./..."
go test ./...

echo "==> Dev Container is ready. Start the API with: go run ./cmd/server"
