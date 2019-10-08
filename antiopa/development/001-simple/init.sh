#!/bin/bash

echo "Create NS antiopa-test"
kubectl create ns antiopa-test
echo "Create RBAC, ConfigMap"
kubectl -n antiopa-test apply -f antiopa-rbac.yaml
kubectl -n antiopa-test apply -f antiopa-cm.yaml
kubectl -n antiopa-test apply -f antiopa-secret-reg.yaml
