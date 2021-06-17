#!/bin/sh

set -e

deckhouseVer="dev"
shellOpVer=$(go list -m all | grep shell-operator | cut -d' ' -f 2-)
addonOpVer=$(go list -m all | grep addon-operator | cut -d' ' -f 2-)

jqRoot=$1
if [ "x${jqRoot}" = "x" ]; then
  >&2 echo "A path to libjq static libraries should be specified!"
  exit 1
fi

# Can be removed when Go 1.16 will be in use.
export GO111MODULE=on

CGO_ENABLED=1 \
    CGO_CFLAGS="-I${jqRoot}/include" \
    CGO_LDFLAGS="-L${jqRoot}/lib" \
    GOOS=linux \
    go build \
     -ldflags="-s -w -X 'main.DeckhouseVersion=$deckhouseVer' -X 'main.AddonOperatorVersion=$addonOpVer' -X 'main.ShellOperatorVersion=$shellOpVer'" \
     -o ./deckhouse-controller \
     ./cmd/deckhouse-controller
