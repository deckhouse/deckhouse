#!/bin/bash

set -e

echo -e "#1 Pull Docker Image\n==="
docker pull registry.flant.com/sys/antiopa/dev/install:master

echo -e "\n#2 Run Docker Container\n==="
id=$(docker ps -aqf "name=dhctl-dev")
if [[ "x$id" == "x" ]]; then
  id=$(docker run \
     --name "dhctl-dev" \
     --detach \
     --rm \
     -v $HOME/.ssh/:/root/.ssh/ \
     --mount type=tmpfs,destination=/tmp:exec \
     -v "$(pwd)/../../dhctl:/dhctl" \
     -v "$(pwd)/../../candi:/deckhouse/candi" \
     registry.flant.com/sys/antiopa/dev/install:master \
     tail -f /dev/null)

  echo -e "\n#3 Install dev dependencies\n==="

  docker exec "$id" apk add go curl
  # install linter
  docker exec -i "$id" sh -c 'curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)"/bin v1.32.2'
  docker exec "$id" sh -c 'ln -fs /root/go/bin/golangci-lint /usr/local/bin/golangci-lint'

  echo "Run new container with ID: ${id}"
else
  echo "Container found: ${id}"
fi


echo -e "\n#4 Exec into Docker Container\n==="
docker exec -it  -w /dhctl/hack/ "$id" bash
