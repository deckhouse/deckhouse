#!/usr/bin/env bash

(
echo "Rebuild antiopa binary"
cd ../..

# get versions
shellOpVer=$(go list -m all | grep shell-operator | cut -d' ' -f 2-)
addonOpVer=$(go list -m all | grep addon-operator | cut -d' ' -f 2-)
antiopaVer=$(git rev-parse --abbrev-ref HEAD):$(git rev-parse --short HEAD)$(git diff --quiet || echo ':dirty'):$(date +'%Y.%m.%d_%H:%M:%S')


CGO_ENABLED=0 GOOS=linux go build -tags='release' -ldflags="-s -w -X 'main.AntiopaVersion=$antiopaVer' -X 'main.AddonOperatorVersion=$addonOpVer' -X 'main.ShellOperatorVersion=$shellOpVer'" -o antiopa-test ./cmd/antiopa


res=$?
if [[ $res != 0 ]] ; then
  echo "go build error: $res"
  exit 1
fi
) || exit 1

cp ../../antiopa-test antiopa

./reload.sh
