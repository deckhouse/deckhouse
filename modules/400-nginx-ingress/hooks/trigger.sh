#!/bin/bash

source /antiopa/shell_lib.sh

function __config__() {
  echo '
{
  "afterHelm": 5
}'
}

function __main__() {
  if values::is_false nginxIngress.rewriteTargetMigration; then
    kill_list=$(kubectl get ing --all-namespaces -o json | jq -r '.items[] | "-n \(.metadata.namespace) \(.metadata.name)"' | grep -P '^.*-rwr$' | sort -u)
    IFS=$'\n'
    for i in $kill_list; do
      unset IFS
      kubectl delete ing $i
    done
    unset IFS
    exit 0
  fi

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

  non_rwr=$(kubectl get ing --all-namespaces -o json | jq -r '.items[] | "-n \(.metadata.namespace) \(.metadata.name)"' | (grep -Pv '^.*-rwr$' || true) | sort -u)
  rwr=$(kubectl get ing --all-namespaces -o json | jq -r '.items[] | "-n \(.metadata.namespace) \(.metadata.name)"' | (grep -P '^.*-rwr$' || true) | sed s/-rwr//g | sort -u)
  trigger_list=$(comm -23 <(echo "$non_rwr") <(echo "$rwr"))

  IFS=$'\n'
  for i in $trigger_list; do
    unset IFS
    kubectl annotate ingress $i webhook.flant.com/trigger= --overwrite
    kubectl annotate ingress $i webhook.flant.com/trigger-
  done
  unset IFS
}

hook::run "$@"
