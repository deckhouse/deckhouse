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
\$MASTERS_COUNT         Number of master nodes in the cluster.
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

  VCD:

\$LAYOUT_VCD_PASSWORD

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
  scp_command="scp -F /tmp/cloud-test-ssh-config"
}

function abort_bootstrap_from_cache() {
  >&2 echo "Run abort_bootstrap_from_cache"
  dhctl --do-not-write-debug-log-file bootstrap-phase abort \
    --force-abort-from-cache \
    --config "$cwd/configuration.yaml" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing $ssh_bastion_params

  return $?
}

function abort_bootstrap() {
  >&2 echo "Run abort_bootstrap"
  dhctl --do-not-write-debug-log-file bootstrap-phase abort \
    --ssh-user "$ssh_user" \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --config "$cwd/configuration.yaml" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing $ssh_bastion_params

  return $?
}

function destroy_cluster() {
  >&2 echo "Run destroy_cluster"
  dhctl --do-not-write-debug-log-file destroy \
    --ssh-agent-private-keys "$ssh_private_key_path" \
    --ssh-user "$ssh_user" \
    --ssh-host "$master_ip" \
    --yes-i-am-sane-and-i-understand-what-i-am-doing $ssh_bastion_params

  return $?
}

function destroy_static_infra() {
  >&2 echo "Run destroy_static_infra from ${terraform_state_file}"

  pushd "$cwd"
  opentofu init -input=false -plugin-dir=/plugins || return $?
  opentofu destroy -state="${terraform_state_file}" -input=false -auto-approve || exitCode=$?
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

  # Check if LAYOUT_STATIC_BASTION_IP is set and not empty
  if [[ -n "$LAYOUT_STATIC_BASTION_IP" ]]; then
    ssh_bastion_params="--ssh-bastion-host $LAYOUT_STATIC_BASTION_IP --ssh-bastion-user $ssh_user"
    >&2 echo "Using static bastion at $LAYOUT_STATIC_BASTION_IP"
  else
    ssh_bastion_params=""
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
  FLANT_AUTH_B64=$(echo -n "$FLANT_REGISTRY_USER:$FLANT_REGISTRY_PASSWORD" | base64 -w0)
  FLANT_CONFIG_JSON="{\"auths\":{\"$FLANT_REGISTRY_HOST\":{\"username\":\"$FLANT_REGISTRY_USER\",\"password\":\"$FLANT_REGISTRY_PASSWORD\",\"auth\":\"$FLANT_AUTH_B64\"}}}"
  FLANT_DOCKERCFG_B64=$(echo "$FLANT_CONFIG_JSON" | base64 -w0)
  case "$PROVIDER" in
  "Yandex.Cloud")
    # shellcheck disable=SC2016
    env CLOUD_ID="$(base64 -d <<< "$LAYOUT_YANDEX_CLOUD_ID")" FOLDER_ID="$(base64 -d <<< "$LAYOUT_YANDEX_FOLDER_ID")" \
        SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="redos"
    ;;

  "GCP")
    # shellcheck disable=SC2016
    env SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="user"
    ;;

  "AWS")
    # shellcheck disable=SC2016
    env AWS_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_ACCESS_KEY")" AWS_SECRET_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_SECRET_ACCESS_KEY")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="ec2-user"
    ;;

  "Azure")
    # shellcheck disable=SC2016
    env SUBSCRIPTION_ID="$LAYOUT_AZURE_SUBSCRIPTION_ID" CLIENT_ID="$LAYOUT_AZURE_CLIENT_ID" \
        CLIENT_SECRET="$LAYOUT_AZURE_CLIENT_SECRET"  TENANT_ID="$LAYOUT_AZURE_TENANT_ID" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="azureuser"
    ;;

  "OpenStack")
    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" FLANT_DOCKERCFG="$FLANT_DOCKERCFG_B64" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="redos"
    ;;

  "vSphere")
    # shellcheck disable=SC2016
    env VSPHERE_PASSWORD="$(base64 -d <<<"$LAYOUT_VSPHERE_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" FLANT_DOCKERCFG="$FLANT_DOCKERCFG_B64" VSPHERE_BASE_DOMAIN="$LAYOUT_VSPHERE_BASE_DOMAIN" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="redos"
    ;;

"VCD")
    # shellcheck disable=SC2016
    env VCD_PASSWORD="$(base64 -d <<<"$LAYOUT_VCD_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" \
        CRI="$CRI" \
        DEV_BRANCH="$DEV_BRANCH" \
        PREFIX="$PREFIX" \
        MASTERS_COUNT="$MASTERS_COUNT" \
        DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
        FLANT_DOCKERCFG="$FLANT_DOCKERCFG_B64" \
        VCD_SERVER="$LAYOUT_VCD_SERVER" \
        VCD_USERNAME="$LAYOUT_VCD_USERNAME" \
        VCD_ORG="$LAYOUT_VCD_ORG" \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    [ -f "$cwd/resources.tpl.yaml" ] && \
        env VCD_ORG="$LAYOUT_VCD_ORG" \
        envsubst <"$cwd/resources.tpl.yaml" >"$cwd/resources.yaml"

    ssh_user="ubuntu"
    ;;

  "Static")
    pre_bootstrap_static_setup
    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" \
        CRI="$CRI" \
        DEV_BRANCH="$DEV_BRANCH" \
        PREFIX="$PREFIX" \
        DECKHOUSE_DOCKERCFG="$LOCAL_DECKHOUSE_DOCKERCFG" \
        FLANT_DOCKERCFG="$FLANT_DOCKERCFG_B64" \
        IMAGES_REPO=$IMAGES_REPO \
        envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" PREFIX="$PREFIX" \
        envsubst <"$cwd/infra.tpl.tf"* >"$cwd/infra.tf"
    # "Hide" infra template from terraform.
    mv "$cwd/infra.tpl.tf" "$cwd/infra.tpl.tf.orig"

    # use different users for different OSs
    ssh_user="astra"
    ssh_user_system="altlinux"
    ssh_redos_user_worker="redos"
    ssh_opensuse_user_worker="opensuse"
    ssh_rosa_user_worker="centos"
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
    test_requirements || return $?
    change_deckhouse_image "${IMAGES_REPO:-"dev-registry.deckhouse.io/sys/deckhouse-oss"}:${SWITCH_TO_IMAGE_TAG}" || return $?
    wait_deckhouse_ready || return $?
    wait_cluster_ready || return $?
  fi
}

# Parse DEV_BRANCH and convert to semver format
parse_version_from_branch() {
    local branch="$1"
    local version=""

    # Extract version pattern like "1.69" from various formats
    if [[ "$branch" =~ release-([0-9]+\.[0-9]+) ]]; then
        version="v${BASH_REMATCH[1]}.0"
    elif [[ "$branch" =~ v?([0-9]+\.[0-9]+)(\.[0-9]+)? ]]; then
        # Handle cases like "v1.69" or "1.69.1"
        if [[ -n "${BASH_REMATCH[2]}" ]]; then
            version="v${BASH_REMATCH[1]}${BASH_REMATCH[2]}"
        else
            version="v${BASH_REMATCH[1]}.0"
        fi
    else
        # Fallback: try to extract any version-like pattern
        if [[ "$branch" =~ ([0-9]+\.[0-9]+) ]]; then
            version="v${BASH_REMATCH[1]}.0"
        else
            # If no version pattern found, return original or default
            version="v0.0.0"
        fi
    fi

    echo "$version"
}

function test_requirements() {
  >&2 echo "Start check requirements ..."
  if [ ! -f /deckhouse/release.yaml ]; then
      >&2 echo "File /deckhouse/release.yaml not found"
      return 1
  fi

  release=$(< /deckhouse/release.yaml)
  if [ -z "${release:-}" ]; then return 1; fi
  release=${release//\"/\\\"}


  >&2 echo "Run script ... "

  SEMVER_VERSION=$(parse_version_from_branch "${DEV_BRANCH}")
  if [ -z "${SEMVER_VERSION:-}" ]; then
    >&2 echo "Failed to parse version from branch '${DEV_BRANCH}'"
    return 1
  fi

  testScript=$(cat <<ENDSC
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail

>&2 echo "Check python ..."
function check_python() {
  for pybin in python3 python2 python; do
    if command -v "\$pybin" >/dev/null 2>&1; then
      python_binary="\$pybin"
      return 0
    fi
  done
  echo "Python not found, exiting..."
  return 1
}
check_python

>&2 echo "Create release file ..."

echo "$release" > /tmp/releaseFile.yaml

>&2 echo "Release file ..."

cat /tmp/releaseFile.yaml

>&2 echo "Apply module config ..."

echo 'apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  settings:
    releaseChannel: Stable
    update:
      mode: Auto' | kubectl apply -f -

>&2 echo "Apply deckhousereleases ..."

echo "apiVersion: deckhouse.io/v1alpha1
approved: false
kind: DeckhouseRelease
metadata:
  annotations:
    dryrun: \"true\"
  name: ${SEMVER_VERSION}
spec:
  version: ${SEMVER_VERSION}
  requirements: {}
" | \$python_binary -c "
import yaml, sys

data = yaml.safe_load(sys.stdin)
with open('/tmp/releaseFile.yaml') as f:
  d1 = yaml.safe_load(f)
r = d1.get('requirements', {})
r.pop('k8s', None)  # remove the 'k8s' key
r.pop('autoK8sVersion', None)  # remove the 'autoK8sVersion' key
data['spec']['requirements'] = r
print(yaml.dump(data))
" | kubectl apply -f -

>&2 echo "Remove release file ..."

rm /tmp/releaseFile.yaml

>&2 echo "Sleep 5 seconds before check..."

sleep 5

>&2 echo "Release status: \$(kubectl get deckhousereleases.deckhouse.io -o 'jsonpath={..status.phase}')"
if [ ! -z "\$(kubectl get deckhousereleases.deckhouse.io -o 'jsonpath={..status.message}')" ]; then
  >&2 echo "Error message: \$(kubectl get deckhousereleases.deckhouse.io -o 'jsonpath={..status.message}')"
fi

[[ "\$(kubectl get deckhousereleases.deckhouse.io -o 'jsonpath={..status.phase}')" == "Deployed" ]]
ENDSC
)

  testRequirementsAttempts=10
  for ((i=1; i<=$testRequirementsAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su - -c /bin/bash <<<$testScript; then
        return 0
    else
      >&2 echo "Test requirements $i/$testRequirementsAttempts failed. Sleeping 5 seconds..."
      sleep 5
    fi
  done

  write_deckhouse_logs
  return 1
}

function pre_bootstrap_static_setup() {
  cd $cwd/registry-mirror

  BASTION_INTERNAL_IP=192.168.199.254
  IMAGES_REPO="${BASTION_INTERNAL_IP}:5000/sys/deckhouse-oss"

  LOCAL_REGISTRY_MIRROR_PASSWORD=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 20; echo)
  LOCAL_REGISTRY_CLUSTER_PASSWORD=$(tr -dc A-Za-z0-9 </dev/urandom | head -c 20; echo)

  LOCAL_REGISTRY_CLUSTER_DOCKERCFG=$(echo -n "cluster:${LOCAL_REGISTRY_CLUSTER_PASSWORD}" | base64 -w0)

  # emulate using local registry
  LOCAL_DECKHOUSE_DOCKERCFG=$(echo -n {\"auths\":{\"${BASTION_INTERNAL_IP}:5000\":{\"auth\":\"${LOCAL_REGISTRY_CLUSTER_DOCKERCFG}\"}}} | base64 -w0)

  cd ..
  # todo: delete after migrating openstack to opentofy
  cp -a /plugins/registry.terraform.io/terraform-provider-openstack/ /plugins/registry.opentofu.org/terraform-provider-openstack/
}

function bootstrap_static() {
  >&2 echo "Run terraform to create nodes for Static cluster ..."
  pushd "$cwd"

  opentofu init -input=false -plugin-dir=/plugins || return $?
  opentofu apply -state="${terraform_state_file}" -auto-approve -no-color | tee "$cwd/terraform.log" || return $?
  popd

  if ! master_ip="$(opentofu output -state="${terraform_state_file}" -raw master_ip_address_for_ssh)"; then
    >&2 echo "ERROR: can't get master_ip from opentofu output"
    return 1
  fi

  if ! system_ip="$(opentofu output -state="${terraform_state_file}" -raw system_ip_address_for_ssh)"; then
    >&2 echo "ERROR: can't get system_ip from opentofu output"
    return 1
  fi

  if ! worker_redos_ip="$(opentofu output -state="${terraform_state_file}" -raw worker_redos_ip_address_for_ssh)"; then
    >&2 echo "ERROR: can't get worker_redos_ip from opentofu output"
    return 1
  fi

  if ! worker_opensuse_ip="$(opentofu output -state="${terraform_state_file}" -raw worker_opensuse_ip_address_for_ssh)"; then
    >&2 echo "ERROR: can't get worker_opensuse_ip from opentofu output"
    return 1
  fi

  if ! worker_rosa_ip="$(opentofu output -state="${terraform_state_file}" -raw worker_rosa_ip_address_for_ssh)"; then
    >&2 echo "ERROR: can't get worker_rosa_ip from opentofu output"
    return 1
  fi

  if ! bastion_ip="$(opentofu output -state="${terraform_state_file}" -raw bastion_ip_address_for_ssh)"; then
    >&2 echo "ERROR: can't get bastion_ip from opentofu output"
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
  until $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_redos_user_worker@$worker_redos_ip" /usr/local/bin/is-instance-bootstrapped; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$waitForInstancesAreBootstrappedAttempts" ]; then
      >&2 echo "ERROR: worker instance couldn't get bootstrapped"
      return 1
    fi
    >&2 echo "ERROR: worker instance isn't bootstrapped yet (attempt #$attempt of $waitForInstancesAreBootstrappedAttempts)"
    sleep 5
  done

  attempt=0
  until $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_opensuse_user_worker@$worker_opensuse_ip" /usr/local/bin/is-instance-bootstrapped; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$waitForInstancesAreBootstrappedAttempts" ]; then
      >&2 echo "ERROR: worker instance couldn't get bootstrapped"
      return 1
    fi
    >&2 echo "ERROR: worker instance isn't bootstrapped yet (attempt #$attempt of $waitForInstancesAreBootstrappedAttempts)"
    sleep 5
  done

  attempt=0
  until $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_rosa_user_worker@$worker_rosa_ip" /usr/local/bin/is-instance-bootstrapped; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "$waitForInstancesAreBootstrappedAttempts" ]; then
      >&2 echo "ERROR: rosa worker instance couldn't get bootstrapped"
      return 1
    fi
    >&2 echo "ERROR: rosa worker instance isn't bootstrapped yet (attempt #$attempt of $waitForInstancesAreBootstrappedAttempts)"
    sleep 5
  done

  testRunAttempts=20
  for ((i=1; i<=$testRunAttempts; i++)); do
    # Install http/https proxy on bastion node
    $scp_command -r -i "$ssh_private_key_path" $cwd/registry-mirror "$ssh_user@$bastion_ip:/tmp"
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$bastion_ip" sudo su -c /bin/bash <<ENDSSH; then
      apt-get update
      apt-get install -y docker.io docker-compose wget curl

      cd /tmp/registry-mirror
      ./gen-auth-cfg.sh "${LOCAL_REGISTRY_MIRROR_PASSWORD}" "${LOCAL_REGISTRY_CLUSTER_PASSWORD}" > auth_config.yaml
      ./gen-ssl.sh
      env BASTION_INTERNAL_IP=${BASTION_INTERNAL_IP} envsubst '\$BASTION_INTERNAL_IP' < registry-config.tpl.yaml > registry-config.yaml
      docker-compose up -d
      cd -
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

  env b64_SSH_KEY="$(base64 -w0 "$ssh_private_key_path")" \
    MASTER_USER="$ssh_user" MASTER_IP="$master_ip" \
    WORKER_REDOS_USER="$ssh_redos_user_worker" WORKER_REDOS_IP="$worker_redos_ip" \
    WORKER_OPENSUSE_USER="$ssh_opensuse_user_worker" WORKER_OPENSUSE_IP="$worker_opensuse_ip" \
    WORKER_ROSA_USER="$ssh_rosa_user_worker" WORKER_ROSA_IP="$worker_rosa_ip" \
    envsubst <"$cwd/resources.tpl.yaml" >"$cwd/resources.yaml"

  D8_MIRROR_USER="$(echo -n ${DECKHOUSE_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f1)"
  D8_MIRROR_PASSWORD="$(echo -n ${DECKHOUSE_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f2)"
  testRunAttempts=20
  for ((i=1; i<=$testRunAttempts; i++)); do
    # Install http/https proxy on bastion node
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$bastion_ip" sudo su -c /bin/bash <<ENDSSH; then
       cat <<'EOF' > /tmp/install-d8-and-pull-push-images.sh
#!/bin/bash
# get latest d8-cli release
URL="https://api.github.com/repos/deckhouse/deckhouse-cli/releases/latest"
DOWNLOAD_URL=\$(wget -qO- "\${URL}" | grep browser_download_url | cut -d '"' -f 4 | grep linux-amd64 | grep -v sha256sum)
if [ -z "\${DOWNLOAD_URL}" ]; then
  echo "Failed to retrieve the URL for the download"
  exit 1
fi
# download
wget -q "\${DOWNLOAD_URL}" -O /tmp/d8.tar.gz
# install
file /tmp/d8.tar.gz
mkdir d8cli
tar -xf /tmp/d8.tar.gz -C d8cli
mv ./d8cli/linux-amd64/bin/d8 /usr/bin/d8

d8 --version
# pull
d8 mirror pull d8 --source-login ${D8_MIRROR_USER} --source-password ${D8_MIRROR_PASSWORD} \
  --source "dev-registry.deckhouse.io/sys/deckhouse-oss" --deckhouse-tag "${DEV_BRANCH}"
# push
d8 mirror push d8 "${IMAGES_REPO}" --registry-login mirror --registry-password $LOCAL_REGISTRY_MIRROR_PASSWORD

# Checking that it's FE-UPGRADE
# Extracting major and minor versions from DECKHOUSE_IMAGE_TAG
dh_version="${DECKHOUSE_IMAGE_TAG#release-}"
dh_major="\${dh_version%%.*}"

# Handle both 'release-x.y' and 'release-x.y-test-z'
dh_minor_version_part="\${dh_version#*.}"
dh_minor_number="\${dh_minor_version_part%%-*}"
dh_minor="\${dh_minor_number%.*}"

# Extracting the major and minor versions from INITIAL_IMAGE_TAG
initial_version="${INITIAL_IMAGE_TAG#release-}"
initial_major="\${initial_version%%.*}"

initial_minor_version_part="\${initial_version#*.}"
initial_minor_number="\${initial_minor_version_part%%-*}"
initial_minor="\${initial_minor_number%.*}"

echo "Initial Minor Version: \$initial_minor"
echo "Deckhouse Minor Version: \$dh_minor"

# Check that the major versions match and the minor differs by +1
if [ "\$dh_major" = "\$initial_major" ] && [ "\$dh_minor" -eq "\$((initial_minor + 1))" ]; then
    >&2 echo "Pull both versions of fe-upgrade"
    # pull
    d8 mirror pull d8-upgrade --source-login ${D8_MIRROR_USER} --source-password ${D8_MIRROR_PASSWORD} \
    --source "dev-registry.deckhouse.io/sys/deckhouse-oss" --deckhouse-tag "${DECKHOUSE_IMAGE_TAG}"
    # push
    d8 mirror push d8-upgrade "${IMAGES_REPO}" --registry-login mirror --registry-password ${LOCAL_REGISTRY_MIRROR_PASSWORD}
fi

set +x
EOF
       chmod +x /tmp/install-d8-and-pull-push-images.sh
       /tmp/install-d8-and-pull-push-images.sh

       # cleanup
       rm -f /tmp/install-d8-and-pull-push-images.sh
       rm -rf d8
       rm -rf d8-upgrade

       docker run -d --name='tinyproxy' --restart=always -p 8888:8888 -e ALLOWED_NETWORKS="127.0.0.1/8 10.0.0.0/8 192.168.0.1/8" mirror.gcr.io/kalaksi/tinyproxy:latest@sha256:561ef49fa0f0a9747db12abdfed9ab3d7de17e95c811126f11e026b3b1754e54
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
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_redos_user_worker@$worker_redos_ip" sudo su -c /bin/bash <<ENDSSH; then
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
      >&2 echo "Initial setup of redos worker in progress (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi

  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_opensuse_user_worker@$worker_opensuse_ip" sudo su -c /bin/bash <<ENDSSH; then
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
      >&2 echo "Initial setup of opensuse worker in progress (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi

  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_rosa_user_worker@$worker_rosa_ip" sudo su -c /bin/bash <<ENDSSH; then
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
      >&2 echo "Initial setup of rosa worker in progress (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi

  # Prepare resources.yaml for starting working node with CAPS
  # shellcheck disable=SC2016
  env b64_SSH_KEY="$(base64 -w0 "$ssh_private_key_path")" \
      MASTER_USER="$ssh_user" MASTER_IP="$master_ip" \
      WORKER_REDOS_USER="$ssh_redos_user_worker" WORKER_REDOS_IP="$worker_redos_ip" \
      WORKER_OPENSUSE_USER="$ssh_opensuse_user_worker" WORKER_OPENSUSE_IP="$worker_opensuse_ip" \
      WORKER_ROSA_USER="$ssh_rosa_user_worker" WORKER_ROSA_IP="$worker_rosa_ip" \
      envsubst <"$cwd/resources.tpl.yaml" >"$cwd/resources.yaml"

  # Bootstrap
  >&2 echo "Run dhctl bootstrap ..."
  for ((i=1; i<=$testRunAttempts; i++)); do
    $scp_command -i "$ssh_private_key_path" $cwd/configuration.yaml "$ssh_user@$bastion_ip:/tmp/configuration.yaml"
    $scp_command -i "$ssh_private_key_path" $cwd/resources.yaml "$ssh_user@$bastion_ip:/tmp/resources.yaml"
    $scp_command -i "$ssh_private_key_path" $ssh_private_key_path "$ssh_user@$bastion_ip:/tmp/sshkey"
    if $ssh_command -i "$ssh_private_key_path" "$ssh_user@$bastion_ip" sudo su -c /bin/bash <<ENDSSH; then
      mkdir -p /etc/docker
      echo '{"insecure-registries":["192.168.199.254:5000"]}' > /etc/docker/daemon.json
      systemctl restart docker
      docker login -p ${LOCAL_REGISTRY_MIRROR_PASSWORD} -u mirror ${IMAGES_REPO}
      docker run \
        -v /tmp/sshkey:/tmp/sshkey \
        -v /tmp/configuration.yaml:/tmp/configuration.yaml \
        -v /tmp/resources.yaml:/tmp/resources.yaml \
        ${IMAGES_REPO}/install:${DEV_BRANCH} \
        dhctl --do-not-write-debug-log-file bootstrap \
            --resources-timeout="30m" --yes-i-want-to-drop-cache \
            --ssh-host "$master_ip" \
            --ssh-agent-private-keys "/tmp/sshkey" \
            --ssh-user "$ssh_user" \
            --ssh-extra-args="-S ssh" \
            --config "/tmp/configuration.yaml" \
            --config "/tmp/resources.yaml" | tee -a "$bootstrap_log" || return $?
ENDSSH
      initial_setup_failed=""
      break
    else
      initial_setup_failed="true"
      >&2 echo "Bootstrap cluster (attempt #$i of $testRunAttempts). Sleeping 5 seconds ..."
      sleep 5
    fi
  done
  if [[ $initial_setup_failed == "true" ]] ; then
    return 1
  fi
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
kubectl get nodes -o json | jq -re '.items | length == 5' >/dev/null
kubectl get nodes -o json | jq -re '[ .items[].status.conditions[] | select(.type == "Ready") ] | map(.status == "True") | all' >/dev/null
ENDSSH
      registration_failed=""
      break
    else
      registration_failed="true"
      >&2 echo "Node registration is still in progress (attempt #$i of 20). Sleeping 60 seconds ..."
      sleep 60
    fi
  done

  if [[ -z $provisioning_failed && $CIS_ENABLED == "true" ]]; then
    for ((i=1; i<=5; i++)); do
      if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<"ENDSSH"; then
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable operator-trivy
kubectl label ns security-scanning.deckhouse.io/enabled="" --all > /dev/null
ENDSSH
        break
      else
        sleep 20
      fi
    done
  fi

  if [[ $registration_failed == "true" ]] ; then
    return 1
  fi
}

function bootstrap() {
  >&2 echo "Run dhctl bootstrap ..."

  # Start ssh-agent and add the private key
  eval "$(ssh-agent -s)"
  ssh-add "$ssh_private_key_path"

  # Check if LAYOUT_STATIC_BASTION_IP is set and not empty
  if [[ -n "$LAYOUT_STATIC_BASTION_IP" ]]; then
    ssh_bastion="-J $ssh_user@$LAYOUT_STATIC_BASTION_IP"
    ssh_bastion_params="--ssh-bastion-host $LAYOUT_STATIC_BASTION_IP --ssh-bastion-user $ssh_user"
    >&2 echo "Using static bastion at $LAYOUT_STATIC_BASTION_IP"

    echo "bastion_ip_address_for_ssh = \"$LAYOUT_STATIC_BASTION_IP\"" >> "$bootstrap_log"
    echo "bastion_user_name_for_ssh = \"$ssh_user\"" >> "$bootstrap_log"

  else
    ssh_bastion=""
    ssh_bastion_params=""
  fi

  dhctl --do-not-write-debug-log-file bootstrap --resources-timeout="30m" --yes-i-want-to-drop-cache $ssh_bastion_params \
        --ssh-agent-private-keys "$ssh_private_key_path" --ssh-user "$ssh_user" \
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
kubectl get instance -o json >/dev/null
kubectl get instance -o json | jq -re '.items | length > 0' >/dev/null
kubectl get instance -o json | jq -re '.items | map(.status.currentStatus.phase == "Running") | all' >/dev/null
ENDSSH
      provisioning_failed=""
      break
    else
      provisioning_failed="true"
      >&2 echo "Machine provisioning is still in progress (attempt #$i of 20). Sleeping 60 seconds ..."
      sleep 60
    fi
  done

  if [[ -z $provisioning_failed && $CIS_ENABLED == "true" ]]; then
    for ((i=1; i<=5; i++)); do
      if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<"ENDSSH"; then
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable operator-trivy
kubectl label ns security-scanning.deckhouse.io/enabled="" --all > /dev/null
ENDSSH
        break
      else
        sleep 20
      fi
    done
  fi

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
  new_image="${1}"
  >&2 echo "Change Deckhouse image to ${new_image}."
  if ! $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<ENDSSH; then
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl -n d8-system set image deployment/deckhouse deckhouse=${new_image}
ENDSSH
    >&2 echo "Cannot change deckhouse image to ${new_image}."
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
  infoScript=$(cat "$(pwd)/deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/info_script.sh")
  $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${infoScript}";

  test_failed=

  testNodeUserScript=$(cat "$(pwd)/deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_nodeuser.sh")

  testRunAttempts=5
  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testNodeUserScript}"; then
        test_failed=""
        break
    else
      test_failed="true"
      >&2 echo "Run test NodeUser script via SSH: attempt $i/$testRunAttempts failed. Sleeping 30 seconds.."
      sleep 30
    fi
  done

  if [[ $test_failed == "true" ]] ; then
    return 1
  fi

  for ((i=1; i<=$testRunAttempts; i++)); do
    if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "user-e2e@$master_ip" whoami; then
        >&2 echo "Connection via NodeUser SSH successful."
        test_failed=""
        break
    else
      test_failed="true"
      >&2 echo "Connection via NodeUser SSH: attempt $i/$testRunAttempts failed. Sleeping 30 seconds.."
      sleep 30
    fi
  done

  if [[ $test_failed == "true" ]] ; then
    return 1
  fi

  testScript=$(cat "$(pwd)/deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_script.sh")

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

  if [[ $TEST_AUTOSCALER_ENABLED == "true" ]] ; then
    testAutoscalerScript=$(cat "$(pwd)/deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_autoscaler.sh")

    testRunAttempts=5
    for ((i=1; i<=$testRunAttempts; i++)); do
      if $ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testAutoscalerScript}"; then
        test_failed=""
        break
      else
        test_failed="true"
        >&2 echo "Run test script via SSH: attempt $i/$testRunAttempts failed. Sleeping 30 seconds..."
        sleep 30
      fi
    done
  else
    echo "Autoscaler test skipped."
  fi

  if [[ $test_failed == "true" ]] ; then
    return 1
  fi

  testAlerts=$(cat "$(pwd)/deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_alerts.sh")

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

  if [[ $CIS_ENABLED == "true" ]]; then
    testCisScript=$(cat "$(pwd)/deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_cis.sh")
    REPORT=$($ssh_command -i "$ssh_private_key_path" $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testCisScript}")
    echo "$REPORT"
  fi

  write_deckhouse_logs
}

function parse_master_ip_from_log() {
  >&2 echo "  Detect master_ip from bootstrap.log ..."
  if ! master_ip="$(grep -m1 -Po '(?<=master_ip_address_for_ssh = ")((\d{1,3}\.){3}\d{1,3})(?=")' "$bootstrap_log")"; then
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
