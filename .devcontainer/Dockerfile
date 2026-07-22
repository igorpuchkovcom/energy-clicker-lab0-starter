
# Dev Container image for Energy Clicker Lab 0.
# Based on the official Microsoft Dev Containers Go image (Debian Bookworm, glibc).
# Go version matches go.mod (go 1.26).
FROM mcr.microsoft.com/devcontainers/go:1.26-bookworm

# System development tools. git, curl, bash and ca-certificates are already in
# the base image, but installing them here is idempotent and makes the list explicit.
RUN apt-get update \
    && export DEBIAN_FRONTEND=noninteractive \
    && apt-get install -y --no-install-recommends \
        git \
        curl \
        jq \
        make \
        bash \
        ca-certificates \
        postgresql-client \
        procps \
        iputils-ping \
        netcat-openbsd \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Go development tools. Installed into GOPATH/bin (/go/bin), which is on PATH
# for the vscode user. Each install is skipped if the tool already exists.
RUN command -v dlv >/dev/null 2>&1 \
    || go install github.com/go-delve/delve/cmd/dlv@latest
RUN command -v goimports >/dev/null 2>&1 \
    || go install golang.org/x/tools/cmd/goimports@latest
RUN command -v golangci-lint >/dev/null 2>&1 \
    || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
        | sh -s -- -b /go/bin

# Make sure the non-root vscode user owns GOPATH content installed above.
RUN chown -R vscode:golang /go
