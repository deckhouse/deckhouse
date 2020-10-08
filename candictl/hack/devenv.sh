#!/bin/sh

set -e

echo -e "#1 Pull Docker Image\n==="
docker pull registry.flant.com/sys/antiopa/dev/install:master

echo -e "\n#2 Run Docker Container\n==="
id=$(docker ps -aqf "name=candictl-dev")
if [[ byName == "" ]]; then
  id=$(docker run \
     --name "candictl-dev" \
     --detach \
     --rm \
     -v $HOME/.ssh/:/root/.ssh/ \
     -v $(pwd)/../../candictl:/candictl \
     -v $(pwd)/../../candi:/deckhouse/candi \
     registry.flant.com/sys/antiopa/dev/install:master \
     tail -f /dev/null)
  echo "Run new container with ID: ${id}"
else
  echo "Container found: ${id}"
fi

echo -e "\n#3 Install dev dependencies\n==="
docker exec ${id} apk add go

echo -e "\n#4 Exec into Docker Container\n==="
docker exec -it ${id} bash
