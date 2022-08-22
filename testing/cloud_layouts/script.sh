#!/bin/bash

# Copyright 2021 Flant JSC
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

usage=$(cat <<EOF
Usage:
  ./script.sh [command]

Commands:

  run-test       Create cluster and install Deckhouse
                 using dhctl.

  cleanup        Delete cluster.

  <no-command>   Create cluster, install Deckhouse and delete cluster
                 if no command specified (execute run-test + cluster).

Required environment variables:

Name                  Description
---------------------+---------------------------------------------------------
$PROVIDER             An infrastructure provider: AWS, GCP, Azure, OpenStack,
                      Static, vSphere or Yandex.Cloud.
                      See them in the cloud_layout directory.
$LAYOUT               Layout for provider: WithoutNAT, Standard or Static.
                      See available layouts inside the provider directory.
$PREFIX               A unique prefix to run several tests simultaneously.
$KUBERNETES_VERSION   A version of Kubernetes to install.
$CRI                  Docker or Containerd.
$DECKHOUSE_DOCKERCFG  Base64 encoded docker registry credentials.
$DECKHOUSE_IMAGE_TAG  An image tag for deckhouse Deployment. A Git tag to
                      test prerelease and release images or pr<NUM> slug
                      to test changes in pull requests.
$INITIAL_IMAGE_TAG    An image tag for Deckhouse deployment to
                      install first and then switching to DECKHOUSE_IMAGE_TAG.
                      Also, run test suite for these 2 versions.

Provider specific environment variables:

  Yandex.Cloud:

$LAYOUT_YANDEX_CLOUD_ID
$LAYOUT_YANDEX_FOLDER_ID
$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON

  GCP:

$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON

  AWS:

$LAYOUT_AWS_ACCESS_KEY
$LAYOUT_AWS_SECRET_ACCESS_KEY

  Azure:

$LAYOUT_AZURE_SUBSCRIPTION_ID
$LAYOUT_AZURE_TENANT_ID
$LAYOUT_AZURE_CLIENT_ID
$LAYOUT_AZURE_CLIENT_SECRET

  Openstack:

$LAYOUT_OS_PASSWORD

  vSphere:

$LAYOUT_VSPHERE_PASSWORD

  Static:

$LAYOUT_OS_PASSWORD

EOF
)

set -Eeo pipefail
shopt -s inherit_errexit
shopt -s failglob

# Image tag to install.
DEV_BRANCH=
# Image tag to switch to if initial_image_tag is set.
SWITCH_TO_IMAGE_TAG=
# ssh command with common args.
ssh_command="ssh -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o LogLevel=quiet"

# Path to private SSH key to connect to cluster after bootstrap
ssh_private_key_path=
# User for SSH connect.
ssh_user=
# IP of master node.
master_ip=

function abort_bootstrap_from_cache() {
  >&2 echo "Run abort_bootstrap_from_cache"
  dhctl bootstrap-phase abort \
    --force-abort-from-cache \
    --config "$cwd/configuration.yaml" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function abort_bootstrap() {
  >&2 echo "Run abort_bootstrap"
  dhctl bootstrap-phase abort \
    --ssh-user "$ssh_user" \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --config "$cwd/configuration.yaml" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function destroy_cluster() {
  >&2 echo "Run destroy_cluster"
  dhctl destroy \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --ssh-user "$ssh_user" \
    --ssh-host "$master_ip" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function destroy_static_infra() {
  >&2 echo "Run destroy_static_infra"

  pushd "$cwd"
  terraform destroy -input=false -auto-approve || exitCode=$?
  popd

  return $exitCode
}

function cleanup() {
  if [[ "$PROVIDER" == "Static" ]]; then
    >&2 echo "Run cleanup ... destroy terraform infra"
    destroy_static_infra || exitCode=$?
    return $exitCode
  fi

  # Check if 'dhctl bootstrap' was not started.
  if [[ ! -f "$cwd/bootstrap.log" ]] ; then
    >&2 echo "Run cleanup ... no bootstrap.log, no need to cleanup."
    return 0
  fi

  >&2 echo "Run cleanup ..."
  if ! master_ip="$(parse_master_ip_from_log)" ; then
    >&2 echo "No master IP: try to abort without cache, then abort from cache"
    abort_bootstrap || abort_bootstrap_from_cache
  else
    >&2 echo "Master IP is '${master_ip}': try to destroy cluster, then abort from cache"
    destroy_cluster || abort_bootstrap_from_cache
  fi
}

function prepare_environment() {
  root_wd="$(pwd)/testing/cloud_layouts"

  if [[ -z "$PROVIDER" || ! -d "$root_wd/$PROVIDER" ]]; then
    >&2 echo "ERROR: Unknown provider \"$PROVIDER\""
    return 1
  fi

  cwd="$root_wd/$PROVIDER/$LAYOUT"
  if [[ "$PROVIDER" == "Static" ]]; then
    cwd="$root_wd/$PROVIDER"
  fi
  if [[ ! -d "$cwd" ]]; then
    >&2 echo "There is no '${LAYOUT}' layout configuration for '${PROVIDER}' provider by path: $cwd"
    return 1
  fi

  ssh_private_key_path="$cwd/sshkey"
  rm -f "$ssh_private_key_path"
  base64 -d <<< "$SSH_KEY" > "$ssh_private_key_path"
  chmod 0600 "$ssh_private_key_path"

  if [[ -z "$KUBERNETES_VERSION" ]]; then
    # shellcheck disable=SC2016
    >&2 echo 'KUBERNETES_VERSION environment variable is required.'
    return 1
  fi

  if [[ -z "$CRI" ]]; then
    # shellcheck disable=SC2016
    >&2 echo 'CRI environment variable is required.'
    return 1
  fi

  if [[ -z "${DECKHOUSE_IMAGE_TAG}" ]]; then
    # shellcheck disable=SC2016
    >&2 echo 'DECKHOUSE_IMAGE_TAG environment variable is required.'
    return 1
  fi
  DEV_BRANCH="${DECKHOUSE_IMAGE_TAG}"

  if [[ -z "$PREFIX" ]]; then
    # shellcheck disable=SC2016
    >&2 echo 'PREFIX environment variable is required.'
    return 1
  fi


  if [[ -n "$INITIAL_IMAGE_TAG" && "${INITIAL_IMAGE_TAG}" != "${DECKHOUSE_IMAGE_TAG}" ]]; then
    # Use initial image tag as devBranch setting in InitConfiguration.
    # Then switch deploment to DECKHOUSE_IMAGE_TAG.
    DEV_BRANCH="${INITIAL_IMAGE_TAG}"
    SWITCH_TO_IMAGE_TAG="${DECKHOUSE_IMAGE_TAG}"
    echo "Will install '${DEV_BRANCH}' first and then switch to '${SWITCH_TO_IMAGE_TAG}'"
  fi

  case "$PROVIDER" in
  "Yandex.Cloud")
    # shellcheck disable=SC2016
    env CLOUD_ID="$(base64 -d <<< "$LAYOUT_YANDEX_CLOUD_ID")" FOLDER_ID="$(base64 -d <<< "$LAYOUT_YANDEX_FOLDER_ID")" \
        SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${CLOUD_ID} ${FOLDER_ID} ${SERVICE_ACCOUNT_JSON}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="cloud-user"
    ;;

  "GCP")
    # shellcheck disable=SC2016
    env SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${SERVICE_ACCOUNT_JSON}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="user"
    ;;

  "AWS")
    # shellcheck disable=SC2016
    env AWS_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_ACCESS_KEY")" AWS_SECRET_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_SECRET_ACCESS_KEY")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${AWS_ACCESS_KEY} ${AWS_SECRET_ACCESS_KEY}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="centos"
    ;;

  "Azure")
    # shellcheck disable=SC2016
    env SUBSCRIPTION_ID="$LAYOUT_AZURE_SUBSCRIPTION_ID" CLIENT_ID="$LAYOUT_AZURE_CLIENT_ID" \
        CLIENT_SECRET="$LAYOUT_AZURE_CLIENT_SECRET"  TENANT_ID="$LAYOUT_AZURE_TENANT_ID" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${TENANT_ID} ${CLIENT_SECRET} ${CLIENT_ID} ${SUBSCRIPTION_ID}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="azureuser"
    ;;

  "OpenStack")
    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${OS_PASSWORD}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"
    ssh_user="debian"
    ;;

  "vSphere")
    # shellcheck disable=SC2016
    env VSPHERE_PASSWORD="$(base64 -d <<<"$LAYOUT_VSPHERE_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" VSPHERE_BASE_DOMAIN="$LAYOUT_VSPHERE_BASE_DOMAIN" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${VSPHERE_PASSWORD} ${VSPHERE_BASE_DOMAIN}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"
    ssh_user="ubuntu"
    ;;

  "Static")
    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${OS_PASSWORD}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" PREFIX="$PREFIX" \
        envsubst '$PREFIX $OS_PASSWORD' \
        <"$cwd/infra.tpl.tf"* >"$cwd/infra.tf"
    # "Hide" infra template from terraform.
    mv "$cwd/infra.tpl.tf" "$cwd/infra.tpl.tf.orig"

    ssh_user="astra"
    ;;
  esac

  >&2 echo "Use configuration in directory '$cwd':"
  >&2 ls -la $cwd
}

function run-test() {
  if [[ "$PROVIDER" == "Static" ]]; then
    bootstrap_static || return $?
  else
    bootstrap || return $?
  fi

  wait_cluster_ready || return $?

  if [[ -n ${SWITCH_TO_IMAGE_TAG} ]]; then
    change_deckhouse_image "${SWITCH_TO_IMAGE_TAG}" || return $?
    wait_cluster_ready || return $?
  fi
}

function bootstrap_static() {
  >&2 echo "Run terraform to create nodes for Static cluster ..."
  pushd "$cwd"
  terraform init -input=false -plugin-dir=/usr/local/share/terraform/plugins || return $?
  terraform apply -auto-approve -no-color | tee "$cwd/terraform.log" || return $?
  popd

  if ! master_ip="$(grep "master_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d " ")" ; then
    >&2 echo "ERROR: can't parse master_ip from terraform.log"
    return 1
  fi
  if ! system_ip="$(grep "system_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d " ")" ; then
    >&2 echo "ERROR: can't parse system_ip from terraform.log"
    return 1
  fi

  # Bootstrap
  >&2 echo "Run dhctl bootstrap ..."
  dhctl bootstrap --yes-i-want-to-drop-cache --ssh-host "$master_ip" --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
  --config "$cwd/configuration.yaml" --resources "$cwd/resources.yaml" | tee "$cwd/bootstrap.log" || return $?

  >&2 echo "==============================================================

  Cluster bootstrapped. Register 'system' node and starting the test now.

  If you'd like to pause the cluster deletion for debugging:
   1. ssh to cluster: 'ssh $ssh_user@$master_ip'
   2. execute 'kubectl create configmap pause-the-test'

=============================================================="

  >&2 echo 'Fetch registration script ...'
  for ((i=0; i<10; i++)); do
    bootstrap_system="$($ssh_command -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo su -c /bin/bash << "ENDSSH"
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
set -Eeuo pipefail
kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-system -o json | jq -r '.data."bootstrap.sh"'
ENDSSH
)" && break
    >&2 echo "Attempt to get secret manual-bootstrap-for-system in d8-cloud-instance-manager namespace #$i failed. Sleeping 30 seconds..."
    sleep 30
  done

  if [[ -z "$bootstrap_system" ]]; then
    >&2 echo "Couldn't get secret manual-bootstrap-for-system in d8-cloud-instance-manager namespace."
    return 1
  fi

  # shellcheck disable=SC2087
  # Node reboots in bootstrap process, so ssh exits with error code 255. It's normal, so we use || true to avoid script fail.
  $ssh_command -i "$ssh_private_key_path" "$ssh_user@$system_ip" sudo su -c /bin/bash <<ENDSSH || true
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
set -Eeuo pipefail
base64 -d <<< "$bootstrap_system" | bash
ENDSSH

  registration_failed=
  >&2 echo 'Waiting until Node registration finishes ...'
  for ((i=1; i<=10; i++)); do
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo su -c /bin/bash <<"ENDSSH"; then
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
set -Eeuo pipefail
kubectl get nodes
kubectl get nodes -o json | jq -re '.items | length > 0' >/dev/null
kubectl get nodes -o json | jq -re '[ .items[].status.conditions[] | select(.type == "Ready") ] | map(.status == "True") | all' >/dev/null
ENDSSH
      registration_failed=""
      break
    else
      registration_failed="true"
      >&2 echo "Node registration is still in progress (attempt #$i of 10). Sleeping 60 seconds ..."
      sleep 60
    fi
  done

  if [[ $registration_failed == "true" ]] ; then
    return 1
  fi
}

function bootstrap() {
  >&2 echo "Run dhctl bootstrap ..."
  dhctl bootstrap --yes-i-want-to-drop-cache --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
  --resources "$cwd/resources.yaml" --config "$cwd/configuration.yaml" | tee "$cwd/bootstrap.log" || return $?

  if ! master_ip="$(parse_master_ip_from_log)"; then
    return 1
  fi

  >&2 echo "==============================================================

  Cluster bootstrapped. Starting the test now.

  If you'd like to pause the cluster deletion for debugging:
   1. ssh to cluster: 'ssh $ssh_user@$master_ip'
   2. execute 'kubectl create configmap pause-the-test'

=============================================================="

  provisioning_failed=

  >&2 echo 'Waiting until Machine provisioning finishes ...'
  for ((i=1; i<=20; i++)); do
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo su -c /bin/bash <<"ENDSSH"; then
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
set -Eeuo pipefail
kubectl -n d8-cloud-instance-manager get machines
kubectl -n d8-cloud-instance-manager get machine -o json | jq -re '.items | length > 0' >/dev/null
kubectl -n d8-cloud-instance-manager get machines -o json|jq -re '.items | map(.status.currentStatus.phase == "Running") | all' >/dev/null
ENDSSH
      provisioning_failed=""
      break
    else
      provisioning_failed="true"
      >&2 echo "Machine provisioning is still in progress (attempt #$i of 20). Sleeping 60 seconds ..."
      sleep 60
    fi
  done

  if [[ $provisioning_failed == "true" ]] ; then
    return 1
  fi
}

# change_deckhouse_image changes deckhouse container image.
#
# Arguments:
#  - ssh_private_key_path
#  - ssh_user
#  - master_ip
#  - branch
function change_deckhouse_image() {
  new_image_tag="${1}"
  >&2 echo "Change Deckhouse image to ${new_image_tag}."
  if ! $ssh_command -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo su -c /bin/bash <<ENDSSH; then
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
set -Eeuo pipefail
kubectl -n d8-system set image deployment/deckhouse deckhouse=dev-registry.deckhouse.io/sys/deckhouse-oss:${new_image_tag}
ENDSSH
    >&2 echo "Cannot change deckhouse image to ${new_image_tag}."
    return 1
  fi

  testScript=$(cat <<"END_SCRIPT"
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
set -Eeuo pipefail
kubectl -n d8-system get pods -l app=deckhouse
[[ "$(kubectl -n d8-system get pods -l app=deckhouse -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}')" ==  "True" ]]
END_SCRIPT
)

  testRunAttempts=5
  for ((i=1; i<=$testRunAttempts; i++)); do
    >&2 echo "Check Deckhouse pod readiness."
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
      test_failed=""
      break
    else
      test_failed="true"
      >&2 echo "Check Deckhouse pod readiness via SSH: attempt $i/$testRunAttempts failed. Sleeping 30 seconds..."
      sleep 30
    fi
  done
  if [[ $test_failed == "true" ]] ; then
    return 1
  fi
}

# wait_cluster_ready constantly checks if cluster components become ready.
#
# Arguments:
#  - ssh_private_key_path
#  - ssh_user
#  - master_ip
function wait_cluster_ready() {
  test_failed=

  testScript=$(cat <<"END_SCRIPT"
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
set -Eeuo pipefail

function pause-the-test() {
  while true; do
    if ! { kubectl get configmap pause-the-test -o json | jq -re '.metadata.name == "pause-the-test"' >/dev/null ; }; then
      break
    fi

    >&2 echo 'Waiting until "kubectl delete cm pause-the-test" before destroying cluster'

    sleep 30
  done
}

trap pause-the-test EXIT

for ((i=0; i<10; i++)); do
  smoke_mini_addr=$(kubectl -n d8-upmeter get ep smoke-mini -o json | jq -re '.subsets[].addresses[0] | .ip') && break
  >&2 echo "Attempt to get Endpoints for smoke-mini #$i failed. Sleeping 30 seconds..."
  sleep 30
done

if [[ -z "$smoke_mini_addr" ]]; then
  >&2 echo "Couldn't get smoke-mini's address from Endpoints in 5 minutes."
  exit 1
fi

if ! ingress_inlet=$(kubectl get ingressnginxcontrollers.deckhouse.io -o json | jq -re '.items[0] | .spec.inlet // empty'); then
  ingress="ok"
else
  ingress=""
fi

for ((i=0; i<15; i++)); do
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

  if [[ "$api:$disk:$dns:$prometheus:$ingress" == "ok:ok:ok:ok:ok" ]]; then
    exit 0
  fi

  sleep 30
done

>&2 echo 'Timeout waiting for checks to succeed'
exit 1
END_SCRIPT
)

  testRunAttempts=5
  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
      test_failed=""
      break
    else
      test_failed="true"
      >&2 echo "Run test script via SSH: attempt $i/$testRunAttempts failed. Sleeping 30 seconds..."
      sleep 30
    fi
  done

  >&2 echo "Fetch Deckhouse logs after test ..."
  $ssh_command -i "$ssh_private_key_path" "$ssh_user@$master_ip" sudo su -c /bin/bash > "$cwd/deckhouse.json.log" <<"ENDSSH"
kubectl -n d8-system logs deploy/deckhouse
ENDSSH

  if [[ $test_failed == "true" ]] ; then
    return 1
  fi
}

function parse_master_ip_from_log() {
  >&2 echo "  Detect master_ip from bootstrap.log ..."
  if ! master_ip="$(grep -Po '(?<=master_ip_address_for_ssh = ).+$' "$cwd/bootstrap.log")"; then
    >&2 echo "    ERROR: can't parse master_ip from bootstrap.log"
    return 1
  fi
  echo "${master_ip}"
}

function main() {
  >&2 echo "Start cloud test script"
  if ! prepare_environment ; then
    exit 2
  fi

  exitCode=0
  case "${1}" in
    run-test)
      run-test || { exitCode=$? && >&2 echo "Cloud test failed or aborted." ;}
    ;;

    cleanup)
      cleanup || exitCode=$?
    ;;

    "")
      # default action is bootstrap + cleanup
      run-test || { exitCode=$? && >&2 echo "Cloud test failed or aborted." ;}
      # Ignore cleanup exit code, return exit code of bootstrap phase.
      cleanup || true
    ;;

    *)
      >&2 echo "Unknown command '${1}'"
      >&2 echo
      >&2 echo "${usage}"
      exit 1
    ;;
  esac
  if [[ $exitCode == 0 ]]; then
    echo "E2E test: Success!"
  else
    echo "E2E test: fail."
  fi
  exit $exitCode
}

main "$@"
