#!/bin/sh
set -e
cd "$(dirname "$0")"
export CGO_ENABLED=0
go build -trimpath -ldflags="-s -w" -o webdownload .
echo Built native webdownload
