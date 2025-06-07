# Copyright 2025 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail

deployment_name="autoscaler-test"
should_nodes_in_cluster="0"
nodes_during_scaling="1"

autoscaler_nodes=""
autoscaler_nodes_count=""


function log_autoscaler() {
  echo "Sleep 2 minutes for collecting errors and warnings logs"
  sleep 120

  echo "Cluster-autoscaler warning logs:"
  kubectl -n d8-cloud-instance-manager logs deployments/cluster-autoscaler | grep -e "^W" || true

  echo "Cluster-autoscaler error logs:"
  kubectl -n d8-cloud-instance-manager logs deployments/cluster-autoscaler | grep -e "^E" || true

  return 0
}

function create_deployment() {
    local attempts=10
    local ret_apply

    for i in $(seq $attempts); do
      ret_apply=0
      kubectl apply -f - <<EOD || ret_apply=$?
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $deployment_name
  labels:
    app: $deployment_name
spec:
  replicas: 1
  selector:
    matchLabels:
      app: $deployment_name
  template:
    metadata:
      labels:
        app: $deployment_name
    spec:
      containers:
      - name: app
        image: registry.deckhouse.io/base_images@sha256:05fb7868d518fe6c562233e1ee1c9304f6d5142920959e7b2d51acdc49cce0c3
        command: ["python3"]
        args: ["-m", "http.server", "8080"]
        imagePullPolicy: IfNotPresent
        resources:
          requests:
            memory: "2048Mi"
            cpu: "1"
      nodeSelector:
        node-role/autoscaler: ""
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: $deployment_name
            topologyKey: kubernetes.io/hostname
      tolerations:
      - key: node
        operator: Equal
        value: autoscaler
EOD

      if [[ "$ret_apply" == "0" ]]; then
        echo "Deployment 'autoscaler-test' was created!"
        return 0
      fi

      >&2 echo "Deployment 'autoscaler-test' did not create!. Attempt $i/$attempts failed. Sleeping 10 seconds..."
      sleep 10
    done

    echo "Deployment 'autoscaler-test' creating timeout exited. Exit"
    return 1
}

function wait_deployment_become_ready() {
  # 15 minutes
  local attempts=90
  local available_replicas

  for i in $(seq $attempts); do
    available_replicas="$(kubectl get deployment "$deployment_name" -o json | jq '.status.availableReplicas // empty')"
    if [[ "$available_replicas" == "1" ]]; then
      echo "Deployment 'autoscaler-test' is ready!"
      return 0
    fi

    >&2 echo "Deployment 'autoscaler-test' is not ready!. Attempt $i/$attempts. Sleeping 10 seconds..."
    sleep 10
  done

  echo "Deployment 'autoscaler-test' become ready timeout exited. Exit"
  log_autoscaler
  return 1
}

function scale_down_deployment() {
  local attempts=10
  local ret_down

  for i in $(seq $attempts); do
    ret_down=0
    kubectl scale --replicas=0 deployment "$deployment_name" || ret_down=$?

    if [[ "$ret_down" == "0" ]]; then
      echo "Deployment 'autoscaler-test' scaled down!"
      return 0
    fi

    >&2 echo "Deployment 'autoscaler-test' did not scale down!. Attempt $i/$attempts failed. Sleeping 10 seconds..."
    sleep 10
  done

  echo "Deployment 'autoscaler-test' scaling down timeout exited. Exit"
  return 1
}

function save_autoscaler_nodes() {
  local attempts=10

  for i in $(seq $attempts); do
    autoscaler_nodes="$(kubectl get no -l node-role/autoscaler="" -o json | jq -rM '.items[].metadata.name')"
    autoscaler_nodes_count="$(echo -n "$autoscaler_nodes" | awk 'END {print NR}')"
    if [[ "$autoscaler_nodes_count" == "$nodes_during_scaling" ]]; then
      echo "Nodes names saved successful: $autoscaler_nodes"
      return 0
    fi

    >&2 echo "Cannot save autoscaler nodes names $autoscaler_nodes. Attempt $i/$attempts. Sleeping 10 seconds..."
    sleep 10
  done

  echo "Waiting saving autoscaler nodes names timeout exited. Exit"
  log_autoscaler
  return 1
}

function wait_become_autoscaler_nodes_delete() {
  # 25 minutes
  local attempts=150
  local autoscaler_nodes_in_cluster

  for i in $(seq $attempts); do
    autoscaler_nodes_in_cluster="$(kubectl get no -l node-role/autoscaler="" -o json | jq --raw-output '.items | length')"
    if [[ "$autoscaler_nodes_in_cluster" == "$should_nodes_in_cluster" ]]; then
      echo "Nodes in autoscaler ng scaled down"
      return 0
    fi

    >&2 echo "Cluster has $autoscaler_nodes_in_cluster autoscaler nodes! Waiting scale down nodes in node group autoscaler to ${should_nodes_in_cluster}. Attempt $i/$attempts. Sleeping 10 seconds..."
    sleep 10
  done

  echo "Waiting scale down nodes in node group autoscaler to $should_nodes_in_cluster timeout exited. Exit"
  log_autoscaler
  return 1
}

function wait_become_autoscaler_instances_delete() {
  # 15 minutes
  local attempts=90
  local autoscaler_nodes_in_cluster

  for i in $(seq $attempts); do
    autoscaler_nodes_in_cluster="$(kubectl get instances -l node.deckhouse.io/group=autoscaler -o json | jq --raw-output '.items | length')"
    if [[ "$autoscaler_nodes_in_cluster" == "$should_nodes_in_cluster" ]]; then
      echo "Instances in autoscaler ng scaled down!"
      return 0
    fi

    >&2 echo "Cluster has $autoscaler_nodes_in_cluster autoscaler nodes! Waiting scale down nodes in node group autoscaler to ${should_nodes_in_cluster}. Attempt $i/$attempts. Sleeping 10 seconds..."
    sleep 10
  done

  echo "Waiting scale down nodes in node group autoscaler to $should_nodes_in_cluster timeout exited. Exit"
  log_autoscaler
  return 1
}

function check_cordon_events() {
  cordon_events="$1"
  echo "Cordon events:"
  echo "$cordon_events"
  echo ""

  local captured_events
  captured_events=0

  for n in $autoscaler_nodes; do
    local cordon_events_for_node
    local cordon_events_count

    cordon_events_for_node="$(echo "$cordon_events" | { grep -i "$n" || true; } )"
    cordon_events_count="$(echo -n "$cordon_events_for_node" | awk 'END {print NR}')"

    if [[ "$cordon_events_count" == "1" ]]; then
      echo "Node $n cordoned before deleting!"
      ((captured_events+=1))
    else
      echo "Cordon events for node $n not found"
      break
    fi
  done

  if [[ "$captured_events" == "$autoscaler_nodes_count" ]]; then
    echo "All nodes cordoned before deleting!"
    return 0
  fi

  >&2 echo "Cluster has $captured_events cordon events for nodes $autoscaler_nodes, should be ${autoscaler_nodes_count}."

  return 1
}

function verify_that_nodes_were_cordoned() {
  local attempts=10
  local cordon_events

  for i in $(seq $attempts); do
    cordon_events="$(kubectl get events --sort-by metadata.creationTimestamp | { grep -i "NodeNotSchedulable" || true; } )"
    if check_cordon_events "$cordon_events"; then
        return 0
    else
      # During testing,
      # we encountered the fact that events about the cordon node are not always set by kubernetes,
      # but at the same time we have events from the autoscaler. We check them
      cordon_events="$(kubectl get events --sort-by metadata.creationTimestamp | { grep -i "marked the node as toBeDeleted/unschedulable" || true; } )"

      if check_cordon_events "$cordon_events"; then
        return 0
      fi
    fi

    echo "Waiting get cordon events from cluster. Attempt $i/$attempts. Sleeping 10 seconds..."
    sleep 10
  done

  echo "Waiting cordon events for deleted node to $nodes_during_scaling timeout exited. Exit."
  log_autoscaler
  return 1
}

function verify_that_nodes_were_drained() {
  local attempts=10
  local drain_events

  for i in $(seq $attempts); do
    drain_events="$(kubectl -n d8-cloud-instance-manager get events --sort-by metadata.creationTimestamp | { grep -i "SuccessfulDrainNode" || true; } )"

    echo "Drain events:"
    echo "$drain_events"
    echo ""

    local captured_events
    captured_events=0

    for n in $autoscaler_nodes; do
      local drain_events_for_node
      local drain_events_count

      drain_events_for_node="$(echo "$drain_events" | { grep -i "$n" || true; } )"
      drain_events_count="$(echo -n "$drain_events_for_node" | awk 'END {print NR}')"

      if [[ "$drain_events_count" == "1" ]]; then
        echo "Node $n drained before deleting!"
        ((captured_events+=1))
      else
        echo "Drain events for node $n not found"
        break
      fi
    done

    if [[ "$captured_events" == "$autoscaler_nodes_count" ]]; then
      echo "All nodes drained before deleting!"
      return 0
    fi

    >&2 echo "Cluster has $captured_events drain events, should be ${autoscaler_nodes_count}. Waiting get drain events from cluster. Attempt $i/$attempts. Sleeping 10 seconds..."
    sleep 10
  done

  echo "Waiting drain events for deleted node to $nodes_during_scaling timeout exited. Exit."
  log_autoscaler
  return 1
}


create_deployment
wait_deployment_become_ready
save_autoscaler_nodes
scale_down_deployment
wait_become_autoscaler_nodes_delete
wait_become_autoscaler_instances_delete
verify_that_nodes_were_cordoned
verify_that_nodes_were_drained
log_autoscaler

echo "Autoscaler test was processed!"

exit 0
