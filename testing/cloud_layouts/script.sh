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
\$PROVIDER             An infrastructure provider: AWS, GCP, Azure, OpenStack,
                      Static, vSphere or Yandex.Cloud.
                      See them in the cloud_layout directory.
\$LAYOUT               Layout for provider: WithoutNAT, Standard or Static.
                      See available layouts inside the provider directory.
\$PREFIX               A unique prefix to run several tests simultaneously.
\$KUBERNETES_VERSION   A version of Kubernetes to install.
\$CRI                  Containerd.
\$DECKHOUSE_DOCKERCFG  Base64 encoded docker registry credentials.
\$DECKHOUSE_IMAGE_TAG  An image tag for deckhouse Deployment. A Git tag to
                      test prerelease and release images or pr<NUM> slug
                      to test changes in pull requests.
\$INITIAL_IMAGE_TAG    An image tag for Deckhouse deployment to
                      install first and then switching to DECKHOUSE_IMAGE_TAG.
                      Also, run test suite for these 2 versions.

Provider specific environment variables:

  Yandex.Cloud:

\$LAYOUT_YANDEX_CLOUD_ID
\$LAYOUT_YANDEX_FOLDER_ID
\$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON

  GCP:

\$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON

  AWS:

\$LAYOUT_AWS_ACCESS_KEY
\$LAYOUT_AWS_SECRET_ACCESS_KEY

  Azure:

\$LAYOUT_AZURE_SUBSCRIPTION_ID
\$LAYOUT_AZURE_TENANT_ID
\$LAYOUT_AZURE_CLIENT_ID
\$LAYOUT_AZURE_CLIENT_SECRET

  Openstack:

\$LAYOUT_OS_PASSWORD

  vSphere:

\$LAYOUT_VSPHERE_PASSWORD

  Static:

\$LAYOUT_OS_PASSWORD

EOF
)

set -Eeo pipefail
shopt -s inherit_errexit
shopt -s failglob

# Image tag to install.
DEV_BRANCH=
# Image tag to switch to if initial_image_tag is set.
SWITCH_TO_IMAGE_TAG=
# Path to private SSH key to connect to cluster after bootstrap
ssh_private_key_path=
# User for SSH connect.
ssh_user=
# IP of master node.
master_ip=
# bootstrap log
bootstrap_log="${DHCTL_LOG_FILE}"
terraform_state_file="/tmp/static-${LAYOUT}-${CRI}-${KUBERNETES_VERSION}.tfstate"
# Logs dir
logs=/tmp/logs
mkdir -p $logs

# function generates temp ssh parameters file
function set_common_ssh_parameters() {
  cat <<EOF >/tmp/cloud-test-ssh-config
BatchMode yes
UserKnownHostsFile /dev/null
StrictHostKeyChecking no
ServerAliveInterval 5
ServerAliveCountMax 5
ConnectTimeout 10
LogLevel quiet
EOF
  # ssh command with common args.
  ssh_command="ssh -F /tmp/cloud-test-ssh-config"
}

function abort_bootstrap_from_cache() {
  >&2 echo "Run abort_bootstrap_from_cache"
  dhctl --do-not-write-debug-log-file bootstrap-phase abort \
    --force-abort-from-cache \
    --config "$cwd/configuration.yaml" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function abort_bootstrap() {
  >&2 echo "Run abort_bootstrap"
  dhctl --do-not-write-debug-log-file bootstrap-phase abort \
    --ssh-user "$ssh_user" \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --config "$cwd/configuration.yaml" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function destroy_cluster() {
  >&2 echo "Run destroy_cluster"
  dhctl --do-not-write-debug-log-file destroy \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --ssh-user "$ssh_user" \
    --ssh-host "$master_ip" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing

  return $?
}

function destroy_static_infra() {
  >&2 echo "Run destroy_static_infra from ${terraform_state_file}"

  pushd "$cwd"
  export TF_PLUGIN_CACHE_DIR=/plugins
  terraform init -input=false || return $?
  terraform destroy -state="${terraform_state_file}" -input=false -auto-approve || exitCode=$?
  popd

  return $exitCode
}

function cleanup() {
  if [[ "$PROVIDER" == "Static" ]]; then
    >&2 echo "Run cleanup ... destroy terraform infra"
    destroy_static_infra || exitCode=$?
    return $exitCode
  fi

  master_ip="$MASTER_CONNECTION_STRING"
  if [[ -n "$MASTER_CONNECTION_STRING" ]]; then
    arrConn=(${MASTER_CONNECTION_STRING//@/ })
    master_ip="${arrConn[1]}"
    if [[ -n "${arrConn[0]}" ]]; then
      ssh_user="${arrConn[0]}"
    fi
  fi

  if [[ -z "$master_ip" ]]; then
      # Check if 'dhctl bootstrap' was not started.
      if [[ ! -f "$bootstrap_log" ]] ; then
        >&2 echo "Run cleanup ... no bootstrap.log, no need to cleanup."
        return 0
      fi

      if ! master_ip="$(parse_master_ip_from_log)" ; then
        master_ip=""
      fi
  fi

  >&2 echo "Run cleanup ..."
  if [[ -z "$master_ip" ]] ; then
    >&2 echo "No master IP: try to abort without cache, then abort from cache"
    abort_bootstrap || abort_bootstrap_from_cache
  else
    >&2 echo "Master IP is '${master_ip}', user is '${ssh_user}': try to destroy cluster, then abort from cache"
    destroy_cluster || abort_bootstrap_from_cache
  fi
}

function prepare_environment() {
  root_wd="/deckhouse/testing/cloud_layouts"

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

  if [ -z "$bootstrap_log" ]; then
    bootstrap_log="$cwd/bootstrap.log"
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

    ssh_user="redos"
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

    ssh_user="redos"
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

    # use different users for different OSs
    ssh_user="astra"
    ssh_user_system="altlinux"
    ssh_user_worker="redos"
    ;;
  esac

  echo -e "\nmaster_user_name_for_ssh = $ssh_user\n" >> "$bootstrap_log"
  echo -e "\nbastion_user_name_for_ssh = $ssh_user\n" >> "$bootstrap_log"

  set_common_ssh_parameters

  >&2 echo "Use configuration in directory '$cwd':"
  >&2 ls -la $cwd
}

function write_deckhouse_logs() {
  testLog=$(cat <<"END_SCRIPT"
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
kubectl -n d8-system logs deploy/deckhouse
END_SCRIPT
)
  >&2 echo -n "Fetch Deckhouse logs if error test ..."

  getDeckhouseLogsAttempts=5
  attempt=0
  for ((i=1; i<=$getDeckhouseLogsAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash > "$logs/deckhouse.json.log" <<<"${testLog}"; then
      return 0
    else
      >&2 echo "Getting deckhouse logs $i/$getDeckhouseLogsAttempts failed. Sleeping 5 seconds..."
      sleep 5
    fi
  done

  >&2 echo "ERROR: getting deckhouse logs after $getDeckhouseLogsAttempts)"
  return 1
}

function run-test() {
  if [[ "$PROVIDER" == "Static" ]]; then
    bootstrap_static
    exit_code=$?
    if [[ $exit_code -ne 0 ]]; then
      write_deckhouse_logs
      return "$exit_code"
    fi
  else
    bootstrap
    exit_code=$?
    if [[ $exit_code -ne 0 ]]; then
      write_deckhouse_logs
      return "$exit_code"
    fi
  fi

  wait_deckhouse_ready || return $?
  wait_cluster_ready || return $?

  if [[ -n ${SWITCH_TO_IMAGE_TAG} ]]; then
    change_deckhouse_image "${SWITCH_TO_IMAGE_TAG}" || return $?
    wait_deckhouse_ready || return $?
    wait_cluster_ready || return $?
  fi
}

function bootstrap_static() {
  >&2 echo "Run terraform to create nodes for Static cluster ..."
  pushd "$cwd"
  export TF_PLUGIN_CACHE_DIR=/plugins
  terraform init -input=false || return $?
  terraform apply -state="${terraform_state_file}" -auto-approve -no-color | tee "$cwd/terraform.log" || return $?
  popd

  if ! master_ip="$(grep "master_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d "\" ")" ; then
    >&2 echo "ERROR: can't parse master_ip from terraform.log"
    return 1
  fi
  if ! system_ip="$(grep "system_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d "\" ")" ; then
    >&2 echo "ERROR: can't parse system_ip from terraform.log"
    return 1
  fi
  if ! worker_ip="$(grep "worker_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d "\" ")" ; then
    >&2 echo "ERROR: can't parse worker_ip from terraform.log"
    return 1
  fi
  if ! bastion_ip="$(grep "bastion_ip_address_for_ssh" "$cwd/terraform.log"| cut -d "=" -f2 | tr -d "\" ")" ; then
    >&2 echo "ERROR: can't parse bastion_ip from terraform.log"
    return 1
  fi

  echo -e "\nmaster_ip_address_for_ssh = $master_ip\n" >> "$bootstrap_log"
  echo -e "\nbastion_ip_address_for_ssh = $bastion_ip\n" >> "$bootstrap_log"

  # Add key to access to hosts thru bastion
  eval "$(ssh-agent -s)"
  ssh-add "$ssh_private_key_path"
  ssh_bastion="-J $ssh_user@$bastion_ip"

  waitForInstancesAreBootstrappedAttempts=20
  attempt=0
  until $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" /usr/local/bin/is-instance-bootstrapped; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$waitForInstancesAreBootstrappedAttempts" ]; then
      >&2 echo "ERROR: master instance couldn't get bootstrapped"
      return 1
    fi
    >&2 echo "ERROR: master instance isn't bootstrapped yet (attempt #$attempt of $waitForInstancesAreBootstrappedAttempts)"
    sleep 5
  done

  attempt=0
  until $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user_system@$system_ip" /usr/local/bin/is-instance-bootstrapped; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$waitForInstancesAreBootstrappedAttempts" ]; then
      >&2 echo "ERROR: system instance couldn't get bootstrapped"
      return 1
    fi
    >&2 echo "ERROR: system instance isn't bootstrapped yet (attempt #$attempt of $waitForInstancesAreBootstrappedAttempts)"
    sleep 5
  done

  attempt=0
  until $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user_worker@$worker_ip" /usr/local/bin/is-instance-bootstrapped; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$waitForInstancesAreBootstrappedAttempts" ]; then
      >&2 echo "ERROR: worker instance couldn't get bootstrapped"
      return 1
    fi
    >&2 echo "ERROR: worker instance isn't bootstrapped yet (attempt #$attempt of $waitForInstancesAreBootstrappedAttempts)"
    sleep 5
  done

  testRunAttempts=20

  for ((i=1; i<=$testRunAttempts; i++)); do
    # Install http/https proxy on bastion node
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$bastion_ip" sudo su -c /bin/bash <<ENDSSH; then
       apt-get update
       apt-get install -y docker.io
       docker run -d --name='tinyproxy' -p 8888:8888 mirror.gcr.io/monokal/tinyproxy:latest ANY
ENDSSH
      initial_setup_failed=""
      break
    else
      initial_setup_failed="true"
      >&2 echo "Initial setup of bastion in progress (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi

  for ((i=1; i<=$testRunAttempts; i++)); do
    # Convert to air-gap environment by removing default route
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<ENDSSH; then
       echo "#!/bin/sh" > /etc/network/if-up.d/add-routes
       echo "ip route add 10.111.0.0/16 dev lo" >> /etc/network/if-up.d/add-routes
       echo "ip route add 10.222.0.0/16 dev lo" >> /etc/network/if-up.d/add-routes
       echo "ip route del default" >> /etc/network/if-up.d/add-routes
       chmod 0755 /etc/network/if-up.d/add-routes
       ip route del default
       ip route add 10.111.0.0/16 dev lo
       ip route add 10.222.0.0/16 dev lo
ENDSSH
      initial_setup_failed=""
      break
    else
      initial_setup_failed="true"
      >&2 echo "Initial setup of master in progress (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi

  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user_system@$system_ip" sudo su -c /bin/bash <<ENDSSH; then
       echo "#!/bin/sh" > /etc/rc.d/rc.local
       echo "ip route add 10.111.0.0/16 dev lo" >> /etc/rc.d/rc.local
       echo "ip route add 10.222.0.0/16 dev lo" >> /etc/rc.d/rc.local
       echo "ip route del default" >> /etc/rc.d/rc.local
       chmod 0755 /etc/rc.d/rc.local
       ip route del default
       ip route add 10.111.0.0/16 dev lo
       ip route add 10.222.0.0/16 dev lo
ENDSSH
      initial_setup_failed=""
      break
    else
      initial_setup_failed="true"
      >&2 echo "Initial setup of system in progress (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi

  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user_worker@$worker_ip" sudo su -c /bin/bash <<ENDSSH; then
       echo "#!/bin/sh" > /etc/NetworkManager/dispatcher.d/add-routes
       echo "ip route add 10.111.0.0/16 dev lo" >> /etc/NetworkManager/dispatcher.d/add-routes
       echo "ip route add 10.222.0.0/16 dev lo" >> /etc/NetworkManager/dispatcher.d/add-routes
       echo "ip route del default" >> /etc/NetworkManager/dispatcher.d/add-routes
       chmod 0755 /etc/NetworkManager/dispatcher.d/add-routes
       ip route del default
       ip route add 10.111.0.0/16 dev lo
       ip route add 10.222.0.0/16 dev lo
ENDSSH
      initial_setup_failed=""
      break
    else
      initial_setup_failed="true"
      >&2 echo "Initial setup of worker in progress (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi

  # Prepare resources.yaml for starting working node with CAPS
  # shellcheck disable=SC2016
  env b64_SSH_KEY="$(base64 -w0 "$ssh_private_key_path")" WORKER_USER="$ssh_user_worker" WORKER_IP="$worker_ip" \
      envsubst '${b64_SSH_KEY} ${WORKER_USER} ${WORKER_IP}' \
      <"$cwd/resources.tpl.yaml" >"$cwd/resources.yaml"

  # Kill previous ssh-agent due to dhctl creates own agent
  kill $SSH_AGENT_PID

  # Bootstrap
  >&2 echo "Run dhctl bootstrap ..."
  dhctl --do-not-write-debug-log-file bootstrap --resources-timeout="30m" --yes-i-want-to-drop-cache --ssh-bastion-host "$bastion_ip" --ssh-bastion-user="$ssh_user" --ssh-host "$master_ip" --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
  --config "$cwd/configuration.yaml" --config "$cwd/resources.yaml" | tee -a "$bootstrap_log" || return $?

  >&2 echo "==============================================================

  Cluster bootstrapped. Register 'system' and 'worker' nodes and starting the test now.

  If you'd like to pause the cluster deletion for debugging:
   1. ssh to cluster: 'ssh $ssh_user@$master_ip'
   2. execute 'kubectl create configmap pause-the-test'

=============================================================="

  >&2 echo 'Fetch registration script ...'
  for ((i=0; i<10; i++)); do
    bootstrap_system="$($ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash << "ENDSSH"
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
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
  $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user_system@$system_ip" sudo su -c /bin/bash <<ENDSSH || true
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
base64 -d <<< "$bootstrap_system" | bash
ENDSSH

  registration_failed=
  >&2 echo 'Waiting until Node registration finishes ...'
  for ((i=1; i<=20; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<"ENDSSH"; then
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl get nodes -o wide
kubectl get nodes -o json | jq -re '.items | length == 3' >/dev/null
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
  dhctl --do-not-write-debug-log-file bootstrap --resources-timeout="30m" --yes-i-want-to-drop-cache --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
  --config "$cwd/resources.yaml" --config "$cwd/configuration.yaml" | tee -a "$bootstrap_log"

  dhctl_exit_code=$?

  if ! master_ip="$(parse_master_ip_from_log)"; then
    return 1
  fi

  if [[ $dhctl_exit_code -ne 0 ]]; then
    return "$dhctl_exit_code"
  fi

  >&2 echo "==============================================================

  Cluster bootstrapped. Starting the test now.

  If you'd like to pause the cluster deletion for debugging:
   1. ssh to cluster: 'ssh $ssh_user@$master_ip $ssh_bastion'
   2. execute 'kubectl create configmap pause-the-test'

=============================================================="

  provisioning_failed=

  >&2 echo 'Waiting until Machine provisioning finishes ...'
  for ((i=1; i<=20; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<"ENDSSH"; then
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl -n d8-cloud-instance-manager get machines.machine.sapcloud.io
kubectl -n d8-cloud-instance-manager get machines.machine.sapcloud.io -o json | jq -re '.items | length > 0' >/dev/null
kubectl -n d8-cloud-instance-manager get machines.machine.sapcloud.io -o json|jq -re '.items | map(.status.currentStatus.phase == "Running") | all' >/dev/null
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
  if ! $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<ENDSSH; then
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl -n d8-system set image deployment/deckhouse deckhouse=dev-registry.deckhouse.io/sys/deckhouse-oss:${new_image_tag}
ENDSSH
    >&2 echo "Cannot change deckhouse image to ${new_image_tag}."
    return 1
  fi
}

# wait_deckhouse_ready check if deckhouse Pod become ready.
function wait_deckhouse_ready() {
  testScript=$(cat <<"END_SCRIPT"
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl -n d8-system get pods -l app=deckhouse
[[ "$(kubectl -n d8-system get pods -l app=deckhouse -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}{..status.phase}')" ==  "TrueRunning" ]]
END_SCRIPT
)

  testRunAttempts=60
  for ((i=1; i<=$testRunAttempts; i++)); do
    >&2 echo "Check Deckhouse Pod readiness..."
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
      return 0
    fi

    if [[ $i < $testRunAttempts ]]; then
      >&2 echo -n "  Deckhouse Pod not ready. Attempt $i/$testRunAttempts failed. Sleep for 30 seconds..."
      sleep 30
    else
      >&2 echo -n "  Deckhouse Pod not ready. Attempt $i/$testRunAttempts failed."
    fi
  done

  write_deckhouse_logs

  return 1
}

# wait_cluster_ready constantly checks if cluster components become ready.
#
# Arguments:
#  - ssh_private_key_path
#  - ssh_user
#  - master_ip
function wait_cluster_ready() {
  # Print deckhouse info and enabled modules.
  infoScript=$(cat "$(pwd)/testing/cloud_layouts/script.d/wait_cluster_ready/info_script.sh")
  $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${infoScript}";

  test_failed=

  testScript=$(cat "$(pwd)/testing/cloud_layouts/script.d/wait_cluster_ready/test_script.sh")

  testRunAttempts=5
  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
      test_failed=""
      break
    else
      test_failed="true"
      >&2 echo "Run test script via SSH: attempt $i/$testRunAttempts failed. Sleeping 30 seconds..."
      sleep 30
    fi
  done

  if [[ $test_failed == "true" ]] ; then
    return 1
  fi

  testAlerts=$(cat "$(pwd)/testing/cloud_layouts/script.d/wait_cluster_ready/test_alerts.sh")

  test_failed="true"
  if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testAlerts}"; then
    test_failed=""
  else
    test_failed="true"
    >&2 echo "Run test script via SSH: attempt $i/$testRunAttempts failed. Sleeping 30 seconds..."
    sleep 30
  fi

  if [[ $test_failed == "true" ]] ; then
      return 1
  fi

  write_deckhouse_logs
}

function parse_master_ip_from_log() {
  >&2 echo "  Detect master_ip from bootstrap.log ..."
  if ! master_ip="$(grep -Po '(?<=master_ip_address_for_ssh = ")((\d{1,3}\.){3}\d{1,3})(?=")' "$bootstrap_log")"; then
    >&2 echo "    ERROR: can't parse master_ip from bootstrap.log"
    return 1
  fi
  echo "${master_ip}"
}

function chmod_dirs_for_cleanup() {
  if [ -n "$USER_RUNNER_ID" ]; then
    echo "Fix temp directories owner before cleanup ..."
    chown -R $USER_RUNNER_ID "/deckhouse/testing" || true
    chown -R $USER_RUNNER_ID /tmp || true
  else
    echo "Fix temp directories permissions before cleanup ..."
    chmod -f -R 777 "/deckhouse/testing" || true
    chmod -f -R 777 /tmp || true
  fi
}


function main() {
  >&2 echo "Start cloud test script"
  # switch to the / folder to dhctl proper work
  cd /

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

  chmod_dirs_for_cleanup
  exit $exitCode
}

main "$@"
