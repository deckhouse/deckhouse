#!/bin/bash

source /antiopa/shell_lib.sh

function __config__() {
  echo '
{
  "beforeHelm": 10,
  "onKubernetesEvent": [
    {
      "kind": "ingress",
      "event": [
        "delete"
      ]
    }
  ]
}'
}

function __main__() {
  if hook::context_jq -re 'any(.[]; .binding=="onKubernetesEvent")' 2>/dev/null 1>&2; then
    for resource_namespace in $(hook::context_jq -r '.[] | select(.binding == "onKubernetesEvent") | .resourceNamespace'); do
      for resource_name in $(hook::context_jq -r ".[] | select(.resourceNamespace==\"$resource_namespace\") | .resourceName"); do
        if [[ $resource_name == *-rwr ]]; then
          continue
        fi
        kubectl -n "$resource_namespace" delete ingress "${resource_name}-rwr" 2>/dev/null 1>&2 || true
      done
    done
  fi

  if hook::context_jq -re 'any(.[]; .binding=="beforeHelm")' 2>/dev/null 1>&2; then
    non_rwr=$(kubectl get ing --all-namespaces -o json | jq -r '.items[] | "-n \(.metadata.namespace) \(.metadata.name)"' | (grep -Pv '^.*-rwr$' || true) | sort -u)
    rwr=$(kubectl get ing --all-namespaces -o json | jq -r '.items[] | "-n \(.metadata.namespace) \(.metadata.name)"' | (grep -P '^.*-rwr$' || true) | sed s/-rwr//g | sort -u)
    kill_list=$(comm -13 <(echo "$non_rwr") <(echo "$rwr"))

    IFS=$'\n'
    for i in $kill_list; do
      unset IFS
      kubectl delete ingress $i-rwr
    done
    unset IFS
  fi
}

hook::run "$@"
