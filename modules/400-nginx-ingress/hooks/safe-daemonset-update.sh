#!/bin/bash

set -e

source /antiopa/shell_lib.sh

function __config__() {
  cat << EOF
{
  "onKubernetesEvent": [
    {
      "kind": "daemonset",
      "event": ["update"],
      "selector": {
        "matchLabels": {
          "nginx-ingress-safe-update": ""
        }
      },
      "jqFilter": ".status",
      "allowFailure": false
    }
  ]
}
EOF
}

function delete_pod_in_ds() {
  namespace=$1
  daemonset_name=$2

  generation=$(kubectl -n $namespace get daemonset $daemonset_name -o json | jq -r .spec.templateGeneration)
  pod_to_kill=$(kubectl -n $namespace get pods -l"app=${daemonset_name},pod-template-generation!=${generation}" -o json | jq -er '.items[0].metadata.name')
  kubectl -n $namespace delete pod $pod_to_kill
}

function __main__() {
  namespace=$(hook::context_jq -r '.[0].resourceNamespace')
  a_name=$(hook::context_jq -r '.[0].resourceName')

  if [[ "$a_name" == "nginx" ]]; then
    b_name=direct-fallback
  else
    b_name=nginx
  fi

  IFS=";" read -r -a a_status <<< $(
    kubectl -n $namespace get daemonset $a_name -o json |\
    jq -r '.status |
      {"need": ((.updatedNumberScheduled // 0) < .desiredNumberScheduled), "ready": (.numberReady == .currentNumberScheduled)} |
      "\(.need);\(.ready)"'
  )
  IFS=";" read -r -a b_status <<< $(
    kubectl -n $namespace get daemonset $b_name -o json |\
    jq -r '.status |
      {"need": ((.updatedNumberScheduled // 0) < .desiredNumberScheduled), "ready": (.numberReady == .currentNumberScheduled)} |
      "\(.need);\(.ready)"'
  )

  a_need_update="${a_status[0]}"
  a_ready="${a_status[1]}"
  b_need_update="${b_status[0]}"
  b_ready="${b_status[1]}"

  if [[ "${a_need_update}" == "true" && "${a_ready}" == "true" && "${b_ready}" == "true" ]]; then
    delete_pod_in_ds $namespace $a_name
  elif [[ "${a_need_update}" == "false" && "${a_ready}" == "true" && "${b_need_update}" == "true" && "$b_ready" == "true" ]]; then
    delete_pod_in_ds $namespace $b_name
  fi
}

hook::run "$@"
