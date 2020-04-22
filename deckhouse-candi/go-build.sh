#!/bin/sh

set -e

GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" ./cmd/deckhouse-candi
