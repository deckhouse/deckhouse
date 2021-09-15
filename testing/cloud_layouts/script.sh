#!/bin/bash

# Copyright 2021 Flant CJSC
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

set -Eeo pipefail
shopt -s inherit_errexit
shopt -s failglob

test_failed=""

function abort_bootstrap_from_cache() {
    dhctl bootstrap-phase abort \
      --force-abort-from-cache \
      --config "$cwd/configuration.yaml" \
      --yes-i-am-sane-and-i-understand-what-i-am-doing

    return $?
}

function abort_bootstrap() {
  dhctl bootstrap-phase abort \
    --ssh-user "$ssh_user" \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --config "$cwd/configuration.yaml" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function destroy_cluster() {
  dhctl destroy \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --ssh-user "$ssh_user" \
    --ssh-host "$master_ip" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function cleanup() {
  cleanup_exit_code=0

  if [[ -z "$master_ip" ]]; then
     {
       abort_bootstrap || abort_bootstrap_from_cache
     } || cleanup_exit_code="$?"
  else
    {
      destroy_cluster || abort_bootstrap_from_cache
    } || cleanup_exit_code="$?"
  fi

  if [[ -n "$test_failed" ]]; then
    exit 1
  fi

  exit "$cleanup_exit_code"
}
trap cleanup EXIT

function fail_test() {
  test_failed="true"
}
trap fail_test ERR

root_wd="$(pwd)/testing/cloud_layouts"
cwd="$root_wd/$PROVIDER/$LAYOUT"
if [[ ! -d "$cwd" ]]; then
  >&2 echo "There is no cloud layout configuration by path: $cwd"
  exit 1
fi

ssh_private_key_path="$cwd/sshkey"
rm -f "$ssh_private_key_path"
base64 -d <<< "$SSH_KEY" > "$ssh_private_key_path"
chmod 0600 "$ssh_private_key_path"

if [[ -z "$KUBERNETES_VERSION" ]]; then
  # shellcheck disable=SC2016
  >&2 echo 'Provide ${KUBERNETES_VERSION}!'
  exit 1
fi

if [[ -z "$CRI" ]]; then
  # shellcheck disable=SC2016
  >&2 echo 'Provide ${CRI}!'
  exit 1
fi

if [[ -z "$DEV_BRANCH" ]]; then
  # shellcheck disable=SC2016
  >&2 echo 'Provide ${DEV_BRANCH}!'
  exit 1
fi
if [[ -z "$PREFIX" ]]; then
  # shellcheck disable=SC2016
  >&2 echo 'Provide ${PREFIX}!'
  exit 1
fi

if [[ "$PROVIDER" == "Yandex.Cloud" ]]; then
  # shellcheck disable=SC2016
  env CLOUD_ID="$(base64 -d <<< "$LAYOUT_YANDEX_CLOUD_ID")" FOLDER_ID="$(base64 -d <<< "$LAYOUT_YANDEX_FOLDER_ID")" \
      SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON")" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${CLOUD_ID} ${FOLDER_ID} ${SERVICE_ACCOUNT_JSON}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="ubuntu"
elif [[ "$PROVIDER" == "GCP" ]]; then
  # shellcheck disable=SC2016
  env SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON")" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${SERVICE_ACCOUNT_JSON}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="user"
elif [[ "$PROVIDER" == "AWS" ]]; then
  # shellcheck disable=SC2016
  env AWS_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_ACCESS_KEY")" AWS_SECRET_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_SECRET_ACCESS_KEY")" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${AWS_ACCESS_KEY} ${AWS_SECRET_ACCESS_KEY}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="ubuntu"
elif [[ "$PROVIDER" == "Azure" ]]; then
  # shellcheck disable=SC2016
  env SUBSCRIPTION_ID="$LAYOUT_AZURE_SUBSCRIPTION_ID" CLIENT_ID="$LAYOUT_AZURE_CLIENT_ID" \
      CLIENT_SECRET="$LAYOUT_AZURE_CLIENT_SECRET"  TENANT_ID="$LAYOUT_AZURE_TENANT_ID" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${TENANT_ID} ${CLIENT_SECRET} ${CLIENT_ID} ${SUBSCRIPTION_ID}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  ssh_user="azureuser"
elif [[ "$PROVIDER" == "OpenStack" ]]; then
  # shellcheck disable=SC2016
  env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${OS_PASSWORD}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"
  ssh_user="ubuntu"
elif [[ "$PROVIDER" == "vSphere" ]]; then
  # shellcheck disable=SC2016
  env VSPHERE_PASSWORD="$(base64 -d <<<"$LAYOUT_VSPHERE_PASSWORD")" \
      KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" VSPHERE_BASE_DOMAIN="$LAYOUT_VSPHERE_BASE_DOMAIN" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${VSPHERE_PASSWORD} ${VSPHERE_BASE_DOMAIN}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"
  ssh_user="ubuntu"
else
  >&2 echo "ERROR: Unknown provider \"$PROVIDER\""
  exit 1
fi

dhctl bootstrap --yes-i-want-to-drop-cache --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
--resources "$cwd/resources.yaml" --config "$cwd/configuration.yaml" | tee "$cwd/bootstrap.log"

# TODO: parse not the output of terraform, but last output of dhctl
if ! master_ip="$(grep -Po '(?<=master_ip_address_for_ssh = ).+$' "$cwd/bootstrap.log")"; then
  >&2 echo "ERROR: can't parse master_ip from bootstrap.log, attempting to abort bootstrap"
  test_failed="true"
  exit 1
fi

>&2 echo "Starting the process, if you'd like to pause the cluster deletion, ssh to cluster \"ssh $ssh_user@$master_ip\" and execute \"kubectl create configmap pause-the-test\""

>&2 echo 'Waiting until Machine provisioning finishes'
for ((i=0; i<10; i++)); do
  if ssh -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo -i /bin/bash <<"ENDSSH"; then
set -Eeuo pipefail
kubectl -n d8-cloud-instance-manager get machines
kubectl -n d8-cloud-instance-manager get machine -o json | jq -re '.items | length > 0' >/dev/null
kubectl -n d8-cloud-instance-manager get machines -o json|jq -re '.items | map(.status.currentStatus.phase == "Running") | all' >/dev/null
ENDSSH
    test_failed=""
    break
  else
    test_failed="true"
    >&2 echo "Machine provisioning is still in progress (attempt #$i of 10). Sleeping 60 seconds..."
    sleep 60
  fi
done

for ((i=0; i<3; i++)); do
  if ssh -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo -i /bin/bash <<"ENDSSH"; then
set -Eeuo pipefail

function cleanup_delay() {
  while true; do
    if ! { kubectl get configmap pause-the-test -o json | jq -re '.metadata.name == "pause-the-test"' >/dev/null ; }; then
      break
    fi

    >&2 echo 'Waiting until "kubectl delete cm pause-the-test" before destroying cluster'

    sleep 30
  done
}

trap cleanup_delay EXIT

for ((i=0; i<10; i++)); do
  smoke_mini_addr=$(kubectl -n d8-upmeter get ep smoke-mini -o json | jq -re '.subsets[].addresses[0] | .ip') && break
  >&2 echo "Attempt to get Endpoints for smoke-mini #$i failed. Sleeping 30 seconds..."
  sleep 30
done

if [[ -z "$smoke_mini_addr" ]]; then
  >&2 echo "Couldn't get smoke-mini's address from Endpoints in 15 minutes."
  exit 1
fi

if ! ingress_inlet=$(kubectl get ingressnginxcontrollers.deckhouse.io -o json | jq -re '.items[0] | .spec.inlet // empty'); then
  ingress="ok"
else
  ingress=""
fi

for ((i=0; i<10; i++)); do
  for path in api disk dns prometheus; do
    result="$(curl -m 5 -sS "${smoke_mini_addr}:8080/${path}")"
    printf -v "$path" "%s" "$result"
  done

  cat <<EOF
Kubernetes API check: $([ "$api" == "ok" ] && echo "success" || echo "failure")
Disk check: $([ "$disk" == "ok" ] && echo "success" || echo "failure")
DNS check: $([ "$dns" == "ok" ] && echo "success" || echo "failure")
Prometheus check: $([ "$prometheus" == "ok" ] && echo "success" || echo "failure")
EOF

  if [[ -n "$ingress_inlet" ]]; then
    if [[ "$ingress_inlet" == "LoadBalancer" ]]; then
      if ingress_service="$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -ojson 2>/dev/null)"; then
        if ingress_lb="$(jq -re '.status.loadBalancer.ingress[0].hostname' <<< "$ingress_service")"; then
          if ingress_lb_code="$(curl -o /dev/null -s -w "%{http_code}" "$ingress_lb")"; then
            if [[ "$ingress_lb_code" == "404" ]]; then
              ingress="ok"
            else
              >&2 echo "Got code $ingress_lb_code from LB $ingress_lb, waiting for 404."
            fi
          else
            >&2 echo "Failed curl request to the LB hostname: $ingress_lb."
          fi
        else
          >&2 echo "Can't get svc/nginx-load-balancer LB hostname."
        fi
      else
        >&2 echo "Can't get svc/nginx-load-balancer."
      fi
    else
      >&2 echo "Ingress controller with inlet $ingress_inlet found in the cluster. But I have no instructions how to test it."
      exit 1
    fi

    cat <<EOF
Ingress $ingress_inlet check: $([ "$ingress" == "ok" ] && echo "success" || echo "failure")
EOF
  fi

  if [[ "$api" == "ok" && "$disk" == "ok" && "$dns" == "ok" && "$prometheus" == "ok" && "$ingress" == "ok" ]]; then
    exit 0
  fi

  sleep 30
done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1
ENDSSH
    test_failed=""
    break
  else
    test_failed="true"

    >&2 echo "SSH #$i failed. Sleeping 30 seconds..."
    sleep 30
  fi
done

ssh -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo -i /bin/bash > "$cwd/deckhouse.json.log" <<"ENDSSH"
kubectl -n d8-system logs deploy/deckhouse
ENDSSH

exit 0
