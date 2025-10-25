#!/bin/bash

# Copyright 2023 Flant JSC
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

  deploy       Create EKS cluster.

  cleanup        Delete EKS cluster.

Required environment variables:

Name                  Description
---------------------+---------------------------------------------------------
\$PREFIX               A unique prefix to run several tests simultaneously.
\$KUBERNETES_VERSION   A version of Kubernetes to install.
\$CRI                  Containerd.

Provider specific environment variables:

\$LAYOUT_AWS_ACCESS_KEY
\$LAYOUT_AWS_SECRET_ACCESS_KEY
\$LAYOUT_AWS_DEFAULT_REGION

EOF
)

set -Eeo pipefail
shopt -s inherit_errexit
shopt -s failglob

bootstrap_log="${DHCTL_LOG_FILE}"
opentofu_state_file="/tmp/eks-${LAYOUT}-${CRI}-${KUBERNETES_VERSION}.tfstate"
kubectl_config_file="/tmp/eks-${LAYOUT}-${CRI}-${KUBERNETES_VERSION}.kubeconfig"

function prepare_environment() {
  root_wd="/deckhouse/testing/cloud_layouts/"
  export cwd="/deckhouse/testing/cloud_layouts/EKS/WithoutNAT/"

  export AWS_ACCESS_KEY_ID="$LAYOUT_AWS_ACCESS_KEY"
  export AWS_SECRET_ACCESS_KEY="$LAYOUT_AWS_SECRET_ACCESS_KEY"
  export KUBERNETES_VERSION="$KUBERNETES_VERSION"
  export CRI="$CRI"
  export LAYOUT="$LAYOUT"
  export DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG"
  export AWS_DEFAULT_REGION="$LAYOUT_AWS_DEFAULT_REGION"
  export INITIAL_IMAGE_TAG="$INITIAL_IMAGE_TAG"
  export DECKHOUSE_IMAGE_TAG="$DECKHOUSE_IMAGE_TAG"
  export PREFIX="$PREFIX"

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
  export DEV_BRANCH="${DECKHOUSE_IMAGE_TAG}"

  if [[ "$DEV_BRANCH" =~ ^release-[0-9]+\.[0-9]+ ]]; then
    echo "DEV_BRANCH = $DEV_BRANCH: detected release branch"
    export DECKHOUSE_DOCKERCFG=$STAGE_DECKHOUSE_DOCKERCFG
  else
    echo "DEV_BRANCH = $DEV_BRANCH: detected dev branch"
  fi

  decode_dockercfg=$(base64 -d <<< "${DECKHOUSE_DOCKERCFG}")
  IMAGES_REPO=$(jq -r '.auths | keys[]'  <<< "$decode_dockercfg")/sys/deckhouse-oss

  if [[ -n "$INITIAL_IMAGE_TAG" && "${INITIAL_IMAGE_TAG}" != "${DECKHOUSE_IMAGE_TAG}" ]]; then
    # Use initial image tag as devBranch setting in InitConfiguration.
    # Then update cluster to DECKHOUSE_IMAGE_TAG.
    # NOTE: currently only release branches are supported for updating.
    if [[ "${DECKHOUSE_IMAGE_TAG}" =~ release-([0-9]+\.[0-9]+) ]]; then
      DEV_BRANCH="${INITIAL_IMAGE_TAG}"
      SWITCH_TO_IMAGE_TAG="v${BASH_REMATCH[1]}.0"
      update_release_channel "${DEV_REGISTRY_PATH}" "${SWITCH_TO_IMAGE_TAG}"
      echo "Will install '${DEV_BRANCH}' first and then update to '${DECKHOUSE_IMAGE_TAG}' as '${SWITCH_TO_IMAGE_TAG}'"
    else
      echo "'${DECKHOUSE_IMAGE_TAG}' doesn't look like a release branch. Update command politely ignored."
    fi
  fi

  # shellcheck disable=SC2016
  env KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" ="$FOX_DOCKERCFG" IMAGES_REPO="$IMAGES_REPO"\
      envsubst <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  env KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" PREFIX="$PREFIX" \
      envsubst <"$cwd/infra.tf.tpl" >"$cwd/infra.tf"

}


function bootstrap_eks() {
  # set -x

  >&2 echo "Run opentofu to create nodes for EKS cluster ..."
#  pushd "$cwd"
  cd $cwd
  /image/bin/opentofu init -input=false -plugin-dir=/usr/local/share/opentofu/plugins || return $?
  /image/bin/opentofu apply -state="${opentofu_state_file}" -auto-approve -no-color | tee "$cwd/opentofu.log" || return $?
#  popd

  if ! cluster_endpoint="$(tail -n5 "$cwd/opentofu.log" | grep "cluster_endpoint" | cut -d "=" -f2 | tr -d " \"")" ; then
    >&2 echo "ERROR: can't parse cluster_endpoint from opentofu output"
    return 1
  fi
  if ! cluster_name="$(tail -n5 "$cwd/opentofu.log" | grep "cluster_name" | cut -d "=" -f2 | tr -d " \"")" ; then
    >&2 echo "ERROR: can't parse cluster_name from opentofu output"
    return 1
  fi
  if ! region="$(tail -n5 "$cwd/opentofu.log" | grep "region" | cut -d "=" -f2 | tr -d " \"")" ; then
    >&2 echo "ERROR: can't parse region from opentofu output"
    return 1
  fi

  _username_="deckhouse-setup"
  _env_="eks-cluster"
  _tmp_kubeconfig="tmp_kubeconfig"
  SECRET_NAME="cluster-admin-secret"
  aws eks --region $region update-kubeconfig --name $cluster_name --kubeconfig $_tmp_kubeconfig
  export ROLE="cluster-admin"
  export NS="d8-system"
  echo "create service account ${_username_} for env ${_env_}"
  KUBECONFIG=$_tmp_kubeconfig kubectl create ns d8-system
  KUBECONFIG=$_tmp_kubeconfig kubectl create sa $_username_ -n $NS
  echo "Bind SA ${_username_} with ClusterRole ${ROLE} for environment ${_env_}"
  KUBECONFIG=$_tmp_kubeconfig kubectl create clusterrolebinding $_username_ \
    --serviceaccount=$NS:$_username_ \
    --clusterrole=${ROLE}
  KUBECONFIG=$_tmp_kubeconfig kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: $_username_-secret
  namespace: $NS
  annotations:
    kubernetes.io/service-account.name: $_username_
type: kubernetes.io/service-account-token
EOF
  TOKEN=$(KUBECONFIG=$_tmp_kubeconfig kubectl get secrets $_username_-secret -n $NS -o json | jq -r .data.token | base64 -d)
  CA=$(KUBECONFIG=$_tmp_kubeconfig kubectl get secrets $_username_-secret -n $NS -o json | jq -r '.data | .["ca.crt"]')
  SERVER=$(aws eks describe-cluster --name $cluster_name | jq -r .cluster.endpoint)
  cat <<-EOF > $kubectl_config_file
apiVersion: v1
kind: Config
users:
- name: $_username_
  user:
    token: $TOKEN
clusters:
- cluster:
    certificate-authority-data: $CA
    server: $SERVER
  name: $_username_
contexts:
- context:
    cluster: $_username_
    user: $_username_
  name: $_username_
current-context: $_username_
EOF
  echo "Created kubeconfig $kubectl_config_file"
  rm $_tmp_kubeconfig
  echo "Removed file tmp_kubeconfig"

  cat $kubectl_config_file
}

# update_release_channel changes the release-channel image to given tag
function update_release_channel() {
  crane copy "$1/release-channel:$2" "$1/release-channel:beta"
}

# trigger_deckhouse_update sets the release channel for the cluster, prompting it to upgrade to the next version.
function trigger_deckhouse_update() {
  >&2 echo "Setting Deckhouse release channel to Beta."
  if ! kubectl patch mc/deckhouse -p '{"spec": {"settings": {"releaseChannel": "Beta"}}}' --type=merge ; then
    >&2 echo "Cannot change Deckhouse release channel."
    return 1
  fi
}

# wait_update_ready checks if the cluster is ready for updating.
function wait_update_ready() {
  expectedVersion="$1"
  testRunAttempts=20
  for ((i=1; i<=$testRunAttempts; i++)); do
    >&2 echo "Check DeckhouseRelease..."
    deployedVersion="$(kubectl get deckhouserelease -o 'jsonpath={.items[?(@.status.phase=="Deployed")].spec.version}')"
    if [[ "${expectedVersion}" == "${deployedVersion}" ]]; then
      return 0
    elif [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n "  Expected DeckhouseRelease not deployed. Attempt $i/$testRunAttempts failed. Sleep for 30 seconds..."
      sleep 30
    else
      >&2 echo -n "  Expected DeckhouseRelease not deployed. Attempt $i/$testRunAttempts failed."
    fi
  done

  write_deckhouse_logs

  return 1
}

# wait_deckhouse_ready check if deckhouse Pod become ready.
function wait_deckhouse_ready() {
  testRunAttempts=60
  for ((i=1; i<=$testRunAttempts; i++)); do
    >&2 echo "Check Deckhouse Pod readiness..."
    if [[ "$(kubectl -n d8-system get pods -l app=deckhouse -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}{..status.phase}')" ==  "TrueRunning" ]]; then
      return 0
    fi

    if [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n "  Deckhouse Pod not ready. Attempt $i/$testRunAttempts failed. Sleep for 30 seconds..."
      sleep 30
    else
      >&2 echo -n "  Deckhouse Pod not ready. Attempt $i/$testRunAttempts failed."
    fi
  done
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
  chmod 755 /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/info_script_eks.sh
  /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/info_script_eks.sh

  test_failed=

  chmod 755 /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_script_eks.sh

  testRunAttempts=5
  for ((i=1; i<=$testRunAttempts; i++)); do
    if /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_script_eks.sh; then
      test_failed=""
      break
    else
      test_failed="true"
      >&2 echo "Run test script: attempt $i/$testRunAttempts failed. Sleeping 30 seconds..."
      sleep 30
    fi
  done

  >&2 echo "Fetch Deckhouse logs after test ..."
  kubectl -n d8-system logs deploy/deckhouse

  if [[ $test_failed == "true" ]] ; then
    return 1
  fi

  chmod 755 /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_alerts.sh

  testRunAttempts=5
  for ((i=1; i<=$testRunAttempts; i++)); do
    if /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_alerts.sh; then
      test_failed=""
      break
    else
      test_failed="true"
      >&2 echo "Run test script: attempt $i/$testRunAttempts failed. Sleeping 30 seconds..."
      sleep 30
    fi
  done

  if [[ $test_failed == "true" ]] ; then
    return 1
  fi

  if [[ $CIS_ENABLED == "true" ]]; then
    chmod 755 /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_cis_eks.sh
    /deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_cis_eks.sh
  fi
}

function destroy_eks_infra() {
  >&2 echo "Run destroy_eks_infra from ${opentofu_state_file}"

#  pushd "$cwd"
  cd $cwd
  /image/bin/opentofu init -input=false -plugin-dir=/usr/local/share/opentofu/plugins || return $?
  /image/bin/opentofu destroy -state="${opentofu_state_file}" -auto-approve -no-color | tee "$cwd/opentofu.log" || return $?
#  popd

  return $exitCode
}


function run-test() {
  bootstrap_eks || return $?
}

function cleanup() {
  destroy_eks_infra || return $?
}

function chmod_dirs_for_cleanup() {

  if [ -n $USER_RUNNER_ID ]; then
    echo "Fix temp directories owner before cleanup ..."
    chown -R $USER_RUNNER_ID "$(pwd)/testing" || true
    chown -R $USER_RUNNER_ID "/deckhouse/testing" || true
    chown -R $USER_RUNNER_ID /tmp || true
  else
    echo "Fix temp directories permissions before cleanup ..."
    chmod -f -R 777 "$(pwd)/testing" || true
    chmod -f -R 777 "/deckhouse/testing" || true
    chmod -f -R 777 /tmp || true
  fi
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

    wait_deckhouse_ready)
      wait_deckhouse_ready || exitCode=$?
    ;;

    wait_cluster_ready)
      wait_cluster_ready || exitCode=$?
    ;;

    trigger_deckhouse_update)
      if [[ -n ${SWITCH_TO_IMAGE_TAG} ]]; then
        echo "Starting Deckhouse update..."
        trigger_deckhouse_update || return $?
        wait_update_ready "${SWITCH_TO_IMAGE_TAG}"|| return $?
        wait_deckhouse_ready || return $?
        wait_cluster_ready || return $?
      fi
    ;;

    cleanup)
      cleanup || exitCode=$?
    ;;

    "")
      >&2 echo "Empty command"
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
