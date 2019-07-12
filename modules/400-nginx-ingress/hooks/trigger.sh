#!/bin/bash

source /antiopa/shell_lib.sh

function __config__() {
  echo '
{
  "afterHelm": 5
}'
}

function __main__() {
  for i in $(seq 1 120); do
    if kubectl -n kube-system get pod -l app=ingress-conversion-webhook -o json | jq -e '.items[].status.conditions | select(.) | all(.[] ; .status == "True")' ; then
      break
    fi

    echo "Waiting for ingress conversion webhook pod in kube-system namespace"
    sleep 1
  done
  if [[ $i -gt 120 ]] ; then
    >&2 echo "Timeout waiting for ingress conversion webhook pod in kube-system namespace"
    return 1
  fi

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
