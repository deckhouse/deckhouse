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

function cleanup() {
  cleanup_exit_code=0
  pushd "$cwd"
  terraform destroy -input=false -auto-approve
  popd
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
cwd="$root_wd/$LAYOUT"
if [[ ! -d "$cwd" ]]; then
  >&2 echo "There is no static layout configuration by path: $cwd"
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

# shellcheck disable=SC2016
env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
    KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
    envsubst '$DECKHOUSE_DOCKERCFG $PREFIX $DEV_BRANCH $KUBERNETES_VERSION $CRI $OS_PASSWORD' \
    <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"
# shellcheck disable=SC2016
env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" PREFIX="$PREFIX" \
    envsubst '$PREFIX $OS_PASSWORD' \
    <"$cwd/infra.tpl.tf" >"$cwd/infra.tf"
rm -f "$cwd/infra.tpl.tf"
ssh_user="ubuntu"

# Terraform master
pushd "$cwd"
terraform init -input=false -plugin-dir=/usr/local/share/terraform/plugins
terraform apply -auto-approve -no-color | tee "$cwd/terraform.log"
popd

master_ip="$(grep "master_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d " ")"
system_ip="$(grep "system_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d " ")"

# Bootstrap
# TODO --resources "$cwd/resources.yaml" dont't work is static clusters !!!!
dhctl bootstrap --yes-i-want-to-drop-cache --ssh-host "$master_ip" --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
--config "$cwd/configuration.yaml" --resources "$cwd/resources.yaml" | tee "$cwd/bootstrap.log"

>&2 echo "Starting the process, if you'd like to pause the cluster deletion, ssh to cluster \"ssh $ssh_user@$master_ip\" and execute \"kubectl create configmap pause-the-test\""

>&2 echo 'Creating resources'
scp -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$cwd/resources.yaml" "$ssh_user@$master_ip":/tmp/resources.yaml
ssh -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo -i /bin/bash <<"ENDSSH"
set -Eeuo pipefail
kubectl create -f /tmp/resources.yaml
ENDSSH

for ((i=0; i<10; i++)); do
  bootstrap_system="$(ssh -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo -i /bin/bash << "ENDSSH"
set -Eeuo pipefail
kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-system -o json | jq -r '.data."bootstrap.sh"'
ENDSSH
)" && break
  >&2 echo "Attempt to get secret manual-bootstrap-for-system in d8-cloud-instance-manager namespace #$i failed. Sleeping 30 seconds..."
  sleep 30
done

if [[ -z "$bootstrap_system" ]]; then
  >&2 echo "Couldn't get secret manual-bootstrap-for-system in d8-cloud-instance-manager namespace."
  exit 1
fi

# shellcheck disable=SC2087
# Node reboots in bootstrap process, so ssh exits with error code 255. It's normal, so we use || true to avoid script fail.
ssh -o GlobalKnownHostsFile=/dev/null -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i "$ssh_private_key_path" "$ssh_user@$system_ip" sudo -i /bin/bash <<ENDSSH || true
set -Eeuo pipefail
base64 -d <<< "$bootstrap_system" | bash
ENDSSH

>&2 echo 'Waiting 10 minutes until Machine provisioning finishes'
sleep 600

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
fi

for ((i=0; i<10; i++)); do
  for path in api disk dns prometheus; do
    # if any path unaccessible, curl returns error exit code, and script fails, so we use || true to avoid script fail.
    result="$(curl -m 5 -sS "${smoke_mini_addr}:8080/${path}")" || true
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
      ingress_lb="$(kubectl -n d8-ingress-nginx get svc nginx-load-balancer -ojson | jq -re '.status.loadBalancer.ingress[0].hostname')"
      if [[ -n "$ingress_lb" ]]; then
        ingress_lb_code="$(curl -o /dev/null -s -w "%{http_code}" "$ingress_lb")"
        if [[ "$ingress_lb_code" == "404" ]]; then
          ingress="ok"
        else
          >&2 echo "Got code $ingress_lb_code from LB $ingress_lb, waiting for 404."
        fi
      else
        >&2 echo "Can't get svc/nginx-load-balancer LB hostname."
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
