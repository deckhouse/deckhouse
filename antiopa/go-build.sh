#!/bin/sh

set -e

export GO111MODULE=on

shellOpVer=$(go list -m all | grep shell-operator | cut -d' ' -f 2-)
addonOpVer=$(go list -m all | grep addon-operator | cut -d' ' -f 2-)
antiopaVer=$1

CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w -X 'main.AntiopaVersion=$antiopaVer' -X 'main.AddonOperatorVersion=$addonOpVer' -X 'main.ShellOperatorVersion=$shellOpVer'" -o antiopa github.com/deckhouse/deckhouse/antiopa
