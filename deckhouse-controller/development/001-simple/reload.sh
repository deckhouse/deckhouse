#!/usr/bin/env bash

cp ../../../shell_lib.sh .
cp -r ../../../shell_lib .
cp -r ../../../jq_lib .
cp -r ../../../helm_lib .

docker build -t "localhost:32500/deckhouse:test" .
docker push localhost:32500/deckhouse:test

#kubectl -n deckhouse-test replace --force -f deckhouse-deploy.yaml
