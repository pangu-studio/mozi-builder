#!/usr/bin/env bash
set -euo pipefail
platform_root="$(cd "$(dirname "$0")/../.." && pwd)"
poc_arch="$(docker version --format '{{.Server.Arch}}')"
case "$poc_arch" in amd64|arm64) ;; *) echo "Unsupported Docker architecture: $poc_arch" >&2; exit 1;; esac
cd "$platform_root"
mkdir -p bin
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" CGO_ENABLED=0 GOOS=linux GOARCH="$poc_arch" go build -trimpath -o bin/mozi-poc ./cmd/poc
docker build -t mozi-v2-poc-probe:local -f deploy/poc/Dockerfile .
