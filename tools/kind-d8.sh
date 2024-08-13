#!/bin/bash

# Copyright 2022 Flant JSC
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

CONFIG_DIR=~/.kind-d8
KIND_IMAGE=kindest/node:v1.27.13@sha256:17439fa5b32290e3ead39ead1250dca1d822d94a10d26f1981756cd51b24b9d8
D8_RELEASE_CHANNEL_TAG=stable
D8_RELEASE_CHANNEL_NAME=Stable
D8_REGISTRY_ADDRESS=registry.deckhouse.io
D8_REGISTRY_PATH=${D8_REGISTRY_ADDRESS}/deckhouse/ce
D8_LICENSE_KEY=

KIND_INSTALL_DIRECTORY=$CONFIG_DIR
KIND_PATH=kind
KIND_CLUSTER_NAME=d8
KIND_VERSION=v0.23.0

KUBECTL_INSTALL_DIRECTORY=$CONFIG_DIR
KUBECTL_PATH=kubectl
KUBECTL_VERSION=v1.27.13

REQUIRE_MEMORY_MIN_BYTES=4000000000 # 4GB

usage() {
  printf "
 Usage: %s [--channel <CHANNEL NAME>] [--key <DECKHOUSE EE LICENSE KEY>] [--os <linux|mac>]

    --channel <CHANNEL NAME>
            Deckhouse Kubernetes Platform release channel name.
            Possible values: Alpha, Beta, EarlyAccess, Stable, RockSolid.
            Default: Stable.

    --key <DECKHOUSE EE LICENSE KEY>
            Deckhouse Kubernetes Platform Enterprise Edition license key.
            If no license key specified, Deckhouse Kubernetes Platform Community Edition will be installed.

    --os <linux|mac>
            Override the OS detection.

    --help|-h
            Print this message.

" "$0"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
    --channel)
      case "$2" in
      "")
        echo "Release channel is empty. Please specify the release channel name."
        usage
        exit 1
        ;;
      *)
        if [[ "$2" =~ ^(Alpha|Beta|EarlyAccess|Stable|RockSolid)$ ]]; then
          D8_RELEASE_CHANNEL_NAME="$2"
          D8_RELEASE_CHANNEL_TAG=$(echo ${D8_RELEASE_CHANNEL_NAME} | sed 's/EarlyAccess/early-access/; s/RockSolid/rock-solid/' | tr '[:upper:]' '[:lower:]')
        else
          echo "Incorrect release channel. Use Alpha, Beta, EarlyAccess, Stable or RockSolid."
          usage
          exit 1
        fi
        shift
        ;;
      esac
      ;;
    --key)
      case "$2" in
      "")
        echo "License key is empty. Please specify the license key or don't use the --key parameter to install Deckhouse Kubernetes Platform Community Edition."
        usage
        exit 1
        ;;
      *)
        D8_LICENSE_KEY="$2"
        D8_REGISTRY_PATH=${D8_REGISTRY_ADDRESS}/deckhouse/ee
        shift
        ;;
      esac
      ;;
    --os)
      case "$2" in
      "")
        echo "Please specify 'linux' or 'mac' for the --os parameter."
        usage
        exit 1
        ;;
      *)
        OS_NAME="$2"
        shift
        ;;
      esac
      ;;
    --help | -h)
      usage
      exit 1
      ;;
    --*)
      echo "Illegal option $1"
      usage
      exit 1
      ;;
    esac
    shift $(($# > 0 ? 1 : 0))
  done
}

os_detect() {
  if [[ (-z "$OS_NAME") ]]; then
    # some systems dont have lsb-release yet have the lsb_release binary and
    # vice-versa
    if [ -e /etc/lsb-release ]; then
      . /etc/lsb-release

      OS_NAME=${DISTRIB_ID}

    elif [ "$(which lsb_release 2>/dev/null)" ]; then
      OS_NAME=$(lsb_release -i | cut -f2 | awk '{ print tolower($1) }')

    elif [ -e /etc/debian_version ]; then
      # some Debians have jessie/sid in their /etc/debian_version
      # while others have '6.0.7'
      OS_NAME=$(cat /etc/issue | head -1 | awk '{ print tolower($1) }')

    elif [[ "$OSTYPE" == 'darwin'* ]]; then
      OS_NAME=mac

    else
      noop # Unknown OS
    fi
  fi

  OS_NAME="${OS_NAME// /}"

  # Supported on ...
  if [[ ("$OS_NAME" == "Ubuntu") || ("$OS_NAME" == "Debian") ]]; then
    OS_NAME=linux
  elif [[ ("$OS_NAME" != "mac") && ("$OS_NAME" != "linux") ]]; then
    OS_NAME=
  fi

  if [ -z "$OS_NAME" ]; then
    printf "Your operating system distribution and version might not supported by this script.

You can override the OS detection by setting the --os parameter to running this script.

E.g, to force Linux: --os linux
"

    exit 1
  fi

  MACHINE_ARCH=$(uname -m)

  echo "Detected operating system as $OS_NAME (${MACHINE_ARCH:-unknown})."
}

prerequisites_check() {
  echo "Checking for docker..."
  if command -v docker >/dev/null; then
    echo "Detected docker..."
  else
    echo "docker is not installed. Please install docker. You may go to https://docs.docker.com/engine/install/ for details."
    exit 1
  fi

  memory_check
  kubectl_check
  kind_check
  preinstall_checks
}

memory_check() {
  if [[ "$OS_NAME" == "linux" ]]; then
    MEMORY_TOTAL_BYTES=$(free --bytes 2>/dev/null | grep -i mem | awk '{print $2}' 2>/dev/null)
  else
    MEMORY_TOTAL_BYTES=$(sysctl -n hw.memsize 2>/dev/null)
  fi

  if [[ ("$MEMORY_TOTAL_BYTES" -gt "0") && ("$MEMORY_TOTAL_BYTES" -lt "$REQUIRE_MEMORY_MIN_BYTES") ]]; then
    echo "Insufficient memory to install Deckhouse Kubernetes Platform."
    echo "Deckhouse Kubernetes Platform requires at least 4 gigabytes of memory."
    exit 1
  fi

  if [[ ("$MEMORY_TOTAL_BYTES" -eq "0") || (-z "$MEMORY_TOTAL_BYTES") ]]; then
    echo "Can't get the total memory value."
    echo "Note, that Deckhouse Kubernetes Platform requires at least 4 gigabytes of memory."
    echo "Press enter to continue..."
    read
  fi
}

kubectl_check() {
  echo "Checking for kubectl..."
  if command -v kubectl >/dev/null; then
    echo "Detected kubectl..."
  elif command -v ${KUBECTL_INSTALL_DIRECTORY}/kubectl >/dev/null; then
    echo "Detected ${KUBECTL_INSTALL_DIRECTORY}/kubectl..."
    KUBECTL_PATH=${KUBECTL_INSTALL_DIRECTORY}/kubectl
  else
    echo "kubectl is not installed."
    while [[ "$should_install_kubectl" != "y" ]]; do
      read -rp "Install kubectl? y/[n]: " should_install_kubectl

      if [[ ("$should_install_kubectl" == "n") || (-z "$should_install_kubectl") ]]; then
        printf "
Please install kubectl.

You can find the installation instruction here: https://kubernetes.io/docs/tasks/tools/#kubectl
"
        exit 1
      fi

      if [[ "$should_install_kubectl" != "y" ]]; then
        echo "Please type 'y' to continue or 'n' to abort the installation."
      fi

    done

    read -rp "kubectl installation directory [$KUBECTL_INSTALL_DIRECTORY]: " kubectl_install_directory_answer
    if [[ -n "$kubectl_install_directory_answer" ]]; then
      KUBECTL_INSTALL_DIRECTORY=$kubectl_install_directory_answer
    fi

    echo "Installing the latest stable kubectl version to ${KUBECTL_INSTALL_DIRECTORY}/kubectl ..."

    mkdir -p $KUBECTL_INSTALL_DIRECTORY
    curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${OS_NAME/mac/darwin}/${MACHINE_ARCH/x86_64/amd64}/kubectl"

    if [ "$?" -ne "0" ]; then
      echo "Unable to download kubectl."
      exit 1
    fi

    install -m 0755 kubectl "${KUBECTL_INSTALL_DIRECTORY}"/kubectl
    if [ "$?" -ne "0" ]; then
      echo "Insufficient permissions to install kubectl. Trying again with sudo..."
      sudo install -m 0755 kubectl "${KUBECTL_INSTALL_DIRECTORY}"/kubectl
      if [ "$?" -ne "0" ]; then
        echo "Unable to install kubectl. Check installation path and permissions."
        exit 1
      fi
    fi

    KUBECTL_PATH=${KUBECTL_INSTALL_DIRECTORY}/kubectl
  fi
}

kind_check() {
  echo "Checking for kind $KIND_VERSION..."
  if [[ "v$(kind version -q 2>/dev/null)" == "$KIND_VERSION" ]]; then
    echo "Detected kind $KIND_VERSION..."
  elif [[ "v$(${KIND_INSTALL_DIRECTORY}/kind version -q 2>/dev/null)" == "$KIND_VERSION" ]]; then
    echo "Detected ${KIND_INSTALL_DIRECTORY}/kind..."
    KIND_PATH=${KIND_INSTALL_DIRECTORY}/kind
  else
    echo "kind is not installed or is not $KIND_VERSION version."
    while [[ "$should_install_kind" != "y" ]]; do
      read -rp "Install kind? y/[n]: " should_install_kind

      if [[ ("$should_install_kind" == "n") || (-z "$should_install_kind") ]]; then
        printf "
Please install kind.

You can find the installation instruction here: https://kind.sigs.k8s.io/docs/user/quick-start/#installation
"
        exit 1
      fi

      if [[ "$should_install_kind" != "y" ]]; then
        echo "Please type 'y' to continue or 'n' to abort the installation."
      fi

    done

    read -rp "kind installation directory [$KIND_INSTALL_DIRECTORY]: " kind_install_directory_answer
    if [[ -n "$kind_install_directory_answer" ]]; then
      KIND_INSTALL_DIRECTORY=$kind_install_directory_answer
    fi

    echo "Installing kind to ${KIND_INSTALL_DIRECTORY}/kind ..."

    mkdir -p ${KIND_INSTALL_DIRECTORY}

    curl -Lo ./kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS_NAME/mac/darwin}-${MACHINE_ARCH/x86_64/amd64}"

    if [ "$?" -ne "0" ]; then
      echo "Unable to download kind."
      exit 1
    fi

    install -m 0755 kind "${KIND_INSTALL_DIRECTORY}"/kind

    if [ "$?" -ne "0" ]; then
      echo "Insufficient permissions to install kind. Trying again with sudo..."
      sudo install -m 0755 kind "${KIND_INSTALL_DIRECTORY}"/kind
      if [ "$?" -ne "0" ]; then
        echo "Unable to install kind. Check installation path and permissions."
        exit 1
      fi
    fi

    KIND_PATH=${KIND_INSTALL_DIRECTORY}/kind
  fi
}

preinstall_checks() {
  local answer=
  local cluster_exist=true

  read -rp "Specify kind cluster name [$KIND_CLUSTER_NAME]: " answer
  if [[ -n "$answer" ]]; then
    KIND_CLUSTER_NAME=$answer
  fi

  while [[ "$cluster_exist" == "true" ]]; do

    # Check if a kind cluster with the name `d8` exist
    ${KIND_PATH} get clusters | grep -q "^${KIND_CLUSTER_NAME}$" &>/dev/null

    if [ "$?" -eq "0" ]; then
      cluster_exist=true
    else
      cluster_exist=false
    fi

    while [[ ("$answer" != "y") && ("$cluster_exist" == "true") ]]; do
      read -rp "A kind cluster named '$KIND_CLUSTER_NAME' already exists. Delete it? y/[n]: " answer
      if [[ ("$answer" == "n") || (-z "$answer") ]]; then
        echo -e "Please delete the cluster '${KIND_CLUSTER_NAME}' manually. You can use the following command:\n\t${KIND_PATH} delete cluster --name ${KIND_CLUSTER_NAME}."
        exit 1
      fi

      if [[ "$answer" != "y" ]]; then
        echo "Please type 'y' to delete cluster '${KIND_CLUSTER_NAME}' or 'n' to abort the installation."
      fi
    done

    if [[ "$cluster_exist" == "true" ]]; then
      ${KIND_PATH} delete cluster --name "${KIND_CLUSTER_NAME}"
      sleep 3
    fi
  done
}
configs_create() {
  mkdir -p ${CONFIG_DIR}

  echo "Creating kind config file (${CONFIG_DIR}/kind.cfg)..."
  cat <<EOF >${CONFIG_DIR}/kind.cfg
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    listenAddress: "127.0.0.1"
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    listenAddress: "127.0.0.1"
    protocol: TCP
EOF

  echo "Creating Deckhouse Kubernetes Platform installation config file (${CONFIG_DIR}/config.yml)..."
  cat <<EOF >${CONFIG_DIR}/config.yml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Minimal
    releaseChannel: EarlyAccess
    logLevel: Info
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: "%s.127.0.0.1.sslip.io"
      https:
          mode: Disabled
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  settings:
    longtermRetentionDays: 0
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ingress-nginx
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-prometheus-crd
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: operator-prometheus
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus-crd
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-kubernetes
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-deckhouse
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-kubernetes-control-plane
spec:
  enabled: true
EOF

  if [[ -n "$D8_LICENSE_KEY" ]]; then
    generate_ee_access_string "$D8_LICENSE_KEY"
    cat <<EOF >>${CONFIG_DIR}/config.yml
  imagesRepo: $D8_REGISTRY_PATH
  registryDockerCfg: $D8_EE_ACCESS_STRING
EOF
  fi

  echo "Creating Deckhouse Kubernetes Platform resource file (${CONFIG_DIR}/resources.yml)..."
  cat <<EOF >${CONFIG_DIR}/resources.yml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
EOF
}

cluster_deletion_info() {

    printf "
To delete created cluster use the following command:

    ${KIND_PATH} delete cluster --name "${KIND_CLUSTER_NAME}"

"
}

cluster_create() {

  ${KIND_PATH} create cluster --name "${KIND_CLUSTER_NAME}" --image "${KIND_IMAGE}" --config "${CONFIG_DIR}/kind.cfg"

  if [ "$?" -ne "0" ]; then
    printf "
Error creating cluster. If error is like '...port is already allocated' or '... address already in use', then you need to free ports 80 and 443.
E.g., you can find programs that use these ports using the following command:

    sudo lsof -n -i TCP@0.0.0.0:80,443 -s TCP:LISTEN

"
    cluster_deletion_info
    exit 1
  fi

  ${KIND_PATH} get kubeconfig --internal --name "${KIND_CLUSTER_NAME}" >${CONFIG_DIR}/kubeconfig

}

deckhouse_install() {
  echo "Running Deckhouse Kubernetes Platform installation (the $D8_RELEASE_CHANNEL_NAME release channel)..."

  docker run --pull=always --rm --network kind -v "${CONFIG_DIR}/config.yml:/config.yml" -v "${CONFIG_DIR}/resources.yml:/resources.yml" \
    -v "${CONFIG_DIR}/kubeconfig:/kubeconfig" ${D8_REGISTRY_PATH}/install:$D8_RELEASE_CHANNEL_TAG \
    bash -c "dhctl bootstrap-phase install-deckhouse --kubeconfig=/kubeconfig --kubeconfig-context=kind-${KIND_CLUSTER_NAME} --config=/config.yml && \
             dhctl bootstrap-phase create-resources --kubeconfig=/kubeconfig --kubeconfig-context=kind-${KIND_CLUSTER_NAME} --resources=/resources.yml"

  if [ "$?" -ne "0" ]; then
    echo "Error installing Deckhouse Kubernetes Platform!"
    cluster_deletion_info
    exit 1
  fi
}

ingress_check() {
  local retries_max=100
  local retries_count=0

  retries_count=0
  echo -n "Waiting for the Ingress controller to be ready..."

  while true; do
    ingress_ready_pods=$(${KUBECTL_PATH} --context kind-"${KIND_CLUSTER_NAME}" -n d8-ingress-nginx get ads/controller-nginx -o jsonpath="{$.status.numberReady}" 2>/dev/null)
    if [ "$?" -ne "0" ]; then
      echo -n "."
      ((retries_count++))
      sleep 10
    else
      # Check Ingress readiness
      if [ "$ingress_ready_pods" = "1" ]; then
        break
      else
        echo -n "."
        ((retries_count++))
        sleep 10
      fi
    fi

    if [ "$retries_count" -ge "$retries_max" ]; then
      echo
      echo "Timeout waiting for the creation of Ingress controller!"
      echo
      echo "Here is the output of the 'kubectl -n d8-ingress-nginx get ads/controller-nginx -o wide' command:"
      ${KUBECTL_PATH} --context "kind-${KIND_CLUSTER_NAME}" -n d8-ingress-nginx get ads/controller-nginx -o wide
      echo
      echo "Here is the output of the 'kubectl -n d8-ingress-nginx get all' command:"
      ${KUBECTL_PATH} --context "kind-${KIND_CLUSTER_NAME}" -n d8-ingress-nginx get all
      echo
      echo "If the controller-nginx Pod is in the ContainerCreating status, you most likely have a slow connection. If so, wait a little longer until the controller-nginx Pod becomes Ready. After that, run the following command to get the admin password for Grafana: '${KUBECTL_PATH} --context kind-${KIND_CLUSTER_NAME} -n d8-system exec deploy/deckhouse -- sh -c \"deckhouse-controller module values prometheus -o json | jq -r '.internal.auth.password'\""
      echo
      cluster_deletion_info
      exit 1
    fi
  done

  echo
  echo "Ingress controller is running."
}

generate_ee_access_string() {
  auth_part=$(echo -n "license-token:$1" | base64 -w0)
  D8_EE_ACCESS_STRING=$(echo -n "{\"auths\": { \"$D8_REGISTRY_ADDRESS\": { \"username\": \"license-token\", \"password\": \"$1\", \"auth\": \"$auth_part\"}}}" | base64 -w0)

  if [ "$?" -ne "0" ]; then
    echo "Error generation container registry access string for Deckhouse Kubernetes Platform Enterprise Edition"
    exit 1
  fi
}

install_show_credentials() {

  local prometheus_password
  prometheus_password=$(${KUBECTL_PATH} --context "kind-${KIND_CLUSTER_NAME}" -n d8-system exec deploy/deckhouse -c deckhouse -- sh -c "deckhouse-controller module values prometheus -o json | jq -r '.internal.auth.password'")
  if [ "$?" -ne "0" ] || [ -z "$prometheus_password" ]; then
    printf "
Error getting Prometheus password.

Try to run the following command to get Prometheus password:

    %s --context %s -n d8-system exec deploy/deckhouse -c deckhouse -- sh -c "deckhouse-controller module values prometheus -o json | jq -r '.internal.auth.password'"

" "${KUBECTL_PATH}" "kind-${KIND_CLUSTER_NAME}"
  else
    printf "
Provide following credentials to access Grafana at http://grafana.127.0.0.1.sslip.io/ :

    Username: admin
    Password: %s

" "${prometheus_password}"
  fi
}

installation_finish() {
  printf "
You have installed Deckhouse Kubernetes Platform in kind!

Don't forget that the default kubectl context has been changed to 'kind-${KIND_CLUSTER_NAME}'.

Run '%s --context %s cluster-info' to see cluster info.
Run '%s delete cluster --name %s' to delete the cluster.
" "${KUBECTL_PATH}" "kind-${KIND_CLUSTER_NAME}" "${KIND_PATH}" "${KIND_CLUSTER_NAME}" >${CONFIG_DIR}/info.txt

  install_show_credentials >>${CONFIG_DIR}/info.txt

  cat ${CONFIG_DIR}/info.txt
  printf "The information above is saved to %s file.

Good luck!
" "${CONFIG_DIR}/info.txt"
}

main() {

  parse_args "$@"

  os_detect
  prerequisites_check
  configs_create
  cluster_create
  deckhouse_install
  ingress_check
  installation_finish
}

main "$@"
