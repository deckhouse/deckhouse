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
                 using commander api.

  cleanup        Delete cluster.

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
shopt -s failglob

function prepare_environment() {
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

    if [[ -z "$PREFIX" ]]; then
      # shellcheck disable=SC2016
      >&2 echo 'PREFIX environment variable is required.'
      return 1
    fi

    if [[ -z "$COMMANDER_TOKEN" ]]; then
      # shellcheck disable=SC2016
      >&2 echo 'COMMANDER_TOKEN environment variable is required.'
      return 1
    fi

    if [[ -z "$COMMANDER_HOST" ]]; then
      # shellcheck disable=SC2016
      >&2 echo 'COMMANDER_HOST environment variable is required.'
      return 1
    fi

    if [[ -n "$INITIAL_IMAGE_TAG" && "${INITIAL_IMAGE_TAG}" != "${DECKHOUSE_IMAGE_TAG}" ]]; then
      # Use initial image tag as devBranch setting in InitConfiguration.
      # Then switch deployment to DECKHOUSE_IMAGE_TAG.
      DEV_BRANCH="${INITIAL_IMAGE_TAG}"
      SWITCH_TO_IMAGE_TAG="${DECKHOUSE_IMAGE_TAG}"
      echo "Will install '${DEV_BRANCH}' first and then switch to '${SWITCH_TO_IMAGE_TAG}'"
    else
      DEV_BRANCH="${DECKHOUSE_IMAGE_TAG}"
    fi

  case "$PROVIDER" in
  "Yandex.Cloud")
    CLOUD_ID="$(base64 -d <<< "$LAYOUT_YANDEX_CLOUD_ID")"
    FOLDER_ID="$(base64 -d <<< "$LAYOUT_YANDEX_FOLDER_ID")"
    SERVICE_ACCOUNT_JSON=$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON
    ssh_user="redos"
    cluster_template_id="6a47d23a-e16f-4e7a-bf57-a65f7c05e8ae"
    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"cloudId\": \"${CLOUD_ID}\",
      \"folderId\": \"${FOLDER_ID}\",
      \"serviceAccountJson\": \"${SERVICE_ACCOUNT_JSON}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "GCP")
    ssh_user="user"
    cluster_template_id="565ed77c-0ae0-4baa-9ece-6603bcf3139a"
    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"serviceAccountJson\": \"${LAYOUT_GCP_SERVICE_ACCOUT_KEY_JSON}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "AWS")
    ssh_user="ec2-user"
    cluster_template_id="9b567623-91a9-4493-96de-f5c0b6acacfe"
    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"awsAccessKey\": \"${LAYOUT_AWS_ACCESS_KEY}\",
      \"awsSecretKey\": \"${LAYOUT_AWS_SECRET_ACCESS_KEY}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "Azure")
    ssh_user="azureuser"
    cluster_template_id="3900de40-547c-4c62-927c-ef42018d62f4"
    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"subscriptionId\": \"${LAYOUT_AZURE_SUBSCRIPTION_ID}\",
      \"clientId\": \"${LAYOUT_AZURE_CLIENT_ID}\",
      \"clientSecret\": \"${LAYOUT_AZURE_CLIENT_SECRET}\",
      \"tenantId\": \"${LAYOUT_AZURE_TENANT_ID}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "OpenStack")
    ssh_user="redos"
    cluster_template_id="cb79a126-4234-4dac-a01e-2d3804266e3e"
    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"osPassword\": \"${OS_PASSWORD}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "vSphere")
    # shellcheck disable=SC2016
    env VSPHERE_PASSWORD="$(base64 -d <<<"$LAYOUT_VSPHERE_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" VSPHERE_BASE_DOMAIN="$LAYOUT_VSPHERE_BASE_DOMAIN" MASTERS_COUNT="$MASTERS_COUNT" \
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
    # shellcheck disable=SC2016
    env OS_PASSWORD="$(base64 -d <<<"$LAYOUT_OS_PASSWORD")" \
        KUBERNETES_VERSION="$KUBERNETES_VERSION" CRI="$CRI" DEV_BRANCH="$DEV_BRANCH" PREFIX="$PREFIX" DECKHOUSE_DOCKERCFG="$DECKHOUSE_DOCKERCFG" \
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
}

function get_cluster_status() {
  local response
  response=$(curl -s -X 'GET' \
    "https://${COMMANDER_HOST}/api/v1/clusters/${cluster_id}" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: ${COMMANDER_TOKEN}")
  echo "${response}"
}

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
  echo ${SSH_KEY} | base64 -d > id_rsa
  chmod 400 id_rsa
  # ssh command with common args.
  ssh_command="ssh -F /tmp/cloud-test-ssh-config -i id_rsa "
}

function wait_alerts_resolve() {

  allow_alerts=(
  "D8DeckhouseIsNotOnReleaseChannel" # Tests may be made on dev branch
  "DeadMansSwitch" # Always active in system. Tells that monitoring works.
  "CertmanagerCertificateExpired" # On some system do not have DNS
  "CertmanagerCertificateExpiredSoon" # Same as above
  "DeckhouseModuleUseEmptyDir" # TODO Need made split storage class
  "D8EtcdExcessiveDatabaseGrowth" # It may trigger during bootstrap due to a sudden increase in resource count
  "D8CNIMisconfigured" # This alert may appear until we completely abandon the use of the `d8-cni-configuration` secret when configuring CNI.
  "ModuleConfigObsoleteVersion" # This alert is informational and should not block e2e tests
  "D8KubernetesVersionIsDeprecated" # Run test on deprecated version is OK
  "D8ClusterAutoscalerPodIsRestartingTooOften" # Pointless, as component might fail on initial setup/update and test will not succeed with a failed component anyway
  )

  # Alerts
  iterations=20
  sleep_interval=30

  for i in $(seq 1 $iterations); do

    response=$(get_cluster_status)
    alerts=()
    while IFS= read -r alert; do
      alerts+=("$alert")
    done < <(jq -r '.cluster_agent_data[] | select(.source == "overview") | .data.warnings.firing_alerts[].alert' <<< "$response")
    alerts_is_ok=true
    for alert in "${alerts[@]}"; do
      # Check if the alert is in the allow list
      if ! [[ "${allow_alerts[*]}" =~ ${alert} ]]; then
        echo "Error: Unexpected alert: '$alert'"
        alerts_is_ok=false
      else
        echo "Alert '$alert' ignored"
      fi
    done

    if $alerts_is_ok; then
      echo "All alerts are in the allow list."
      break
    else
      echo "Cluster components are not ready. Attempt $i/$iterations failed. Sleep for $sleep_interval seconds..."
      if [[ "$i" -eq "$iterations" ]]; then
        echo "Maximum iterations reached. Cluster components are not ready."
        return 1
      fi
    fi
    sleep "$sleep_interval"

  done
}

function wait_upmeter_green() {
  # Upmeter
  iterations=40
  sleep_interval=30

  for i in $(seq 1 $iterations); do
    response=$(get_cluster_status)
    upmeter_data_exists=$(echo "$response" | jq -r '.cluster_agent_data[] | select(.source == "upmeter") | .data.rows[]' 2>/dev/null)
    if [[ -n $upmeter_data_exists ]]; then
      statuses=$(jq -r '.cluster_agent_data[] | select(.source == "upmeter") | .data.rows[] | .probes[] | "\(.probe):\(.availability)"' <<< "$response")
    else
      echo "  Upmeter don't ready"
      sleep "$sleep_interval"
      continue
    fi

    all_ok=true
    while IFS= read -r line; do
      service=$(echo "$line" | cut -d':' -f1)
      availability=$(echo "$line" | cut -d':' -f2 | awk -F. '{printf "%.2f", $1"."$2}')
      printf "%-50s %8s\n" "$service" "$availability"
      if [[ "$availability" != "1.00" ]]; then
        all_ok=false
      fi
    done <<< "$statuses"

    if $all_ok; then
      echo "All components are available"
      break
    else
      echo "  Cluster components are not ready. Attempt $i/$iterations failed. Sleep for $sleep_interval seconds..."
      if [[ "$i" -eq "$iterations" ]]; then
        echo "Maximum iterations reached. Cluster components are not ready."
        return 1
      fi
    fi
    sleep "$sleep_interval"
  done

}

function check_resources_state_results() {
  echo "Check applied resource status..."
  response=$(get_cluster_status)
  errors=$(jq -r '.resources_state_results[] | select(.errors) | .errors' <<< "$response")
  if [ -n "$errors" ]; then
    echo "  Errors found:"
    echo "${errors}"
    return 1
  fi
  echo "Check applied resource status... Passed"
}

function change_deckhouse_image() {
  new_image_tag="${1}"
  >&2 echo "Change Deckhouse image to ${new_image_tag}."
  if ! $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<ENDSSH; then
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
  for ((i=1; i<=testRunAttempts; i++)); do
    >&2 echo "Check Deckhouse Pod readiness..."
    if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
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

function update_comment() {
  echo "  Updating comment on pull request..."
  comment_url="${GITHUB_API_SERVER}/repos/${REPOSITORY}/issues/comments/${COMMENT_ID}"

  comment=$(curl -s -X GET \
    --retry 3 --retry-delay 5 \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer $GITHUB_TOKEN" \
    "$comment_url" \
    -w "\n%{http_code}")

  http_code=$(echo "$comment" | tail -n 1)
  comment=$(echo "$comment" | sed '$d')

  # Check for HTTP errors
  if [[ ${http_code} -ge 400 ]]; then
    echo "Error: Getting comment error ${http_code}" >&2
    echo "$comment" >&2
    return 1
  fi

  local connection_str_body="${PROVIDER}-${LAYOUT}-${CRI}-${KUBERNETES_VERSION} - Connection string: \`ssh ${bastion_connection} ${master_connection}\`"
  local result_body

  if ! result_body="$(echo "$comment" | jq -crM --arg a "$connection_str_body" '{body: (.body + "\r\n\r\n" + $a + "\r\n")}')"; then
    return 1
  fi

  update_comment_response=$(curl -s -X PATCH \
    --retry 3 --retry-delay 5 \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer $GITHUB_TOKEN" \
    -d "$result_body" \
    "$comment_url" \
    -w "\n%{http_code}")

    http_code=$(echo "$update_comment_response" | tail -n 1)
    response=$(echo "$update_comment_response" | sed '$d')

    # Check for HTTP errors
    if [[ ${http_code} -ge 400 ]]; then
      echo "Error: Writing comment error ${http_code}" >&2
      echo "$response" >&2
      return 1
    else
      echo "  Updating comment on pull request completed"
    fi
}

function run-test() {
  local payload
  local response
  local cluster_id

  cluster_template_version_id=$(curl -s -X 'GET' \
    "https://${COMMANDER_HOST}/api/v1/cluster_templates/${cluster_template_id}?without_archived=true" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: ${COMMANDER_TOKEN}" |
    jq -r 'del(.cluster_template_versions).current_cluster_template_version_id')

  payload="{
    \"name\": \"${PREFIX}\",
    \"cluster_template_version_id\": \"${cluster_template_version_id}\",
    \"values\": ${values}
  }"

  echo "Bootstrap payload: ${payload}"

  sleep_second=0
  for (( j=1; j<=5; j++ )); do
    sleep "$sleep_second"
    sleep_second=5

    response=$(curl -s -X POST  \
      "https://${COMMANDER_HOST}/api/v1/clusters" \
      -H 'accept: application/json' \
      -H "X-Auth-Token: ${COMMANDER_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$payload" \
      -w "\n%{http_code}")

    http_code=$(echo "$response" | tail -n 1)
    response=$(echo "$response" | sed '$d')

    # Check for HTTP errors
    if [[ ${http_code} -ge 400 ]]; then
      echo "Error: HTTP error ${http_code}" >&2
      echo "$response" >&2
      continue
    else
      break
    fi
  done

  cluster_id=$(jq -r '.id' <<< "$response")
  if [[ $cluster_id == "null" ]]; then
    echo "Error: jq failed to extract cluster ID" >&2
     echo "$response" >&2
    return 1
  fi

  echo "Cluster ID: ${cluster_id}"

  # Waiting to cluster ready
  testRunAttempts=80
  sleep=30
  master_ip_find=false
  for ((i=1; i<=testRunAttempts; i++)); do
    response=$(get_cluster_status)
    >&2 echo "Check Cluster ready..."


    # Get ssh connection string
    # TODO add bastion logic
    if [[ "$master_ip_find" == "false" ]]; then
      master_ip=$(jq -r '.connection_hosts.masters[0].host' <<< "$response")
      master_user=$(jq -r '.connection_hosts.masters[0].user' <<< "$response")
      if [[ "$master_ip" != "null" && "$master_user" != "null" ]]; then
        master_connection="${master_user}@${master_ip}"
        master_ip_find=true
        echo "  SSH connection string:"
        echo "      ssh $master_connection"
        update_comment
        echo "$master_connection" > ssh-connect_str-"${PREFIX}"
        # TODO add workflow template
      fi
    fi

    # Get cluster status
    cluster_status=$(jq -r '.status' <<< "$response")
    if [ "in_sync" = "$cluster_status" ]; then
      echo "  Cluster status: $cluster_status"
      echo "Bootstrap completed, starting to deploy additional components"
      break
    elif [ "creation_failed" = "$cluster_status" ]; then
      echo "  Cluster status: $cluster_status"
      return 1
    elif [ "configuration_error" = "$cluster_status" ]; then
      echo "  Cluster status: $cluster_status"
      return 1
    else
      echo "  Cluster status: $cluster_status"
    fi
    if [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n "  Cluster not ready. Attempt $i/$testRunAttempts failed. Sleep for $sleep seconds..."
      sleep $sleep
    else
      >&2 echo -n "  Cluster not ready. Attempt $i/$testRunAttempts failed."
      return 1
    fi
  done

  wait_upmeter_green || return $?

  check_resources_state_results || return $?

  wait_alerts_resolve || return $?

  set_common_ssh_parameters

  testScript=$(cat "$(pwd)/testing/cloud_layouts/script.d/wait_cluster_ready/test_commander_script.sh")

  if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
    echo "Ingress and Istio test passed"
  else
    echo "Ingress and Istio test failure"
    return 1
  fi

  testOpenvpnReady=$(cat "$(pwd)/testing/cloud_layouts/script.d/wait_cluster_ready/test_openvpn_ready.sh")

  test_failed="true"
    if $ssh_command $ssh_bastion "$ssh_user@$master_ip" \
      sudo su -c /bin/bash <<<"${testOpenvpnReady}"; then
      test_failed=""
    else
      >&2 echo "OpenVPN test failed for Static provider. Sleeping 30 seconds..."
      sleep 30
    fi

    if [[ $test_failed == "true" ]]; then
      return 1
    fi
  if [[ $TEST_AUTOSCALER_ENABLED == "true" ]] ; then
    echo "Run Autoscaler test"
    testAutoscalerScript=$(cat "$(pwd)/testing/cloud_layouts/script.d/wait_cluster_ready/test_autoscaler.sh")
    testRunAttempts=5
    for ((i=1; i<=$testRunAttempts; i++)); do
      if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testAutoscalerScript}"; then
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

  if [[ -n ${SWITCH_TO_IMAGE_TAG} ]]; then
    echo "Starting switch deckhouse image"
    change_deckhouse_image "${SWITCH_TO_IMAGE_TAG}" || return $?
    wait_deckhouse_ready || return $?
    wait_upmeter_green || return $?
    wait_alerts_resolve || return $?

    testScript=$(cat "$(pwd)/testing/cloud_layouts/script.d/wait_cluster_ready/test_script.sh")

    if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
      echo "Ingress and Istio test passed"
    else
      echo "Ingress and Istio test failure"
      return 1
    fi
  fi
}

function cleanup() {
  #Get cluster id
  cluster_id=$(curl \
    "https://${COMMANDER_HOST}/api/v1/clusters" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: ${COMMANDER_TOKEN}" |
    jq -r ".[] | select(.name == \"${PREFIX}\") | .id")

  if [ -z $cluster_id ]; then
    echo "  Error getting cluster id"
    return 1
  fi

  echo "  Deleting cluster ${cluster_id}"

  response=$(curl -s -X 'DELETE' \
    "https://${COMMANDER_HOST}/api/v1/clusters/${cluster_id}" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: ${COMMANDER_TOKEN}" \
    -w "\n%{http_code}")

  http_code=$(echo "$response" | tail -n 1)
  response=$(echo "$response" | sed '$d')

  # Check for HTTP errors
  if [[ ${http_code} -ge 400 ]]; then
    echo "Error: HTTP error ${http_code}" >&2
    echo "$response" >&2
    return 1
  fi

  # Waiting to cluster cleanup
  testRunAttempts=40
  sleep=30
  for ((i=1; i<=testRunAttempts; i++)); do
    cluster_status="$(curl -s -X 'GET' \
      "https://${COMMANDER_HOST}/api/v1/clusters/${cluster_id}" \
      -H 'accept: application/json' \
      -H "X-Auth-Token: ${COMMANDER_TOKEN}" |
      jq -r '.status')"
    >&2 echo "Check Cluster delete..."
    echo "  Cluster status: $cluster_status"
    if [ "deleted" = "$cluster_status" ]; then
      return 0
    elif [ "deletion_failed" = "$cluster_status" ]; then
      return 1
    fi
    if [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n "  Cluster not deleted. Attempt $i/$testRunAttempts failed. Sleep for $sleep seconds..."
      sleep $sleep
    else
      >&2 echo -n "  Cluster not deleted. Attempt $i/$testRunAttempts failed."
      return 1
    fi
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
      exit 1
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
