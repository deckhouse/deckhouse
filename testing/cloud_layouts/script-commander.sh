#!/bin/bash

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
\$PREFIX               A unique prefix to run several tests simultaneously.
\$KUBERNETES_VERSION   A version of Kubernetes to install.
\$CRI                  Containerd.

Provider specific environment variables:

  Yandex.Cloud:

\$LAYOUT_YANDEX_CLOUD_ID
\$LAYOUT_YANDEX_FOLDER_ID
\$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON

  GCP:

\$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON

  AWS, EKS:

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

function prepare_environment() {
    root_wd="${PWD}/testing/cloud_layouts"

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
    BRANCH="${DECKHOUSE_IMAGE_TAG}"

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
    CLOUD_ID="$(base64 -d <<< "$LAYOUT_YANDEX_CLOUD_ID")"
    FOLDER_ID="$(base64 -d <<< "$LAYOUT_YANDEX_FOLDER_ID")"
    SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON")"
    ssh_user="redos"
    cluster_template_version_id="6a47d23a-e16f-4e7a-bf57-a65f7c05e8ae"
    ;;

  "GCP")
    # shellcheck disable=SC2016
    env SERVICE_ACCOUNT_JSON="$(base64 -d <<< "$LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${SERVICE_ACCOUNT_JSON} ${MASTERS_COUNT}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="user"
    ;;

  "AWS")
    # shellcheck disable=SC2016
    env AWS_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_ACCESS_KEY")" AWS_SECRET_ACCESS_KEY="$(base64 -d <<< "$LAYOUT_AWS_SECRET_ACCESS_KEY")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${AWS_ACCESS_KEY} ${AWS_SECRET_ACCESS_KEY} ${MASTERS_COUNT}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="ec2-user"
    ;;

  "Azure")
    # shellcheck disable=SC2016
    env SUBSCRIPTION_ID="$LAYOUT_AZURE_SUBSCRIPTION_ID" CLIENT_ID="$LAYOUT_AZURE_CLIENT_ID" \
        CLIENT_SECRET="$LAYOUT_AZURE_CLIENT_SECRET"  TENANT_ID="$LAYOUT_AZURE_TENANT_ID" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${TENANT_ID} ${CLIENT_SECRET} ${CLIENT_ID} ${SUBSCRIPTION_ID} ${MASTERS_COUNT}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="azureuser"
    ;;

  "OpenStack")
    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${OS_PASSWORD} ${MASTERS_COUNT}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    ssh_user="redos"
    ;;

  "vSphere")
    # shellcheck disable=SC2016
    env VSPHERE_PASSWORD="$(base64 -d <<<"$LAYOUT_VSPHERE_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" VSPHERE_BASE_DOMAIN="$LAYOUT_VSPHERE_BASE_DOMAIN" MASTERS_COUNT="$MASTERS_COUNT" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${VSPHERE_PASSWORD} ${VSPHERE_BASE_DOMAIN} ${MASTERS_COUNT}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

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
        VCD_SERVER="$LAYOUT_VCD_SERVER" \
        VCD_USERNAME="$LAYOUT_VCD_USERNAME" \
        VCD_ORG="$LAYOUT_VCD_ORG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${VCD_PASSWORD} ${VCD_SERVER} ${VCD_USERNAME} ${VCD_ORG} ${MASTERS_COUNT}' \
        <"$cwd/configuration.tpl.yaml" >"$cwd/configuration.yaml"

    [ -f "$cwd/resources.tpl.yaml" ] && \
        env VCD_ORG="$LAYOUT_VCD_ORG" \
        envsubst '${DECKHOUSE_DOCKERCFG} ${PREFIX} ${DEV_BRANCH} ${KUBERNETES_VERSION} ${CRI} ${VCD_PASSWORD} ${VCD_SERVER} ${VCD_USERNAME} ${VCD_ORG}' \
        <"$cwd/resources.tpl.yaml" >"$cwd/resources.yaml"

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
    ssh_redos_user_worker="redos"
    ssh_opensuse_user_worker="opensuse"
    ssh_rosa_user_worker="centos"
    ;;
  esac
}

function run-test() {
  local commander_host="$COMMANDER_HOST"
  local commander_token="$COMMANDER_TOKEN"
  local payload
  local response
  local cluster_id

payload="{
    \"name\": \"${PREFIX}\",
    \"cluster_template_version_id\": \"${cluster_template_version_id}\",
    \"values\": {
        \"branch\": \"${BRANCH}\",
        \"prefix\": \"${PREFIX}\",
        \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
        \"defaultCRI\": \"${CRI}\",
        \"master_count\": ${MASTERS_COUNT},
        \"cloud_id\": \"${CLOUD_ID}\",
        \"folder_id\": \"${FOLDER_ID}\",
        \"service_account_json\": ${SERVICE_ACCOUNT_JSON},
        \"sshPrivateKey\": \"${SSH_KEY}\",
        \"sshUser\": \"${ssh_user}\",
        \"deckhouse_dockercfg\": \"${DECKHOUSE_DOCKERCFG}\"
    }
}"

  echo "Bootstrap payload: ${payload}"

  response=$(curl -X POST "https://${commander_host}/api/v1/clusters" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: ${commander_token}" \
    -H 'Content-Type: application/json' \
    -d "$payload" \
    -w "\n%{http_code}")

  http_code=$(echo "$response" | tail -n 1)
  response=$(echo "$response" | head -n -1)

  # Check for HTTP errors
  if [[ ${http_code} -ge 400 ]]; then
    echo "Error: HTTP error ${http_code}" >&2
    echo "$response" >&2
    return 1
  fi

  echo "Bootstrap resuest response: ${response}"

  # Extract id and handle errors
  CLUSTER_ID=$(jq -r '.id' <<< "$response")
  if [[ $? -ne 0 ]]; then
    echo "Error: jq failed to extract cluster ID" >&2
     echo "$response" >&2
    return 1
  fi

  echo "Cluster ID: ${CLUSTER_ID}"

  # Waiting to cluster ready
  testRunAttempts=120
  sleep=30
  for ((i=1; i<=testRunAttempts; i++)); do
    cluster_status="$(curl -s -X 'GET' \
      "https://${COMMANDER_HOST}/api/v1/clusters/${cluster_id}" \
      -H 'accept: application/json' \
      -H "X-Auth-Token: ${COMMANDER_TOKEN}" |
      jq -r '.status')"
    >&2 echo "Check Cluster ready..."
    if [ "in_sync" != "$cluster_status" ]; then
      return 0
    fi
    if [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n "  Cluster not ready. Attempt $i/$testRunAttempts failed. Sleep for $sleep seconds..."
      sleep $sleep
    else
      >&2 echo -n "  Cluster not ready. Attempt $i/$testRunAttempts failed."
    fi
    return 1
  done
}

function cleanup() {
  curl -s -X 'DELETE' \
      "https://$COMMANDER_HOST/api/v1/clusters/${cluster_id}" \
      -H 'accept: application/json' \
      -H "X-Auth-Token: $COMMANDER_TOKEN"

  # Waiting to cluster cleanup
  testRunAttempts=40
  sleep=30
  for ((i=1; i<=testRunAttempts; i++)); do
    cluster_status=$(curl -s -X 'GET' \
      -H 'accept: application/json' \
      -H "X-Auth-Token: ${COMMANDER_TOKEN}" \
      -o /dev/null -w "%{http_code}" \
      "https://${COMMANDER_HOST}/api/v1/clusters/${cluster_id}")
    >&2 echo "Check Cluster delete..."
    if [ "404" != "$cluster_status" ]; then
      return 0
    fi
    if [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n "  Cluster not deleted. Attempt $i/$testRunAttempts failed. Sleep for $sleep seconds..."
      sleep $sleep
    else
      >&2 echo -n "  Cluster not deleted. Attempt $i/$testRunAttempts failed."
    fi
    return 1
  done

}


function main() {
  exitCode=0
  >&2 echo "Start cloud test script"
  if ! prepare_environment ; then
   exit 2
  fi
  case "${1}" in
    run-test)
      run-test || { exitCode=$? && >&2 echo "Cloud test failed or aborted." ;}
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

  exit $exitCode
}

main "$@"
