#!/bin/bash

dapp kube minikube setup

cat > /tmp/tiller-rbac-config.yaml << END
apiVersion: v1
kind: ServiceAccount
metadata:
  name: tiller
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: tiller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: tiller
    namespace: kube-system
END
kubectl create -f /tmp/tiller-rbac-config.yaml

helm reset
helm init --service-account tiller
