#!/bin/bash

source /antiopa/shell_lib.sh

function __config__() {
  echo '
{
  "afterHelm": 5
}'
}

function __main__() {
  all_ingresses=$(kubectl get ing --all-namespaces -o json | jq -r '.items[] |
                                                                      if .metadata.annotations == null then . |= (.metadata.annotations = {}) else . end |
                                                                      select(
                                                                        any(.metadata.annotations | to_entries[]; .key | startswith("ingress.kubernetes.io/"))
                                                                        and all(.metadata.annotations | to_entries[]; .key | startswith("nginx.ingress.kubernetes.io/") | not)
                                                                      )
                                                                      | ["-n \(.metadata.namespace) \(.metadata.name)"] | join(" ")')
  IFS=$'\n'
  for i in $all_ingresses; do
    unset IFS
    kubectl annotate ingress $i webhook.flant.com/trigger=
    kubectl annotate ingress $i webhook.flant.com/trigger-
  done
  unset IFS
}

hook::run "$@"
