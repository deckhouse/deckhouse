#!/usr/bin/env bash

KUBE_TOKEN=$(</var/run/secrets/kubernetes.io/serviceaccount/token)
KUBE_CA=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
KUBE_NS=$(</var/run/secrets/kubernetes.io/serviceaccount/namespace)

KUBE_CURL="curl -sS --cacert $KUBE_CA -H \"Authorization: Bearer $KUBE_TOKEN\" https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT_HTTPS"

. resty
resty https://$KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT_HTTPS -sS --cacert $KUBE_CA -H "Authorization: Bearer $KUBE_TOKEN"

cat <<EOF
  Usage:
\$ GET /api/v1/nodes - list all nodes in cluster
\$ GET /api/v1/namespace/\$KUBE_NS/pods - list pods in current namespace
\$ GET /apis/extensions/v1beta1/ingresses - list all inggreses in cluster

\$ GET /api/v1/nodes | jq '.["items"][] | { name: .metadata.name, labels: .metadata.labels }'
  - list all nodes with name and labels
EOF
