#!/usr/bin/env bash

cp ../../../shell_lib.sh .
cp -r ../../../shell_lib .
cp -r ../../../jq_lib .
cp -r ../../../helm_lib .

docker build -t "localhost:32500/antiopa:test" .
docker push localhost:32500/antiopa:test

#kubectl -n antiopa-test replace --force -f antiopa-deploy.yaml
