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

# Check that we do not have new code after go generate
register_path="cmd/deckhouse-controller/register-go-hooks.go"
file_before=$(mktemp /tmp/register-before.XXXXX)
cp "$register_path" "$file_before"
go generate

if ! files_diff=$(diff "$file_before" "$register_path"); then
  >&2 echo "There are changes in the repo after running go:generate. Make sure that you have committed all changes."
  >&2 echo "$files_diff"
  exit 1
fi

CGO_ENABLED=1 \
    CGO_CFLAGS="-I${jqRoot}/include" \
    CGO_LDFLAGS="-L${jqRoot}/lib" \
    GOOS=linux \
    go build \
     -ldflags="-s -w -X 'main.DeckhouseVersion=$deckhouseVer' -X 'main.AddonOperatorVersion=$addonOpVer' -X 'main.ShellOperatorVersion=$shellOpVer'" \
     -o ./deckhouse-controller \
     ./cmd/deckhouse-controller
