#!/bin/bash

echo "Create NS deckhouse-test"
kubectl create ns deckhouse-test
echo "Create RBAC, ConfigMap"
kubectl -n deckhouse-test apply -f deckhouse-rbac.yaml
kubectl -n deckhouse-test apply -f deckhouse-cm.yaml
kubectl -n deckhouse-test apply -f deckhouse-secret-reg.yaml
