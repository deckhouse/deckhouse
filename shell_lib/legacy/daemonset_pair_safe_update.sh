#!/bin/bash

function legacy::common_hooks::daemonset_pair_safe_update::delete_all_not_updated_crashloopback_pods_in_ds() {
  namespace=$1
  daemonset_name=$2

  selector=$(kubectl -n $namespace get daemonset $daemonset_name -o json | jq -r '
    [
      .spec.selector.matchLabels | to_entries[] | .key+"="+.value
    ] +
    [
      "pod-template-generation!=" + ((.spec.templateGeneration // .metadata.annotations."deprecated.daemonset.template.generation" // .metadata.generation) | tostring)
    ] | join(",")'
  )

  if pods_to_kill=$(kubectl -n $namespace get pods -l"${selector}" -o json | jq -er '.items[] |
  select(
    (.status.containerStatuses | reduce .[] as $i (false; . or ($i.state.waiting.reason == "CrashLoopBackOff") or ($i.state.terminated.reason == "Error")))
  ) | .metadata.name' | xargs)
  then
    kubectl -n "${namespace}" delete pod ${pods_to_kill}
  fi
}

function legacy::common_hooks::daemonset_pair_safe_update::delete_pod_in_ds() {
  namespace=$1
  daemonset_name=$2

  selector=$(kubectl -n $namespace get daemonset $daemonset_name -o json | jq -r '
    [
      .spec.selector.matchLabels | to_entries[] | .key+"="+.value
    ] +
    [
      "pod-template-generation!=" + ((.spec.templateGeneration // .metadata.annotations."deprecated.daemonset.template.generation" // .metadata.generation) | tostring)
    ] | join(",")'
  )
  if pod_to_kill=$(kubectl -n $namespace get pods -l"${selector}" -o json | jq -er '.items[0].metadata.name')
  then
    kubectl -n $namespace delete pod $pod_to_kill
  fi
}

# $1 — основной daemonset, $2 — вспомогательный
function legacy::common_hooks::daemonset_pair_safe_update::main() {
  namespace=$(context::get object.metadata.namespace)
  a_name=$(context::get object.metadata.name)

  if [[ "$a_name" == "${1}" ]]; then
    b_name=${2}
  else
    b_name=${1}
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
    legacy::common_hooks::daemonset_pair_safe_update::delete_pod_in_ds $namespace $a_name
  elif [[ "${b_need_update}" == "true" && "${b_ready}" == "true" && "${a_ready}" == "true" ]]; then
    legacy::common_hooks::daemonset_pair_safe_update::delete_pod_in_ds $namespace $b_name
  fi

  legacy::common_hooks::daemonset_pair_safe_update::delete_all_not_updated_crashloopback_pods_in_ds $namespace $a_name
  legacy::common_hooks::daemonset_pair_safe_update::delete_all_not_updated_crashloopback_pods_in_ds $namespace $b_name
}
