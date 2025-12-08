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

  DVP:

\$LAYOUT_DVP_KUBECONFIGDATABASE64

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

\$LAYOUT_VSPHERE_USERNAME
\$LAYOUT_VSPHERE_PASSWORD
\$LAYOUT_VSPHERE_BASE_DOMAIN

  VCD:

\$LAYOUT_VCD_PASSWORD

  Static:

\$LAYOUT_OS_PASSWORD

  Static-cse:

\$LAYOUT_OS_PASSWORD

EOF
)


set -Eeo pipefail
shopt -s failglob

function create_registry() {
  registry_suffix=$(echo "$INSTALL_IMAGE_NAME" | sed -E 's|^[^/]*/([^/]+/[^/]+)/[^:]+:.*|\1|') # repo.url/project/path/install:prNum ==> project/path
  decode_dockercfg=$(base64 -d <<< "${1}")
  registry_address=$(jq -r '.auths | keys[]'  <<< "$decode_dockercfg")
  registry_auth=$(jq -r ".auths.\"${registry_address}\".auth" <<< "$decode_dockercfg")
  sleep_second=0
  payload="{
      \"name\": \"${PREFIX}\",
      \"images_repo\": \"${registry_address}/${registry_suffix}\",
      \"scheme\": \"https\",
      \"dev_branch\": \"${DEV_BRANCH}\",
      \"auth\": \"${registry_auth}\"
  }"
  for (( j=1; j<=5; j++ )); do
    sleep "$sleep_second"
    sleep_second=5

    response=$(curl -s -X POST  \
      "https://${COMMANDER_HOST}/api/v1/registries" \
      -H 'accept: application/json' \
      -H "X-Auth-Token: ${COMMANDER_TOKEN}" \
      -H 'Content-Type: application/json' \
      -d "$payload" \
      -w "\n%{http_code}")

    http_code=$(echo "$response" | tail -n 1)
    response_body=$(echo "$response" | sed '$d')

    # Check for HTTP errors
    if [[ ${http_code} -ge 400 ]]; then
      echo "Error: HTTP error ${http_code}" >&2
      echo "$response_body" >&2
      continue
    else
      registry_id=$(jq -r '.id' <<< "$response_body")
      break
    fi
  done

  if [[ -n "$registry_id" ]]; then
      echo "$registry_id"
  else
      echo "Failed to create registry." >&2
      return 1
  fi
}

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
      # Then update cluster to DECKHOUSE_IMAGE_TAG.
      # NOTE: currently only release branches are supported for updating.
      if [[ "${DECKHOUSE_IMAGE_TAG}" =~ release-([0-9]+\.[0-9]+) ]]; then
        DEV_BRANCH="${INITIAL_IMAGE_TAG}"
        SWITCH_TO_IMAGE_TAG="v${BASH_REMATCH[1]}.0"
        update_release_channel "$(echo -n "${STAGE_DECKHOUSE_DOCKERCFG}" | base64 -d | awk -F'\"' '{print $4}')/${REGISTRY_PATH}" "${SWITCH_TO_IMAGE_TAG}"
        echo "Will install '${DEV_BRANCH}' first and then update to '${DECKHOUSE_IMAGE_TAG}' as '${SWITCH_TO_IMAGE_TAG}'"
      else
        echo "'${DECKHOUSE_IMAGE_TAG}' doesn't look like a release branch."
        return 1
      fi
    else
      DEV_BRANCH="${DECKHOUSE_IMAGE_TAG}"
    fi

  case "$PROVIDER" in
  "Yandex.Cloud")
    CLOUD_ID=$LAYOUT_YANDEX_CLOUD_ID
    FOLDER_ID=$LAYOUT_YANDEX_FOLDER_ID
    SERVICE_ACCOUNT_JSON=$LAYOUT_YANDEX_SERVICE_ACCOUNT_KEY_JSON
    ssh_user="redos"
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

  "DVP")
    KUBECONFIGDATABASE64=$LAYOUT_DVP_KUBECONFIGDATABASE64
    ssh_user="debian"
    bastion_host="185.11.73.171"
    bastion_user="e2e-user"
    ssh_bastion="-J ${bastion_user}@${bastion_host}"

    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"kubeconfigDataBase64\": \"${KUBECONFIGDATABASE64}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"sshBastionHost\": \"${bastion_host}\",
      \"sshBastionUser\": \"${bastion_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "DVP-cse")
    cwd=$(pwd)/../testing/cloud_layouts/Static
    KUBECONFIGDATABASE64=$LAYOUT_DVP_KUBECONFIGDATABASE64
    ssh_user="altlinux"
    bastion_host="185.11.73.171"
    bastion_user="e2e-user"
    ssh_bastion="-J ${bastion_user}@${bastion_host}"

    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"kubeconfigDataBase64\": \"${KUBECONFIGDATABASE64}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"sshBastionHost\": \"${bastion_host}\",
      \"sshBastionUser\": \"${bastion_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "GCP")
    ssh_user="user"
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
    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"a${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"osPassword\": \"${LAYOUT_OS_PASSWORD}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"
    ;;

  "vSphere")
    ssh_user="redos"
    bastion_user="ubuntu"
    bastion_host="31.128.54.168"
    bastion_port="53359"
    ssh_bastion="-J ${bastion_user}@${bastion_host}:${bastion_port}"
    values="{
      \"branch\": \"${DEV_BRANCH}\",
      \"prefix\": \"${PREFIX}\",
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"masterCount\": \"${MASTERS_COUNT}\",
      \"vSphereUsername\": \"${LAYOUT_VSPHERE_USERNAME}\",
      \"vSpherePassword\": \"${LAYOUT_VSPHERE_PASSWORD}\",
      \"vSphereBaseDomain\": \"${LAYOUT_VSPHERE_BASE_DOMAIN}\",
      \"sshPrivateKey\": \"${SSH_KEY}\",
      \"sshUser\": \"${ssh_user}\",
      \"sshBastionHost\": \"${bastion_host}\",
      \"sshBastionUser\": \"${bastion_user}\",
      \"sshBastionPort\": \"${bastion_port}\",
      \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
      \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
    }"

    ;;

  "VCD")
    # shellcheck disable=SC2016
    cwd=$(pwd)/testing/cloud_layouts/VCD/Standard
    export VCD_USER="$LAYOUT_VCD_USERNAME"
    export VCD_PASSWORD="$LAYOUT_VCD_PASSWORD"
    export VCD_ORG="$LAYOUT_VCD_ORG"
    export VCD_VDC="${LAYOUT_VCD_ORG}-MSK1-S1-vDC2"
    export VCD_URL="$LAYOUT_VCD_SERVER/api"
    export TF_VAR_PREFIX="$PREFIX"
    export TF_VAR_VCD_ORG="$LAYOUT_VCD_ORG"
    export TF_VAR_VCD_VDC="${LAYOUT_VCD_ORG}-MSK1-S1-vDC2"
    ssh_user="ubuntu"
    ssh_bastion_ip="$LAYOUT_STATIC_BASTION_IP"
    ssh_bastion="-J ${ssh_user}@${ssh_bastion_ip}"
    values="{
    \"vcdUsername\": \"${LAYOUT_VCD_USERNAME}\",
    \"vcdPassword\": \"${LAYOUT_VCD_PASSWORD}\",
    \"vcdOrg\": \"${LAYOUT_VCD_ORG}\",
    \"vcdServer\": \"${LAYOUT_VCD_SERVER}\",
    \"branch\": \"${DEV_BRANCH}\",
    \"prefix\": \"${PREFIX}\",
    \"kubeVersion\": \"${KUBERNETES_VERSION}\",
    \"defaultCRI\": \"${CRI}\",
    \"masterCount\": \"${MASTERS_COUNT}\",
    \"sshPrivateKey\": \"${SSH_KEY}\",
    \"sshUser\": \"${ssh_user}\",
    \"sshBastionHost\": \"${ssh_bastion_ip}\",
    \"sshBastionUser\": \"${ssh_user}\",
    \"deckhouseDockercfg\": \"${DECKHOUSE_DOCKERCFG}\",
    \"flantDockercfg\": \"${FOX_DOCKERCFG}\"
  }"
    ;;

  "Static")
    cwd=$(pwd)/testing/cloud_layouts/Static
    export TF_VAR_OS_PASSWORD="$LAYOUT_OS_PASSWORD"
    export TF_VAR_PREFIX="$PREFIX"

    # use different users for different OSs
    ssh_user="astra"
    ssh_user_system="altlinux"
    ssh_redos_user_worker="redos"
    ssh_opensuse_user_worker="opensuse"
    ssh_rosa_user_worker="centos"

    ;;

  "Static-cse")
    cwd=$(pwd)/../testing/cloud_layouts/Static
    export TF_VAR_OS_PASSWORD="$LAYOUT_OS_PASSWORD"
    export TF_VAR_PREFIX="$PREFIX"

    # use different users for different OSs
    ssh_astra_user="astra"
    ssh_alt_user="altlinux"
    ssh_redos_user="redos"
    ssh_mosos_user="opensuse"
    ssh_user="$ssh_astra_user"
    ssh_user_system="$ssh_mosos_user"

    ;;
  esac
}

function get_opentofu() {
  rm -rf $cwd/plugins
  CONTAINER_ID=$(docker create "${INSTALL_IMAGE_NAME}")
  docker cp "${CONTAINER_ID}:/bin/opentofu" $cwd/opentofu
  docker cp "${CONTAINER_ID}:/plugins" $cwd/
  docker rm "$CONTAINER_ID"
  chmod +x $cwd/opentofu
  cp -r $cwd/plugins/registry.terraform.io/terraform-provider-openstack $cwd/plugins/registry.opentofu.org/terraform-provider-openstack
  cp -r $cwd/plugins/registry.terraform.io/vmware $cwd/plugins/registry.opentofu.org/vmware
}
function bootstrap_vcd() {
   >&2 echo "Run terraform to create vapp for vcd cluster ..."
   cd $cwd

  pwd
   get_opentofu
   $cwd/opentofu init -plugin-dir $cwd/plugins -input=false -backend-config="key=${TF_VAR_PREFIX}" || return $?
   $cwd/opentofu  apply -auto-approve -no-color | tee "$cwd/terraform.log" || return $?
}
function bootstrap_static() {
  >&2 echo "Run terraform to create nodes for Static cluster ..."

  cd $cwd

  get_opentofu

  if [[ ${PROVIDER} == "Static" ]]; then
    $cwd/opentofu init -plugin-dir $cwd/plugins -input=false -backend-config="key=${TF_VAR_PREFIX}" || return $?
  elif [[ ${PROVIDER} == "Static-cse" ]]; then
    $cwd/opentofu init -input=false -backend-config="key=${TF_VAR_PREFIX}" || return $?
  fi

  $cwd/opentofu apply -auto-approve -no-color | tee "$cwd/terraform.log" || return $?

  if [[ ${PROVIDER} == "Static" ]]; then

    if ! master_ip="$($cwd/opentofu output -raw master_ip_address_for_ssh)"; then
      >&2 echo "ERROR: can't get master_ip from opentofu output"
      return 1
    fi

    if ! system_ip="$($cwd/opentofu output -raw system_ip_address_for_ssh)"; then
      >&2 echo "ERROR: can't get system_ip from opentofu output"
      return 1
    fi

    if ! worker_redos_ip="$($cwd/opentofu output -raw worker_redos_ip_address_for_ssh)"; then
      >&2 echo "ERROR: can't get worker_redos_ip from opentofu output"
      return 1
    fi

    if ! worker_opensuse_ip="$($cwd/opentofu output -raw worker_opensuse_ip_address_for_ssh)"; then
      >&2 echo "ERROR: can't get worker_opensuse_ip from opentofu output"
      return 1
    fi

    if ! worker_rosa_ip="$($cwd/opentofu output -raw worker_rosa_ip_address_for_ssh)"; then
      >&2 echo "ERROR: can't get worker_rosa_ip from opentofu output"
      return 1
    fi

    if ! bastion_ip="$($cwd/opentofu output -raw bastion_ip_address_for_ssh)"; then
      >&2 echo "ERROR: can't get bastion_ip from opentofu output"
      return 1
    fi

  elif [[ ${PROVIDER} == "Static-cse" ]]; then
    if ! bastion_ip="$($cwd/opentofu output -raw bastion_ip_address_for_ssh)"; then # todo change to opentofu from $cwd
      >&2 echo "ERROR: can't get bastion_ip from opentofu output"
      return 1
    fi

    if ! master_ip="$($cwd/opentofu output -json node_ip_address_for_ssh | jq -r '.master1_ssh_addr' )"; then
      >&2 echo "ERROR: can't get master_ip from opentofu output"
      return 1
    fi

    if ! master2_ip="$($cwd/opentofu output -json node_ip_address_for_ssh | jq -r '.master2_ssh_addr')"; then
      >&2 echo "ERROR: can't get master2_ip from opentofu output"
      return 1
    fi

    if ! master3_ip="$($cwd/opentofu output -json node_ip_address_for_ssh | jq -r '.master3_ssh_addr')"; then
      >&2 echo "ERROR: can't get master3_ip from opentofu output"
      return 1
    fi

    if ! system_ip="$($cwd/opentofu output -json node_ip_address_for_ssh | jq -r '.system_ssh_addr')"; then
      >&2 echo "ERROR: can't get system_ip from opentofu output"
      return 1
    fi

    if ! worker1_ip="$($cwd/opentofu output -json node_ip_address_for_ssh | jq -r '.worker1_ssh_addr')"; then
      >&2 echo "ERROR: can't get worker1_ip from opentofu output"
      return 1
    fi

    if ! worker2_ip="$($cwd/opentofu output -json node_ip_address_for_ssh | jq -r '.worker2_ssh_addr')"; then
      >&2 echo "ERROR: can't get worker2_ip from opentofu output"
      return 1
    fi

    if ! worker3_ip="$($cwd/opentofu output -json node_ip_address_for_ssh | jq -r '.worker3_ssh_addr')"; then
      >&2 echo "ERROR: can't get worker3_ip from opentofu output"
      return 1
    fi

  fi


  # Add key to access to hosts thru bastion
  set_common_ssh_parameters
  eval "$(ssh-agent -s)"
  ssh-add "id_rsa"
  scp_command="scp -S /usr/bin/ssh -F /tmp/cloud-test-ssh-config"
  ssh_bastion="-J $ssh_user@$bastion_ip"

  if [[ ${PROVIDER} == "Static" ]]; then

    D8_MIRROR_USER="$(echo -n ${DECKHOUSE_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f1)"
    D8_MIRROR_PASSWORD="$(echo -n ${DECKHOUSE_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f2)"
    D8_MIRROR_HOST=$(echo -n "${DECKHOUSE_DOCKERCFG}" | base64 -d | awk -F'\"' '{print $4}')

    D8_MODULES_USER="$(echo -n ${DECKHOUSE_E2E_MODULES_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f1)"
    D8_MODULES_PASSWORD="$(echo -n ${DECKHOUSE_E2E_MODULES_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f2)"
    D8_MODULES_HOST=$(echo -n "${DECKHOUSE_E2E_MODULES_DOCKERCFG}" | base64 -d | awk -F'\"' '{print $4}')

    E2E_REGISTRY_USER="$(echo -n ${DECKHOUSE_E2E_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f1)"
    E2E_REGISTRY_PASSWORD="$(echo -n ${DECKHOUSE_E2E_DOCKERCFG} | base64 -d | awk -F'\"' '{ print $8 }' | base64 -d | cut -d':' -f2)"
    E2E_REGISTRY_HOST=$(echo -n "${DECKHOUSE_E2E_DOCKERCFG}" | base64 -d | awk -F'\"' '{print $4}')

    IMAGES_REPO="${E2E_REGISTRY_HOST}/sys/deckhouse-oss"
    D8_MODULES_URL="${D8_MODULES_HOST}/deckhouse/ee"
    testRunAttempts=20
    for ((i=1; i<=$testRunAttempts; i++)); do
      # Install http/https proxy on bastion node
      if $ssh_command "$ssh_user@$bastion_ip" sudo su -c /bin/bash <<ENDSSH; then
         cat <<'EOF' > /tmp/install-d8-and-pull-push-images.sh
#!/bin/bash
apt-get update
apt-get install -y docker.io docker-compose wget curl chrony
# setup chrony
cat << 'CONF' > /etc/chrony/chrony.conf
bindaddress 0.0.0.0
bindaddress ::
server time.google.com iburst
local stratum 10
allow 192.168.199.0/24
CONF
echo DAEMON_OPTS="-F 1 -f /etc/chrony/chrony.conf" > /etc/default/chrony
systemctl daemon-reexec
systemctl enable --now chronyd
systemctl restart chronyd
chronyc tracking
# get latest d8-cli release
URL="https://api.github.com/repos/deckhouse/deckhouse-cli/releases/latest"
# DOWNLOAD_URL=\$(wget -qO- "\${URL}" | grep browser_download_url | cut -d '"' -f 4 | grep linux-amd64 | grep -v sha256sum)
# if [ -z "\${DOWNLOAD_URL}" ]; then
#   echo "Failed to retrieve the URL for the download"
#   exit 1
# fi
# download
DOWNLOAD_URL=https://github.com/deckhouse/deckhouse-cli/releases/download/v0.15.0/d8-v0.15.0-linux-amd64.tar.gz
wget -qL "\${DOWNLOAD_URL}" -O /tmp/d8.tar.gz
file /tmp/d8.tar.gz
mkdir d8cli
tar -xf /tmp/d8.tar.gz -C d8cli
mv ./d8cli/linux-amd64/bin/d8 /usr/bin/d8

d8 --version

#download crane
wget -q "https://github.com/google/go-containerregistry/releases/download/v0.20.6/go-containerregistry_Linux_x86_64.tar.gz" -O "/tmp/crane.tar.gz"
mkdir crane
tar -xf /tmp/crane.tar.gz -C crane
mv crane/crane /usr/bin/crane

crane version

# pull
d8 mirror pull d8-modules \
  --source "${D8_MODULES_URL}" \
  --source-login "${D8_MODULES_USER}" \
  --source-password "${D8_MODULES_PASSWORD}" \
  --include-module commander-agent --include-module commander --include-module prompp  --include-module pod-reloader  --include-module runtime-audit-engine --no-platform  --no-security-db

d8 mirror pull d8 --source-login ${D8_MIRROR_USER} --source-password ${D8_MIRROR_PASSWORD} \
  --source "dev-registry.deckhouse.io/sys/deckhouse-oss" --deckhouse-tag "${DEV_BRANCH}"
# push
d8 mirror push d8 "${IMAGES_REPO}" --registry-login ${E2E_REGISTRY_USER} --registry-password ${E2E_REGISTRY_PASSWORD} --insecure
d8 mirror push d8-modules "${IMAGES_REPO}" --registry-login ${E2E_REGISTRY_USER} --registry-password ${E2E_REGISTRY_PASSWORD} --insecure

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
    >&2 echo "Mirroring the updated version..."
    # pull
    d8 mirror pull d8-upgrade --source-login ${D8_MIRROR_USER} --source-password ${D8_MIRROR_PASSWORD} \
    --source "${D8_MIRROR_HOST}/sys/deckhouse-oss" --deckhouse-tag "${DECKHOUSE_IMAGE_TAG}"
    # push
    d8 mirror push d8-upgrade "${IMAGES_REPO}" --registry-login ${E2E_REGISTRY_USER} --registry-password ${E2E_REGISTRY_PASSWORD} --insecure
    >&2 echo "Copying the release-channel images..."
    crane auth login "${D8_MIRROR_HOST}" -u "${D8_MIRROR_USER}" -p "${D8_MIRROR_PASSWORD}"
    crane auth login "${E2E_REGISTRY_HOST}" -u "${E2E_REGISTRY_USER}" -p "${E2E_REGISTRY_PASSWORD}"
    crane copy "${D8_MIRROR_HOST}/sys/deckhouse-oss/release-channel:beta" "${IMAGES_REPO}/release-channel:beta"
    crane copy "${IMAGES_REPO}/install:${DECKHOUSE_IMAGE_TAG}" "${IMAGES_REPO}/install:${SWITCH_TO_IMAGE_TAG}"
    crane copy "${IMAGES_REPO}:${DECKHOUSE_IMAGE_TAG}" "${IMAGES_REPO}:${SWITCH_TO_IMAGE_TAG}"
    crane auth logout "${D8_MIRROR_HOST}"
    crane auth logout "${E2E_REGISTRY_HOST}"
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
      if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<ENDSSH; then
         echo "echo Master ip"
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
      if $ssh_command $ssh_bastion "$ssh_user_system@$system_ip" sudo su -c /bin/bash <<ENDSSH; then
         echo "echo System ip"
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
      if $ssh_command $ssh_bastion "$ssh_redos_user_worker@$worker_redos_ip" sudo su -c /bin/bash <<ENDSSH; then
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
      if $ssh_command $ssh_bastion "$ssh_opensuse_user_worker@$worker_opensuse_ip" sudo su -c /bin/bash <<ENDSSH; then
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
      if $ssh_command $ssh_bastion "$ssh_rosa_user_worker@$worker_rosa_ip" sudo su -c /bin/bash <<ENDSSH; then
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

  fi
}

function system_node_register() {

  >&2 echo "==============================================================

  Cluster bootstrapped. Register 'system' and 'worker' nodes and starting the test now.

  If you'd like to pause the cluster deletion for debugging:
   1. ssh to cluster: 'ssh $ssh_user@$master_ip'
   2. execute 'kubectl create configmap pause-the-test'

=============================================================="

  >&2 echo 'Fetch registration script ...'
  for ((i=0; i<10; i++)); do
    bootstrap_system="$($ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash << "ENDSSH"
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
  $ssh_command $ssh_bastion "$ssh_user_system@$system_ip" sudo su -c /bin/bash <<ENDSSH || true
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
base64 -d <<< "$bootstrap_system" | bash
ENDSSH

  registration_failed=
  >&2 echo 'Waiting until Node registration finishes ...'
  for ((i=1; i<=20; i++)); do
    if [[ "$PROVIDER" == "Static" ]]; then
      target_count="5"
    elif [[ "$PROVIDER" == "Static-cse" ]]; then
      target_count="7"
    fi

    if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<EOF
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl get nodes -o wide
kubectl get nodes -o json | jq -re ".items | length == ${target_count}" >/dev/null
kubectl get nodes -o json | jq -re '[ .items[].status.conditions[] | select(.type == "Ready") ] | map(.status == "True") | all'
EOF
    then
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
      if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<"ENDSSH"; then
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
EOF
  echo ${SSH_KEY} | base64 -d > id_rsa
  chmod 600 id_rsa
  # ssh command with common args.
  ssh_command="ssh -F /tmp/cloud-test-ssh-config -i id_rsa "
  eval "$(ssh-agent -s)"
  ssh-add "id_rsa"
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
  "D8IstioPodsWithoutIstioSidecar" # Expected behaviour in clusters that start too quickly, and tests do start quickly
  "LoadAverageHigh" # Pointless, as test servers have minimal resources
  "SecurityEventsDetected" # This is normal for e2e tests
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
  iterations=60
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
  local testRunAttempts=20
  echo "Check applied resource status..."
  for ((i=1; i<=testRunAttempts; i++)); do
    response=$(get_cluster_status)
    errors=$(jq -c '
      .resources_state_results[]
      | select(.errors)
      | .errors |= map(select(test("vstaticinstancev1alpha1.deckhouse.io") | not))
      | select(.errors | length > 0)
      | .errors
    ' <<< "$response")
    if [ -n "$errors" ]; then
      if [[ $i -lt $testRunAttempts ]]; then
        echo "  Errors found. Attempt $i/$testRunAttempts failed. Sleep for 30 seconds..."
        sleep 30
        continue
      else
        echo "  Attempt $i/$testRunAttempts failed."
        echo "${errors}"
        return 1
      fi
    else
      echo "Check applied resource status... Passed"
      return 0
    fi
  done
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


# update_release_channel changes the release-channel image to given tag
function update_release_channel() {
  crane copy "$1/release-channel:$2" "$1/release-channel:beta"
}

# trigger_deckhouse_update sets the release channel for the cluster, prompting it to upgrade to the next version.
function trigger_deckhouse_update() {
  >&2 echo "Setting Deckhouse release channel to Beta."
  if ! $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<ENDSSH; then
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl patch mc/deckhouse -p '{"spec": {"settings": {"releaseChannel": "Beta"}}}' --type=merge
ENDSSH
    >&2 echo "Cannot change Deckhouse release channel."
    return 1
  fi
}

# wait_update_ready checks if the cluster is ready for updating.
function wait_update_ready() {
  expectedVersion="$1"
  testScript=$(cat <<"END_SCRIPT"
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl get deckhouserelease -o 'jsonpath={.items[?(@.status.phase=="Deployed")].spec.version}'
END_SCRIPT
)

  testRunAttempts=20
  for ((i=1; i<=$testRunAttempts; i++)); do
    >&2 echo "Check DeckhouseRelease..."
    deployedVersion="$($ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}")"
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

function wait_prom_rules_mutating_ready() {
  testScript=$(cat <<"END_SCRIPT"
export PATH="/opt/deckhouse/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
export LANG=C
set -Eeuo pipefail
kubectl get pods -l app=prom-rules-mutating
[[ "$(kubectl get pods -l app=prom-rules-mutating -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}{..status.phase}')" ==  "TrueRunning" ]]
END_SCRIPT
)

  testRunAttempts=60
  for ((i=1; i<=testRunAttempts; i++)); do
    >&2 echo "Check prom-rules-mutating pod readiness..."
    if $ssh_command $ssh_bastion "$ssh_user@$master_ip" sudo su -c /bin/bash <<<"${testScript}"; then
      return 0
    fi

    if [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n "  prom-rules-mutating pod not ready. Attempt $i/$testRunAttempts failed. Sleep for 30 seconds..."
      sleep 30
    else
      >&2 echo -n "  prom-rules-mutating pod not ready. Attempt $i/$testRunAttempts failed."
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

  local connection_str_body="${PROVIDER}-${LAYOUT}-${CRI}-${KUBERNETES_VERSION} - Connection string: \`ssh ${ssh_bastion} ${master_connection}\`"
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

  if [[ "$PROVIDER" == "Static" ]]; then
    echo "Provider = $PROVIDER: switch registry"
    registry_id=$(create_registry "${DECKHOUSE_E2E_DOCKERCFG}")
  elif [[ "$DEV_BRANCH" =~ ^release-[0-9]+\.[0-9]+ ]]; then
    echo "DEV_BRANCH = $DEV_BRANCH: detected release branch"
    registry_id=$(create_registry "${STAGE_DECKHOUSE_DOCKERCFG}")
  else
    registry_id=$(create_registry "${DECKHOUSE_DOCKERCFG}")
  fi

  if [[ ${PROVIDER} == "Static" ]]; then
      bootstrap_static || return $?
      values="{
            \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
            \"defaultCRI\": \"${CRI}\",
            \"sshMasterHost\": \"${master_ip}\",
            \"sshMasterUser\": \"${ssh_user}\",
            \"sshBastionHost\": \"${bastion_ip}\",
            \"sshBastionUser\": \"${ssh_user}\",
            \"sshRedosHost\": \"${worker_redos_ip}\",
            \"sshRedosUser\": \"${ssh_redos_user_worker}\",
            \"sshOpensuseHost\": \"${worker_opensuse_ip}\",
            \"sshOpensuseUser\": \"${ssh_opensuse_user_worker}\",
            \"sshRosaHost\": \"${worker_rosa_ip}\",
            \"sshRosaUser\": \"${ssh_rosa_user_worker}\",
            \"sshPrivateKey\": \"${SSH_KEY}\",
            \"imagesRepo\": \"${IMAGES_REPO}\",
            \"branch\": \"${DEV_BRANCH}\",
            \"deckhouseDockercfg\": \"${DECKHOUSE_E2E_DOCKERCFG}\"
          }"
  elif [[ ${PROVIDER} == "Static-cse" ]]; then
    bootstrap_static || return $?
    values="{
      \"kubernetesVersion\": \"${KUBERNETES_VERSION}\",
      \"defaultCRI\": \"${CRI}\",
      \"sshMaster1Host\": \"${master_ip}\",
      \"sshMaster1User\": \"${ssh_astra_user}\",
      \"sshMaster2Host\": \"${master2_ip}\",
      \"sshMaster2User\": \"${ssh_redos_user}\",
      \"sshMaster3Host\": \"${master3_ip}\",
      \"sshMaster3User\": \"${ssh_alt_user}\",
      \"sshBastionHost\": \"${bastion_ip}\",
      \"sshBastionUser\": \"${ssh_astra_user}\",
      \"sshWorker1Host\": \"${worker1_ip}\",
      \"sshWorker1User\": \"${ssh_alt_user}\",
      \"sshWorker2Host\": \"${worker2_ip}\",
      \"sshWorker2User\": \"${ssh_redos_user}\",
      \"sshWorker3Host\": \"${worker3_ip}\",
      \"sshWorker3User\": \"${ssh_astra_user}\",
      \"sshSystemHost\": \"${system_ip}\",
      \"sshSystemUser\": \"${ssh_mosos_user}\",
      \"sshPrivateKey\": \"${SSH_KEY}\"
    }"
  fi
  if [[ ${PROVIDER} == "VCD" ]]; then
      bootstrap_vcd || return $?
  fi
  cluster_template_version_id=$(curl -s -X 'GET' \
    "https://${COMMANDER_HOST}/api/v1/cluster_templates/${TEMPLATE_ID}?without_archived=true" \
    -H 'accept: application/json' \
    -H "X-Auth-Token: ${COMMANDER_TOKEN}" |
    jq -r 'del(.cluster_template_versions).current_cluster_template_version_id')

  payload="{
    \"name\": \"${PREFIX}\",
    \"registry_id\": \"${registry_id}\",
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
    echo http_code: $http_code

    # Check for HTTP errors
    if [[ "$http_code" -ge 200 && "$http_code" -lt 300 ]]; then
      break
    else
      echo "Error: HTTP error ${http_code}" >&2
      echo "$response" >&2
      continue
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
    if [[ "$master_ip_find" == "false" ]]; then
      master_ip=$(jq -r '.connection_hosts.masters[0].host' <<< "$response")
      master_user=$(jq -r '.connection_hosts.masters[0].user' <<< "$response")
      if [[ "$master_ip" != "null" && "$master_user" != "null" ]]; then
        master_connection="${master_user}@${master_ip}"
        master_ip_find=true
        echo "  SSH connection string:"
        echo "      ssh ${ssh_bastion} ${master_connection}"
        update_comment
        echo "$master_connection" > ssh-connect_str-"${PREFIX}"
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

  if [[ "$PROVIDER" == "Static" ]] || [[ "$PROVIDER" == "Static-cse" ]]; then
    system_node_register || return $?
  fi

  wait_upmeter_green || return $?

  check_resources_state_results || return $?

  wait_alerts_resolve || return $?

  set_common_ssh_parameters
  if [[ "$PROVIDER" != "Static-cse" && "$PROVIDER" != "DVP-cse" ]]; then
    wait_prom_rules_mutating_ready || return $?
  else
    echo "Use ${PROVIDER} provider, skipping prom_rules_mutating_ready check, continue..."
  fi

  if [[ "$PROVIDER" != "Static-cse" && "$PROVIDER" != "DVP-cse" ]]; then
    testScript="${GITHUB_WORKSPACE}/testing/cloud_layouts/script.d/wait_cluster_ready/test_commander_script.sh"
  else
    testScript="${cwd}/../../../deckhouse/testing/cloud_layouts/script.d/wait_cluster_ready/test_commander_script.sh"
  fi

  testRunAttempts=5
  $ssh_command $ssh_bastion "$ssh_user@$master_ip" "cat > /tmp/test.sh" < "${testScript}"
  for ((i=1; i<=testRunAttempts; i++)); do
    if $ssh_command $ssh_bastion "$ssh_user@$master_ip" "sudo bash /tmp/test.sh"; then
      echo "Ingress and Istio test passed"
      break
    fi
    if [[ $i -lt $testRunAttempts ]]; then
      >&2 echo -n " Ingress and Istio test. Attempt $i/$testRunAttempts failed. Sleep for 30 seconds..."
      sleep 30
    else
      >&2 echo -n "  Ingress and Istio test. Attempt $i/$testRunAttempts failed."
      return 1
    fi
  done

  if [[ $TEST_AUTOSCALER_ENABLED == "true" ]] ; then
    echo "Run Autoscaler test"
    testAutoscalerScript=$(cat "${GITHUB_WORKSPACE}/testing/cloud_layouts/script.d/wait_cluster_ready/test_autoscaler.sh")
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
    echo "Starting Deckhouse update..."
    trigger_deckhouse_update || return $?
    wait_update_ready "${SWITCH_TO_IMAGE_TAG}"|| return $?
    wait_deckhouse_ready || return $?
    wait_upmeter_green || return $?
    wait_alerts_resolve || return $?

    testScript=$(cat "${GITHUB_WORKSPACE}/testing/cloud_layouts/script.d/wait_cluster_ready/test_script.sh")

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

  if [ -z "$cluster_id" ] && { [ "$PROVIDER" = "Static" ] || [ "$PROVIDER" = "Static-cse" ] || [ "$PROVIDER" = "VCD" ]; }; then
    echo "  Error getting cluster id, but provider is Static or VCD, continue"
  elif [ -z $cluster_id ]; then
    echo "  Error getting cluster id"
    return 1
  else
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
    testRunAttempts=80
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
        echo "  Cluster deleted"
        break
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
  fi


  if [[ ${PROVIDER} == "Static" ]] || [[ "$PROVIDER" == "Static-cse" ]] || [[ ${PROVIDER} == "VCD" ]]; then
    cd $cwd

    get_opentofu

    if [[ ${PROVIDER} == "Static" ]] || [[ ${PROVIDER} == "VCD" ]]; then
      $cwd/opentofu init -plugin-dir $cwd/plugins -input=false -backend-config="key=${TF_VAR_PREFIX}" || return $?
    elif [[ ${PROVIDER} == "Static-cse" ]]; then
      $cwd/opentofu init -input=false -backend-config="key=${TF_VAR_PREFIX}" || return $?
    fi

    $cwd/opentofu destroy -auto-approve -no-color | tee "$cwd/terraform.log" || return $?
  fi

  return 0

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
