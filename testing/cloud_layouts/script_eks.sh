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
terraform_state_file="/tmp/eks-${LAYOUT}-${CRI}-${KUBERNETES_VERSION}.tfstate"
kubectl_config_file="/tmp/eks-${LAYOUT}-${CRI}-${KUBERNETES_VERSION}.kubeconfig"

function prepare_environment() {
  root_wd="/deckhouse/testing/cloud_layouts/"
  export cwd="/deckhouse/testing/cloud_layouts/EKS/WithoutNAT/"

  export AWS_ACCESS_KEY_ID="$(base64 -d <<< "$LAYOUT_AWS_ACCESS_KEY")"
  export AWS_SECRET_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_SECRET_ACCESS_KEY")"
  export KUBERNETES_VERSION="$KUBERNETES_VERSION"
  export CRI="$CRI"
  export LAYOUT="$LAYOUT"
  export DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG"
  export AWS_DEFAULT_REGION="$LAYOUT_AWS_DEFAULT_REGION"
  export INITIAL_IMAGE_TAG="$INITIAL_IMAGE_TAG"
  export DECKHOUSE_IMAGE_TAG="$DECKHOUSE_IMAGE_TAG"

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

  if [[ -n "$INITIAL_IMAGE_TAG" && "${INITIAL_IMAGE_TAG}" != "${DECKHOUSE_IMAGE_TAG}" ]]; then
    # Use initial image tag as devBranch setting in InitConfiguration.
    # Then switch deploment to DECKHOUSE_IMAGE_TAG.
    export DEV_BRANCH="${INITIAL_IMAGE_TAG}"
    export SWITCH_TO_IMAGE_TAG="${DECKHOUSE_IMAGE_TAG}"
    echo "Will install '${DEV_BRANCH}' first and then switch to '${SWITCH_TO_IMAGE_TAG}'"
  fi

  # shellcheck disable=SC2016
  env KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI}' \
      <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

  env KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
      envsubst '${DECKHOUSE_DOCKERCFG} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI}' \
      <"$cwd/infra.tf.tpl" >"$cwd/infra.tf"

}


function bootstrap_eks() {
  # set -x

  >&2 echo "Run terraform to create nodes for EKS cluster ..."
#  pushd "$cwd"
  cd $cwd
  terraform init -input=false -plugin-dir=/usr/local/share/terraform/plugins || return $?
  terraform apply -state="${terraform_state_file}" -auto-approve -no-color | tee "$cwd/terraform.log" || return $?
#  popd

  if ! cluster_endpoint="$(tail -n5 "$cwd/terraform.log" | grep "cluster_endpoint" | cut -d "=" -f2 | tr -d " \"")" ; then
    >&2 echo "ERROR: can't parse cluster_endpoint from terraform output"
    return 1
  fi
  if ! cluster_name="$(tail -n5 "$cwd/terraform.log" | grep "cluster_name" | cut -d "=" -f2 | tr -d " \"")" ; then
    >&2 echo "ERROR: can't parse cluster_name from terraform output"
    return 1
  fi
  if ! region="$(tail -n5 "$cwd/terraform.log" | grep "region" | cut -d "=" -f2 | tr -d " \"")" ; then
    >&2 echo "ERROR: can't parse region from terraform output"
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

# wait_deckhouse_ready check if deckhouse Pod become ready.
function wait_deckhouse_ready() {
  testRunAttempts=60
  for ((i=1; i<=$testRunAttempts; i++)); do
    >&2 echo "Check Deckhouse Pod readiness..."
    if [[ "$(kubectl -n d8-system get pods -l app=deckhouse -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}{..status.phase}')" ==  "TrueRunning" ]]; then
      return 0
    fi

    if [[ $i < $testRunAttempts ]]; then
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
}

function destroy_eks_infra() {
  >&2 echo "Run destroy_eks_infra from ${terraform_state_file}"

#  pushd "$cwd"
  cd $cwd
  terraform init -input=false -plugin-dir=/plugins || return $?
  terraform destroy -state="${terraform_state_file}" -auto-approve -no-color | tee "$cwd/terraform.log" || return $?
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
