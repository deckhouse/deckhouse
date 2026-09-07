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

# Colors to identify the chip
BOLD='\033[1m'
GREEN='\033[0;32m'
PURPLE='\033[0;35m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

#  Checking OS and getting a chip name
if uname -s | grep -q "Darwin"; then
    chip_info=$(sysctl -n machdep.cpu.brand_string)
    if [[ "$chip_info" == *"Apple M"* ]]; then
    # Retrieving the processor generation for Apple on the M
    chip_model=$(echo "$chip_info" | awk -F'Apple ' '{print $2}' | cut -d' ' -f1-2 | sed 's/ / /')
    # Display an alert for Apple on M
    echo -e "${BOLD}${PURPLE}Warning. ${CYAN}Your computer has been identified as: ${GREEN}Apple $chip_model ${NC}
    ${YELLOW}Keep Rosetta emulation enabled in your container runtime.
    Deckhouse is published for amd64 only, so on Apple Silicon it always runs emulated. What rules
    QEMU out is throughput, not a missing instruction: the shell-operator task queues report a
    histogram for every action they measure, and the metrics vault serializes all of those on a
    single mutex which it holds while registering the collector and reading its label names
    (pkg/metrics-storage/storage/grouped.go, RegisterHistogram). Slowed down by QEMU, that one lock
    becomes the bottleneck - goroutine dumps show the queue workers parked in RegisterHistogram and
    the module installation never completes. Under Rosetta the same contention is visible in the
    dumps, but the queues keep draining and the cluster comes up.
    The known price of Rosetta is one container: the kubelet-eviction-thresholds-exporter of
    node-exporter, which Rosetta crashes in Pods that share the host PID namespace. On arm64 hosts
    this script removes just that container (see --arch-workaround), so only the kubelet eviction
    threshold metrics are missing.

    In Docker Desktop keep ${CYAN}Settings > General > Virtual Machine Options > Use Rosetta for x86_64/amd64 emulation on Apple Silicon ${YELLOW}checked.
    In OrbStack: ${CYAN}orbctl config set rosetta true${YELLOW}, then restart it with ${CYAN}orbctl stop${YELLOW}.${NC}"
    fi
fi

CONFIG_DIR=~/.kind-d8
KIND_IMAGE_NAME=kindest/node
KIND_IMAGE_TAG=v1.34.8
D8_RELEASE_CHANNEL_TAG=stable
D8_RELEASE_CHANNEL_NAME=Stable
D8_REGISTRY_ADDRESS=registry.deckhouse.io
D8_REGISTRY_PATH=
D8_LICENSE_KEY=
D8_DEV_BRANCH=
D8_CHANNEL_EXPLICIT=false
D8_RESOURCES_TIMEOUT=30m
# Deckhouse charts only publish a module's Ingress when https is not Disabled, so the web UIs
# (e.g. the console module) are unreachable by URL until this is changed. See --https-mode.
D8_HTTPS_MODE=Disabled
# Address and host ports the cluster's HTTP and HTTPS are published on.
# See --listen-address, --listen-http-port and --listen-https-port.
KIND_LISTEN_ADDRESS=127.0.0.1
KIND_HTTP_PORT=80
KIND_HTTPS_PORT=443
# Root domain the module web UIs are published under, as <module>.<domain>. See --public-domain.
D8_PUBLIC_DOMAIN=127.0.0.1.sslip.io
# Dex user created for the web UIs (console, Grafana, ...) when https is not Disabled. The password
# is taken from --admin-password or generated, and printed at the end of the installation.
D8_ADMIN_EMAIL=admin@deckhouse.io
D8_ADMIN_PASSWORD=
# Whether the Prometheus stack is installed. See --with-metrics.
D8_WITH_METRICS=false
# Whether the registry-packages-proxy module is installed. See --with-registry-packages-proxy.
D8_WITH_REGISTRY_PACKAGES_PROXY=false
# Whether the documentation module is installed. See --with-documentation.
D8_WITH_DOCUMENTATION=false
# Comma-separated MODULE=VERSION list that pins source-based modules to an exact version through
# ModulePullOverrides, applied once Deckhouse is up. See --module-version-override.
D8_MODULE_VERSIONS=
# deckhouse.allowExperimentalModules and deckhouse.allowedExperimentalModules. See --allow-experimental-modules.
D8_ALLOW_EXPERIMENTAL=false
D8_EXPERIMENTAL_MODULES=
DRY_RUN=false

# Deckhouse is published for linux/amd64 only. On an arm64 host its images still run inside the
# arm64 kind node through the container runtime's emulation, but two things have to be papered over,
# which arch_workaround_apply() does at admission time so that no chart has to be touched:
# an arch nodeAffinity that would keep some Pods Pending, and libevent's use of epoll_wait(2).
#
# Making the kind node itself amd64 is not an alternative: under emulation seccomp is unavailable, so
# kubelet cannot create any Pod sandbox at all ("failed to generate seccomp spec opts: seccomp is not
# supported") - every static Pod of the control plane fails and kubeadm never finishes.
#
# Auto-enabled on arm64 hosts, see --arch-workaround.
ARCH_WORKAROUND=

KIND_INSTALL_DIRECTORY=$CONFIG_DIR
KIND_PATH=kind
KIND_CLUSTER_NAME=d8
KIND_VERSION=v0.32.0

KUBECTL_INSTALL_DIRECTORY=$CONFIG_DIR
KUBECTL_PATH=kubectl
KUBECTL_VERSION=v1.36.3

REQUIRE_MEMORY_MIN_BYTES=4000000000 # 4GB

# In --dry-run mode, prints the command that would be executed instead of running it.
run_cmd() {
  if [[ "$DRY_RUN" == "true" ]]; then
    printf '+ %s\n' "$*"
    return 0
  fi
  "$@"
}

# In --dry-run mode, prints a generated config file's content, delimited so it's easy to spot and copy.
print_dry_run_file() {
  if [[ "$DRY_RUN" == "true" ]]; then
    echo "----- BEGIN $1 -----"
    cat "$1"
    echo "----- END $1 -----"
    echo
  fi
}

usage() {
  printf "
 Usage: %s [--channel <CHANNEL NAME>] [--key <DECKHOUSE EE LICENSE KEY>] [--registry-address <REGISTRY ADDRESS>] [--registry-path <REGISTRY PATH>] [--dev-branch <TAG>] [--resources-timeout <DURATION>] [--https-mode <Disabled|OnlyInURI|SelfSigned>] [--public-domain <DOMAIN>] [--listen-address <ADDRESS>] [--listen-http-port <PORT>] [--listen-https-port <PORT>] [--admin-password <PASSWORD>] [--with-metrics] [--with-registry-packages-proxy] [--with-documentation] [--module-version-override <MODULE=VERSION[,...]>] [--allow-experimental-modules [MODULE[,...]]] [--arch-workaround <auto|on|off>] [--kind-image-tag <TAG>] [--dry-run] [--os <linux|mac>]

    --channel <CHANNEL NAME>
            Deckhouse Kubernetes Platform release channel name.
            Possible values: Alpha, Beta, EarlyAccess, Stable, RockSolid.
            Default: Stable.

    --key <DECKHOUSE EE LICENSE KEY>
            Deckhouse Kubernetes Platform Enterprise Edition license key.
            If no license key specified, Deckhouse Kubernetes Platform Community Edition will be installed.
            The key is also used to log the host docker in to --registry-address, which is needed
            because the installer image and the Deckhouse image are pulled by the host itself.

    --registry-address <REGISTRY ADDRESS>
            Container registry address to install Deckhouse Kubernetes Platform from.
            Default: registry.deckhouse.io.

    --registry-path <REGISTRY PATH>
            Path to the Deckhouse Kubernetes Platform images in the container registry.
            Can be given either as a full path including the registry address (e.g. my-registry.example/foo/bar)
            or as a short path starting with / (e.g. /foo/bar), which is then prefixed with --registry-address.
            If not specified, it is derived from --registry-address (and edition, based on whether --key is set).

    --dev-branch <TAG>
            Sets the devBranch field of the Deckhouse InitConfiguration, to deploy a specific build tag of the
            Deckhouse controller image (e.g. a git branch name or a custom CI build tag) instead of the tag
            resolved from --channel.
            If --channel is not also explicitly specified, the installer image is pulled using this same tag
            (for registries that publish builds tagged by branch/PR rather than by release channel).

    --resources-timeout <DURATION>
            Timeout for the dhctl create-resources phase to wait for resources to become ready,
            in Go duration format (e.g. 30m, 45m, 1h).
            Default: 30m.

    --https-mode <Disabled|OnlyInURI|SelfSigned>
            Value of the global modules.https setting, which decides whether module web UIs get an
            Ingress at all - Deckhouse charts skip it when https is Disabled. It therefore also
            decides which modules are worth installing at all.
              Disabled   - no Ingress and no web UIs. The console, user-authn and control-plane-manager
                           modules are left out: without an Ingress there is nothing to open,
                           user-authn switches itself off anyway, and control-plane-manager exists
                           here only to make kube-apiserver trust Dex - which would cost several
                           minutes of rerendering the control plane for nothing. The fastest cluster
                           to bring up, and the one to pick when only the Kubernetes API is needed.
              OnlyInURI  - Ingress without TLS (still plain HTTP on port 80), but Deckhouse builds
                           https:// links, assuming TLS is terminated in front of the cluster.
              SelfSigned - https.mode=CertManager with the selfsigned ClusterIssuer of the
                           cert-manager module: cert-manager issues a certificate per module domain
                           under the root domain from --public-domain, so the UIs open over https with the usual
                           browser warning about an untrusted certificate.
            Default: Disabled.

    --public-domain <DOMAIN>
            Root domain the module web UIs are published under. Deckhouse builds their names from it
            as <module>.<DOMAIN> (the global modules.publicDomainTemplate setting), so the console
            ends up at console.<DOMAIN> and Grafana at grafana.<DOMAIN>.
            The default needs nothing set up: sslip.io resolves 127.0.0.1.sslip.io and everything
            under it to the loopback address. A custom domain has to resolve to 127.0.0.1 on its own
            - through /etc/hosts, a wildcard DNS record or a similar service - otherwise the UIs stay
            unreachable even though the cluster itself is healthy.
            Default: 127.0.0.1.sslip.io.

    --listen-address <ADDRESS>
            Address the cluster's HTTP and HTTPS ports are published on by kind (the listenAddress of
            its extraPortMappings).
            The default keeps them on the loopback interface, so the web UIs are reachable from this
            machine only. Use 0.0.0.0 to publish them on every interface - which is what a
            --public-domain pointing at this machine's address needs, and which also exposes the
            cluster to anyone who can reach that address.
            Default: 127.0.0.1.

    --listen-http-port <PORT>
    --listen-https-port <PORT>
            Host ports the cluster's HTTP and HTTPS are published on, for when 80 and 443 are already
            taken on this machine. Inside the cluster the Ingress controller keeps listening on 80 and
            443 - only the host side of the mapping changes.
            Mind what this cannot do: Deckhouse builds every address of its own without a port.
            publicDomainTemplate, the Ingress hosts and the redirect URIs of Dex are plain DNS names,
            and none of them accepts one. So with a non-standard HTTPS port the web UIs answer at
            https://<module>.<domain>:<port>/, but signing in redirects to the standard port and
            fails - which leaves them unusable, even though the cluster itself is fully functional.
            When the standard ports are taken, reach the UIs one of these ways instead:
              - run a proxy in front that listens on 443 and forwards to this port - the setup
                IngressNginxController's behindL7Proxy option exists for;
              - keep 443 and publish it on a different address with --listen-address;
              - forward the port from the machine you browse from, e.g.
                'ssh -L 443:127.0.0.1:<port> <host>' combined with --public-domain 127.0.0.1.sslip.io.
            Defaults: 80 and 443.

    --admin-password <PASSWORD>
            Password of the admin@deckhouse.io user created for the web UIs, in plain text - the
            script hashes it with bcrypt before writing it into the User resource, since that is the
            form the resource expects.
            If not specified, a random password is generated. Either way it is printed at the end of
            the installation.
            Ignored with --https-mode Disabled, where no user is created at all.

    --with-metrics
            Also install the Prometheus stack: the operator-prometheus, prometheus,
            monitoring-kubernetes and prometheus-metrics-adapter modules, and with them Grafana.
            Left out by default because it is what makes a kind stand heavy - the stack alone accounts
            for a good half of the Pods and a large part of the CPU and memory the cluster uses, which
            is felt most under emulation on Apple Silicon.
            Without it the cluster has no Grafana and no metrics-based autoscaling; everything else,
            including the console, works as usual.
            Note that with --https-mode Disabled the stack is installed but Grafana gets no Ingress,
            so it stays reachable only through kubectl port-forward.

    --with-registry-packages-proxy
            Also install the registry-packages-proxy module, which serves the packages nodes install
            from the registry through the cluster. A kind node installs nothing that way, so it is
            left out by default.

    --with-documentation
            Also install the documentation module, which serves the platform documentation for the
            installed version at documentation.<domain>.

    --module-version-override <MODULE=VERSION[,MODULE=VERSION...]>
            Pin modules delivered through a ModuleSource to an exact version, as a comma-separated
            list of pairs (MODULE=VERSION, or MODULE:VERSION), e.g. console=v1.60.1,stronghold=v1.4.0.
            Each pair becomes a ModulePullOverride, applied after Deckhouse is installed; the script
            then waits for the module to report the requested version.
            Useful because release channel tags in a development registry are not curated - any
            branch's CI can move them, so a module can silently go backwards (the console module went
            from v1.60.x to v1.45.11 that way, on every channel at once).
            While an override exists the module no longer follows its release channel, which is what
            makes the version stick. Remove it to let the module follow the channel again:
            kubectl delete modulepulloverride <MODULE>.

    --allow-experimental-modules [MODULE[,MODULE...]]
            Allow modules in the Experimental lifecycle stage to be enabled, which Deckhouse refuses
            by default.
            Without a value, sets deckhouse.allowExperimentalModules to true, allowing all of them.
            With a comma-separated list of module names, sets deckhouse.allowedExperimentalModules
            instead, allowing only those - the more conservative form of the same opt-in.
            Experimental modules can be unstable and are not covered by the Deckhouse SLA.

    --arch-workaround <auto|on|off>
            Make the amd64-only parts of Deckhouse work on a kind node of another architecture, using
            MutatingAdmissionPolicies (so the charts themselves are left alone):
              - drops a required nodeAffinity on kubernetes.io/arch=amd64, which otherwise leaves
                Pods Pending forever - the kruise controller and the validator of the ingress-nginx
                module are pinned that way, while their images do run under emulation;
              - sets EVENT_NOEPOLL=1, so libevent-based programs avoid epoll_wait(2), which an
                arm64 kernel does not implement (it breaks the memcached container of the
                prometheus module);
              - raises a CPU limit already set below 1 core, and a memory limit already set below 256Mi,
                execution starves an emulated process (the config-reloader containers of the
                prometheus module ship a 10m limit and never finish starting up);
              - drops the kubelet-eviction-thresholds-exporter container from Pods that share the
                host PID namespace, where Rosetta's path resolution crashes it in a loop; the
                kubelet eviction threshold metrics are lost, the rest of node-exporter keeps working.
            'auto' enables it on arm64 hosts only. Default: auto.

    --kind-image-tag <TAG>
            Tag (version) of the kindest/node image to use for the cluster (e.g. v1.34.8).
            Must be a version supported by the installed kind CLI (see: kind version) - a node image
            newer than what kind was built for can make cluster creation hang or fail unpredictably.
            Default: v1.34.8.

    --dry-run
            Print the commands that would be executed (docker, kind, kubectl, curl, install) instead of
            running them. The local files are still generated normally, so their content can be
            inspected: kind.cfg, core-module-configs.yml, config.yml, resources.yml, and arch-policy.yml
            when the arch workaround applies. No cluster is created or modified.

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
          D8_CHANNEL_EXPLICIT=true
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
        shift
        ;;
      esac
      ;;
    --registry-address)
      case "$2" in
      "")
        echo "Registry address is empty. Please specify the registry address."
        usage
        exit 1
        ;;
      *)
        D8_REGISTRY_ADDRESS="$2"
        shift
        ;;
      esac
      ;;
    --registry-path)
      case "$2" in
      "")
        echo "Registry path is empty. Please specify the registry path."
        usage
        exit 1
        ;;
      *)
        D8_REGISTRY_PATH="$2"
        shift
        ;;
      esac
      ;;
    --dev-branch)
      case "$2" in
      "")
        echo "Dev branch/tag is empty. Please specify the value for devBranch."
        usage
        exit 1
        ;;
      *)
        D8_DEV_BRANCH="$2"
        shift
        ;;
      esac
      ;;
    --resources-timeout)
      case "$2" in
      "")
        echo "Resources timeout is empty. Please specify a Go duration (e.g. 30m, 1h)."
        usage
        exit 1
        ;;
      *)
        D8_RESOURCES_TIMEOUT="$2"
        shift
        ;;
      esac
      ;;
    --public-domain)
      case "$2" in
      "" | --*)
        echo "Domain is empty. Please specify the root domain for the web UIs, e.g. 127.0.0.1.sslip.io."
        usage
        exit 1
        ;;
      *)
        if [[ ! "$2" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]]; then
          echo "Incorrect --public-domain value '$2'. Use a DNS name, e.g. 127.0.0.1.sslip.io."
          usage
          exit 1
        fi
        D8_PUBLIC_DOMAIN="$2"
        shift
        ;;
      esac
      ;;
    --listen-address)
      case "$2" in
      "" | --*)
        echo "Listen address is empty. Please specify an IPv4 address, e.g. 127.0.0.1 or 0.0.0.0."
        usage
        exit 1
        ;;
      *)
        if [[ ! "$2" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]; then
          echo "Incorrect --listen-address value '$2'. Use an IPv4 address, e.g. 127.0.0.1 or 0.0.0.0."
          usage
          exit 1
        fi
        KIND_LISTEN_ADDRESS="$2"
        shift
        ;;
      esac
      ;;
    --listen-http-port | --listen-https-port)
      case "$2" in
      "" | --*)
        echo "Port is empty. Please specify a TCP port for ${1}, e.g. 8080."
        usage
        exit 1
        ;;
      *)
        if [[ ! "$2" =~ ^[0-9]+$ ]] || [[ "$2" -lt 1 ]] || [[ "$2" -gt 65535 ]]; then
          echo "Incorrect ${1} value '$2'. Use a TCP port number between 1 and 65535."
          usage
          exit 1
        fi
        if [[ "$1" == "--listen-http-port" ]]; then
          KIND_HTTP_PORT="$2"
        else
          KIND_HTTPS_PORT="$2"
        fi
        shift
        ;;
      esac
      ;;
    --admin-password)
      case "$2" in
      "")
        echo "Admin password is empty. Please specify the password or don't use the --admin-password parameter to have one generated."
        usage
        exit 1
        ;;
      *)
        D8_ADMIN_PASSWORD="$2"
        shift
        ;;
      esac
      ;;
    --https-mode)
      case "$2" in
      Disabled | OnlyInURI | SelfSigned)
        D8_HTTPS_MODE="$2"
        shift
        ;;
      *)
        echo "Incorrect --https-mode value. Use Disabled, OnlyInURI or SelfSigned."
        usage
        exit 1
        ;;
      esac
      ;;
    --arch-workaround)
      case "$2" in
      auto)
        ARCH_WORKAROUND=
        ;;
      on | true)
        ARCH_WORKAROUND=true
        ;;
      off | false)
        ARCH_WORKAROUND=false
        ;;
      *)
        echo "Incorrect --arch-workaround value. Use auto, on or off."
        usage
        exit 1
        ;;
      esac
      shift
      ;;
    --kind-image-tag)
      case "$2" in
      "")
        echo "kind image tag is empty. Please specify a kindest/node tag (e.g. v1.31.6)."
        usage
        exit 1
        ;;
      *)
        KIND_IMAGE_TAG="$2"
        shift
        ;;
      esac
      ;;
    --module-version-override)
      case "$2" in
      "" | --*)
        echo "Module version list is empty. Please specify MODULE=VERSION pairs, e.g. console=v1.60.1,stronghold=v1.4.0."
        usage
        exit 1
        ;;
      *)
        D8_MODULE_VERSIONS="$2"
        shift
        ;;
      esac
      ;;
    --with-metrics)
      D8_WITH_METRICS=true
      ;;
    --with-registry-packages-proxy)
      D8_WITH_REGISTRY_PACKAGES_PROXY=true
      ;;
    --with-documentation)
      D8_WITH_DOCUMENTATION=true
      ;;
    --allow-experimental-modules)
      # Optional value: a list of module names narrows the opt-in to those modules, no value allows
      # every Experimental module.
      case "$2" in
      "" | --*)
        D8_ALLOW_EXPERIMENTAL=true
        ;;
      *)
        D8_EXPERIMENTAL_MODULES="$2"
        shift
        ;;
      esac
      ;;
    --dry-run)
      DRY_RUN=true
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

registry_path_default() {
  if [[ -z "$D8_REGISTRY_PATH" ]]; then
    if [[ -n "$D8_LICENSE_KEY" ]]; then
      D8_REGISTRY_PATH=${D8_REGISTRY_ADDRESS}/deckhouse/ee
    else
      D8_REGISTRY_PATH=${D8_REGISTRY_ADDRESS}/deckhouse/ce
    fi
  elif [[ "$D8_REGISTRY_PATH" == /* ]]; then
    # Short form: just the path part, e.g. "/sys/deckhouse-oss" -> "<registry-address>/sys/deckhouse-oss".
    D8_REGISTRY_PATH="${D8_REGISTRY_ADDRESS}${D8_REGISTRY_PATH}"
  fi

  # If a dev branch/tag is requested and the release channel wasn't explicitly
  # set, assume the installer image is published under the same tag (this is
  # the case for registries that only tag builds by branch/PR, not by release
  # channel). Passing --channel explicitly keeps using an official installer
  # image alongside a custom devBranch.
  if [[ -n "$D8_DEV_BRANCH" && "$D8_CHANNEL_EXPLICIT" == "false" ]]; then
    D8_RELEASE_CHANNEL_TAG="$D8_DEV_BRANCH"
  fi
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
  if [[ ("$OS_NAME" == "Ubuntu") || ("$OS_NAME" == "ubuntu") || ("$OS_NAME" == "Debian") || ("$OS_NAME" == "debian") ]]; then
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

  # 'auto': the workaround is only needed when the kind node won't be amd64.
  if [[ -z "$ARCH_WORKAROUND" ]]; then
    case "$MACHINE_ARCH" in
    arm64 | aarch64)
      ARCH_WORKAROUND=true
      ;;
    *)
      ARCH_WORKAROUND=false
      ;;
    esac
  fi

  if [[ "$ARCH_WORKAROUND" == "true" ]]; then
    echo "Pods pinned to amd64 nodes will be allowed to run on this ${MACHINE_ARCH} node (--arch-workaround)."
  fi
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
  registry_login
  registry_images_check
  preinstall_checks
}

# Logs the host docker in to the registry. Two steps pull images with the host docker itself - the
# installer image in deckhouse_install() and the Deckhouse image api_proxy_install() reads the
# api-proxy digest from - and neither is covered by registryDockerCfg, which only reaches the
# cluster. Without a login they fail with "unauthorized" in the middle of the installation.
registry_login() {
  if [[ -z "$D8_LICENSE_KEY" ]]; then
    return 0
  fi

  echo "Logging in to ${D8_REGISTRY_ADDRESS} as license-token..."

  if [[ "$DRY_RUN" == "true" ]]; then
    printf '+ %s\n' "docker login ${D8_REGISTRY_ADDRESS} --username license-token --password-stdin"
    return 0
  fi

  if ! printf '%s' "$D8_LICENSE_KEY" | docker login "${D8_REGISTRY_ADDRESS}" --username license-token --password-stdin >/dev/null; then
    echo "Error logging in to ${D8_REGISTRY_ADDRESS} with the given license key!"
    exit 1
  fi
}

# Checks up front that both images this installation needs actually exist, because a missing tag used
# to surface only after the cluster had been created: as a digest error in api_proxy_install(), and
# it would have failed again in the cluster anyway - dhctl points Deckhouse at imagesRepo:tag, the
# same reference (see GetInclusterImage in dhctl/pkg/config/deckhouse_config.go).
registry_images_check() {
  local image=
  local err=

  echo "Checking that the Deckhouse Kubernetes Platform images exist..."

  for image in "${D8_REGISTRY_PATH}/install:${D8_RELEASE_CHANNEL_TAG}" "${D8_REGISTRY_PATH}:${D8_RELEASE_CHANNEL_TAG}"; do
    if [[ "$DRY_RUN" == "true" ]]; then
      printf '+ %s\n' "docker manifest inspect ${image}"
      continue
    fi

    if err=$(docker manifest inspect "$image" 2>&1 >/dev/null); then
      continue
    fi

    echo "Error: the image ${image} is not available."
    echo "${err}" | tail -1

    case "$err" in
    *unauthorized* | *denied* | *authentication*)
      echo "The registry denied access. Pass the license key with --key, or log in beforehand with 'docker login ${D8_REGISTRY_ADDRESS}'."
      ;;
    *)
      if [[ -n "$D8_DEV_BRANCH" ]]; then
        echo "The tag comes from --dev-branch (${D8_DEV_BRANCH}). Such a build can expire or may not be published yet -"
        echo "note that the installer image and the Deckhouse image are published separately, so one can exist without the other."
      fi
      ;;
    esac

    exit 1
  done
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
    if [[ "$DRY_RUN" != "true" ]]; then
      echo "Press enter to continue..."
      read
    fi
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

    if [[ "$DRY_RUN" == "true" ]]; then
      echo "+ (would prompt to install kubectl into ${KUBECTL_INSTALL_DIRECTORY})"
      run_cmd mkdir -p $KUBECTL_INSTALL_DIRECTORY
      run_cmd curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${OS_NAME/mac/darwin}/${MACHINE_ARCH/x86_64/amd64}/kubectl"
      run_cmd install -m 0755 kubectl "${KUBECTL_INSTALL_DIRECTORY}"/kubectl
      KUBECTL_PATH=${KUBECTL_INSTALL_DIRECTORY}/kubectl
      return
    fi

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

    if [[ "$DRY_RUN" == "true" ]]; then
      echo "+ (would prompt to install kind into ${KIND_INSTALL_DIRECTORY})"
      run_cmd mkdir -p ${KIND_INSTALL_DIRECTORY}
      run_cmd curl -Lo ./kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS_NAME/mac/darwin}-${MACHINE_ARCH/x86_64/amd64}"
      run_cmd install -m 0755 kind "${KIND_INSTALL_DIRECTORY}"/kind
      KIND_PATH=${KIND_INSTALL_DIRECTORY}/kind
      return
    fi

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

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "+ (would prompt for kind cluster name, default: ${KIND_CLUSTER_NAME})"
    if command -v ${KIND_PATH} >/dev/null 2>&1; then
      if ${KIND_PATH} get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
        run_cmd ${KIND_PATH} delete cluster --name "${KIND_CLUSTER_NAME}"
      fi
    else
      echo "+ (kind is not available yet to check for an existing cluster named '${KIND_CLUSTER_NAME}')"
    fi
    return
  fi

  read -rp "Specify kind cluster name [$KIND_CLUSTER_NAME] (the cluster name must match the regular expression \`^[a-z0-9.-]+$\`, underscore are not supported): " answer
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

# Prints the local addresses already listening on TCP port $1, one per line ("0.0.0.0", "127.0.0.1",
# "[::]", ...). Uses whichever tool the system has - macOS ships lsof but no ss, most Linux
# distributions the other way round, and netstat is the fallback present on both - and prints nothing
# when none of them is available.
port_listeners() {
  local port="$1"

  if command -v lsof >/dev/null 2>&1; then
    # -F n prints one "n<address>:<port>" line per listening socket
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN -F n 2>/dev/null | sed -n 's/^n//p' | sed 's/:[0-9]*$//'
    return 0
  fi

  if command -v ss >/dev/null 2>&1; then
    ss -tlnH "sport = :${port}" 2>/dev/null | awk '{print $4}' | sed 's/:[0-9]*$//'
    return 0
  fi

  if command -v netstat >/dev/null 2>&1; then
    # The two netstat flavours differ in how they join the address and the port: BSD/macOS writes
    # "127.0.0.1.80" and "*.80", Linux writes "127.0.0.1:80" and ":::80". Splitting on the last
    # separator, whichever it is, handles both - and IPv6 along with them.
    netstat -an 2>/dev/null | awk -v want="${port}" '$NF == "LISTEN" {
        addr = $4
        if (match(addr, /[.:][0-9]+$/) && substr(addr, RSTART + 1) == want) {
          print substr(addr, 1, RSTART - 1)
        }
      }'
    return 0
  fi

  return 0
}

# Refuses to build a cluster whose ports are already taken, instead of letting kind fail halfway
# through with "port is already allocated" and leaving a half-created cluster behind.
#
# Whether a port collides depends on the address it is published on, so both are checked together:
# publishing on 0.0.0.0 collides with any listener on that port, while a listener bound to 0.0.0.0
# (or ::) collides with every address. Two listeners on different specific addresses do not collide.
#
# This runs after prerequisites_check, so an existing cluster of the same name is already gone by now
# and does not report its own ports as taken.
ports_check() {
  local port=
  local listener=
  local busy=false

  echo "Checking that ${KIND_LISTEN_ADDRESS}:${KIND_HTTP_PORT} and ${KIND_LISTEN_ADDRESS}:${KIND_HTTPS_PORT} are free..."

  for port in "$KIND_HTTP_PORT" "$KIND_HTTPS_PORT"; do
    for listener in $(port_listeners "$port"); do
      case "$listener" in
      "$KIND_LISTEN_ADDRESS" | '0.0.0.0' | '*' | '::' | '[::]')
        echo "Port ${port} is already in use on ${listener}."
        busy=true
        ;;
      *)
        if [[ "$KIND_LISTEN_ADDRESS" == "0.0.0.0" ]]; then
          echo "Port ${port} is already in use on ${listener}, and it is being published on every address."
          busy=true
        fi
        ;;
      esac
    done
  done

  if [[ "$busy" != "true" ]]; then
    return 0
  fi

  printf "
Free the port(s), or pick different ones with --listen-http-port and --listen-https-port
(mind their limitations, see --help), or publish on another address with --listen-address.

Use the following command to see what holds them:

"
  if command -v lsof >/dev/null 2>&1; then
    printf "    lsof -nP -iTCP:%s -iTCP:%s -sTCP:LISTEN\n\n" "$KIND_HTTP_PORT" "$KIND_HTTPS_PORT"
  elif command -v ss >/dev/null 2>&1; then
    printf "    ss -tlnp 'sport = :%s or sport = :%s'\n\n" "$KIND_HTTP_PORT" "$KIND_HTTPS_PORT"
  else
    printf "    netstat -an | grep LISTEN | grep -E '[.:](%s|%s)\\b'\n\n" "$KIND_HTTP_PORT" "$KIND_HTTPS_PORT"
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "(continuing anyway: --dry-run creates nothing)"
    return 0
  fi

  exit 1
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
    hostPort: ${KIND_HTTP_PORT}
    listenAddress: "${KIND_LISTEN_ADDRESS}"
    protocol: TCP
  - containerPort: 443
    hostPort: ${KIND_HTTPS_PORT}
    listenAddress: "${KIND_LISTEN_ADDRESS}"
    protocol: TCP
EOF

  # MutatingAdmissionPolicy is beta and disabled by default in Kubernetes 1.34, so the API has to
  # be switched on explicitly before arch_workaround_apply() can create the policy.
  if [[ "$ARCH_WORKAROUND" == "true" ]]; then
    cat <<EOF >>${CONFIG_DIR}/kind.cfg
featureGates:
  MutatingAdmissionPolicy: true
runtimeConfig:
  "admissionregistration.k8s.io/v1beta1": true
EOF
  fi

  print_dry_run_file "${CONFIG_DIR}/kind.cfg"

  # These two are the ModuleConfigs the install-deckhouse phase itself consumes, and the ones dhctl
  # rewrites on the way in: it removes deckhouse.releaseChannel (so that Deckhouse does not update
  # itself mid-bootstrap) and global.modules.https, and restores them only at the very end of a full
  # 'dhctl bootstrap'. This script runs the phases as separate commands, so that never happens - which
  # is why they are written to their own file and applied twice: as configuration for
  # install-deckhouse, and again as resources afterwards (create-resources patches what exists).
  # Without the second pass the release channel stays unset and every module is looked up in Stable
  # instead, leaving ingress-nginx and prometheus undeployed.
  # SelfSigned is spelled out as CertManager with the selfsigned ClusterIssuer that the cert-manager
  # module ships: cert-manager then issues a certificate for every module's own domain under
  # publicDomainTemplate, so no wildcard certificate has to be created by hand.
  local https_settings
  case "$D8_HTTPS_MODE" in
  SelfSigned)
    https_settings=$'      https:\n        mode: CertManager\n        certManager:\n          clusterIssuerName: selfsigned'
    ;;
  *)
    https_settings=$'      https:\n        mode: '"${D8_HTTPS_MODE}"
    ;;
  esac

  # The cluster Kubernetes version, taken from the node image tag the way create-d8-cluster-configuration.sh
  # cuts it (major.minor). It is set in the control-plane-manager ModuleConfig rather than in
  # ClusterConfiguration: the field there is deprecated, and the alert about it fires simply because the
  # field is present, even when the ModuleConfig already decides the version. Pinning the node image
  # version matters - "Default" would let Deckhouse pick the version it considers stable for the release
  # and then try to upgrade a control plane whose node image cannot change.
  local k8s_version="${KIND_IMAGE_TAG#v}"
  k8s_version="${k8s_version%.*}"

  # Deckhouse refuses to enable a module in the Experimental stage unless it is opted into, either
  # wholesale or by name. See --allow-experimental-modules.
  local experimental_settings=
  local module_name=
  if [[ "$D8_ALLOW_EXPERIMENTAL" == "true" ]]; then
    experimental_settings=$'    allowExperimentalModules: true\n'
  elif [[ -n "$D8_EXPERIMENTAL_MODULES" ]]; then
    experimental_settings=$'    allowedExperimentalModules:\n'
    for module_name in ${D8_EXPERIMENTAL_MODULES//,/ }; do
      experimental_settings+="    - ${module_name}"$'\n'
    done
  fi

  echo "Creating Deckhouse Kubernetes Platform core module configuration file (${CONFIG_DIR}/core-module-configs.yml)..."
  cat <<EOF >${CONFIG_DIR}/core-module-configs.yml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings:
    defaultClusterStorageClass: sc-lpp-default
    modules:
      publicDomainTemplate: "%s.${D8_PUBLIC_DOMAIN}"
${https_settings}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
${experimental_settings}    bundle: Minimal
    releaseChannel: $D8_RELEASE_CHANNEL_NAME
    logLevel: Info
    update:
      mode: Manual
EOF
  print_dry_run_file "${CONFIG_DIR}/core-module-configs.yml"

  echo "Creating Deckhouse Kubernetes Platform installation config file (${CONFIG_DIR}/config.yml)..."
  cat "${CONFIG_DIR}/core-module-configs.yml" >${CONFIG_DIR}/config.yml
  cat <<EOF >>${CONFIG_DIR}/config.yml
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: $D8_REGISTRY_PATH
EOF

  if [[ -n "$D8_LICENSE_KEY" ]]; then
    generate_ee_access_string "$D8_LICENSE_KEY"
    cat <<EOF >>${CONFIG_DIR}/config.yml
  registryDockerCfg: $D8_EE_ACCESS_STRING
EOF
  fi

  if [[ -n "$D8_DEV_BRANCH" ]]; then
    cat <<EOF >>${CONFIG_DIR}/config.yml
  devBranch: $D8_DEV_BRANCH
EOF
  fi
  print_dry_run_file "${CONFIG_DIR}/config.yml"

  # The create-resources phase applies these in a retry loop, refreshing the API discovery cache on
  # every iteration. That is what makes the ordering work out on its own: a ModuleConfig of a module
  # delivered through a ModuleSource (ingress-nginx, prometheus, ...) is accepted only once the
  # modules it depends on are enabled, and IngressNginxController only exists as a kind after the
  # ingress-nginx module has been deployed. dhctl cannot apply those ModuleConfigs during
  # install-deckhouse anyway: it has no validation schema for them in the installer image and
  # therefore treats them as resources rather than as configuration.
  echo "Creating Deckhouse Kubernetes Platform resource file (${CONFIG_DIR}/resources.yml)..."
  cat "${CONFIG_DIR}/core-module-configs.yml" >${CONFIG_DIR}/resources.yml
  cat <<EOF >>${CONFIG_DIR}/resources.yml
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: local-path-provisioner
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cert-manager
spec:
  enabled: true
EOF

  # These three only make sense together with Dex, which exists only when https is not Disabled:
  # user-authn turns itself off in that case ("https.mode is set to 'Disabled'"), the console gets no
  # Ingress from the charts, and control-plane-manager - whose whole job here is to make
  # kube-apiserver trust Dex - would spend several minutes rerendering the control plane for nothing.
  if [[ "$D8_HTTPS_MODE" != "Disabled" ]]; then
    cat <<EOF >>${CONFIG_DIR}/resources.yml
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 3
  enabled: true
  settings:
    kubernetesVersion: "${k8s_version}"
    # Once this module manages the control plane it renders kube-apiserver's --feature-gates itself,
    # dropping whatever kind put there - including the MutatingAdmissionPolicy gate the arch
    # workaround needs (its policies would still exist but never be applied). Asking for the gate
    # here is the module's own supported way to keep it on.
    enabledFeatureGates:
    - MutatingAdmissionPolicy
    apiserver:
      auditPolicyEnabled: false
      basicAuditPolicyEnabled: false
      publishAPI:
        ingress:
          addKubeconfigGeneratorEntry: true
          enabled: true
          https:
            global:
              kubeconfigGeneratorMasterCA: ""
            mode: Global
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: console
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
EOF
  fi

  cat <<EOF >>${CONFIG_DIR}/resources.yml
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authz
spec:
  version: 1
  enabled: true
  settings:
    enableMultiTenancy: false
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: ingress-nginx
spec:
  enabled: true
EOF

  # Optional, see --with-documentation.
  if [[ "$D8_WITH_DOCUMENTATION" == "true" ]]; then
    cat <<EOF >>${CONFIG_DIR}/resources.yml
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  enabled: true
EOF
  fi

  # The Prometheus stack is opt-in, see --with-metrics. These four have to stay in dependency order -
  # operator-prometheus, then prometheus, then monitoring-kubernetes - because that is the order
  # create-resources works through the file. The ModuleConfig webhook rejects a module whose
  # dependencies are not enabled yet ("the 'prometheus' module depends on disabled module(s):
  # operator-prometheus"), and a rejected resource aborts the whole iteration in dhctl (resources.go,
  # createAll returns on the first error) after retrying it for ten minutes. With the dependency
  # listed later in the file it would never get created, so the rejection could never clear and the
  # phase would fail.
  if [[ "$D8_WITH_METRICS" == "true" ]]; then
    cat <<EOF >>${CONFIG_DIR}/resources.yml
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
  name: prometheus
spec:
  enabled: true
  settings:
    longtermRetentionDays: 0
    retentionDays: 7
  version: 1
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
  name: prometheus-metrics-adapter
spec:
  enabled: true
EOF
  fi

  # Optional, see --with-registry-packages-proxy.
  if [[ "$D8_WITH_REGISTRY_PACKAGES_PROXY" == "true" ]]; then
    cat <<EOF >>${CONFIG_DIR}/resources.yml
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry-packages-proxy
spec:
  enabled: true
EOF
  fi

  # Custom resources go last on purpose: their kinds only exist once the module that ships the CRD has
  # been deployed, and create-resources aborts the whole iteration on the first resource it cannot
  # create (resources.go, createAll returns on the first error). Kept at the end, the wait for those
  # CRDs blocks nothing - every ModuleConfig above has already been applied, so the modules install in
  # parallel while these wait.
  cat <<EOF >>${CONFIG_DIR}/resources.yml
---
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: sc-lpp-default
spec:
  path: /opt/local-path-provisioner/default
  reclaimPolicy: Delete
---
apiVersion: deckhouse.io/v2
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

  # The user only makes sense together with Dex, and the user-authn module turns itself off while
  # https is Disabled ("turned off by enabled script: https.mode is set to 'Disabled'"). Its CRDs
  # would then be missing and create-resources would keep retrying the User until it times out.
  if [[ "$D8_HTTPS_MODE" != "Disabled" ]]; then
    if [[ -n "$D8_ADMIN_PASSWORD" ]]; then
      echo "Creating the ${D8_ADMIN_EMAIL} user with the password from --admin-password..."
    else
      D8_ADMIN_PASSWORD=$(LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 20)
      echo "Creating the ${D8_ADMIN_EMAIL} user with a generated password..."
    fi
    cat <<EOF >>${CONFIG_DIR}/resources.yml
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: $D8_ADMIN_EMAIL
  password: $(bcrypt_hash "$D8_ADMIN_PASSWORD")
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: $D8_ADMIN_EMAIL
  accessLevel: SuperAdmin
  portForwarding: true
EOF
  fi

  print_dry_run_file "${CONFIG_DIR}/resources.yml"
}

cluster_deletion_info() {

    printf "
To delete created cluster use the following command:

    ${KIND_PATH} delete cluster --name "${KIND_CLUSTER_NAME}"

"
}

cluster_create() {

  run_cmd ${KIND_PATH} create cluster --name "${KIND_CLUSTER_NAME}" --image "${KIND_IMAGE_NAME}:${KIND_IMAGE_TAG}" --config "${CONFIG_DIR}/kind.cfg"

  if [ "$?" -ne "0" ]; then
    printf "
Error creating cluster. If error is like '...port is already allocated' or '... address already in use', then you need to free ports 80 and 443.
E.g., you can find programs that use these ports using the following command:

    sudo lsof -n -i TCP@0.0.0.0:80,443 -s TCP:LISTEN

"
    cluster_deletion_info
    exit 1
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    printf '+ %s\n' "${KIND_PATH} get kubeconfig --internal --name ${KIND_CLUSTER_NAME} > ${CONFIG_DIR}/kubeconfig"
  else
    ${KIND_PATH} get kubeconfig --internal --name "${KIND_CLUSTER_NAME}" >${CONFIG_DIR}/kubeconfig
  fi

}

# Pins module installation to the symlink layout - the one that needs nothing from the kernel.
#
# Deckhouse installs modules either as erofs images on dm-verity devices or as plain directories with
# symlinks, and takes the first branch when verity.IsSupported() manages to open a dm-verity mapping
# for a scratch file. In kind the outcome of that check is decided by the host rather than by this
# script: docker copies whatever /dev/loop* exist at container creation time into the node, so the
# same command produces a different module layout on different machines. The erofs path also needs
# free loop devices later on - when they run out, the controller quietly falls back to symlinks on
# its next restart, which is the same stand behaving in two ways over its lifetime.
#
# Removing the loop devices settles it: veritysetup answers "cannot find an unused loop device", the
# check fails immediately and identically everywhere, and the symlink installer is used. Nothing else
# in a kind cluster needs them.
#
# This assumes a controller whose verity.IsSupported() actually tries to open a mapping (deckhouse
# PR 22183). Older builds decide by looking for erofs in /proc/filesystems alone, and their erofs
# installer has no fallback - on a host with erofs loaded such a build fails to install modules here.
#
# The devices are only removed from the node's own tmpfs /dev, where they are copies docker made at
# creation time. With the host /dev bind-mounted in (devtmpfs) the same command would delete the
# machine's device nodes, hence the filesystem type check.
loop_devices_drop() {
  local node="${KIND_CLUSTER_NAME}-control-plane"
  local dev_fstype=

  if [[ "$DRY_RUN" == "true" ]]; then
    printf '+ %s\n' "docker exec ${node} sh -c 'rm -f /dev/loop-control /dev/loop[0-9]*' (only if the node's /dev is a tmpfs)"
    return 0
  fi

  dev_fstype=$(docker exec "${node}" sh -c "grep -E ' /dev ' /proc/mounts | head -1 | cut -d' ' -f3" 2>/dev/null | tr -d '\r')

  if [[ "$dev_fstype" != "tmpfs" ]]; then
    echo "The /dev of ${node} is ${dev_fstype:-of an unknown type}, not the tmpfs docker creates - leaving the loop devices alone."
    echo "Modules may then be installed as erofs images on dm-verity devices rather than as directories."
    return 0
  fi

  echo "Removing loop devices from ${node} (modules are installed as directories, not erofs images)..."

  docker exec "${node}" sh -c 'rm -f /dev/loop-control /dev/loop[0-9]*'
}

# Removes the storage kind installs on its own, because Deckhouse brings its own local-path
# provisioner (the local-path-provisioner module with its sc-lpp-default class, named as
# global.defaultClusterStorageClass).
#
# kind cannot be told to skip it: its config API has no storage option - networking.disableDefaultCNI
# is the only switch of that kind - and the install step is appended unconditionally, right next to a
# "TODO: make this controllable from the command line?" in pkg/cluster/internal/create/create.go. The
# manifest itself is baked into the node image (/kind/manifests/default-storage.yaml) and applied with
# kubectl, so the only option left is to delete the objects afterwards.
#
# Leaving them in place would give the cluster two local-path provisioners and two StorageClasses
# annotated as the default one, and Kubernetes picks between such classes non-deterministically - a
# PVC without an explicit storageClassName could land on kind's class, which Deckhouse does not manage.
# This runs before Deckhouse is installed so that no volume ever binds to that class: deleting a
# StorageClass that already has bound volumes would leave them orphaned.
default_storage_drop() {
  local ctx="kind-${KIND_CLUSTER_NAME}"

  echo "Removing the StorageClass kind installs (Deckhouse ships its own provisioner)..."

  # Deleting the namespace takes the ServiceAccount, Role, RoleBinding, Deployment and ConfigMap with
  # it, but the cluster-scoped RBAC objects are not part of it - hence the separate deletes. Older node
  # images ship only the legacy host-path StorageClass and none of the rest, so nothing here may fail
  # on a missing object.
  run_cmd ${KUBECTL_PATH} --context "$ctx" delete storageclass standard --ignore-not-found
  run_cmd ${KUBECTL_PATH} --context "$ctx" delete namespace local-path-storage --ignore-not-found --wait=false
  run_cmd ${KUBECTL_PATH} --context "$ctx" delete clusterrole local-path-provisioner-role --ignore-not-found
  run_cmd ${KUBECTL_PATH} --context "$ctx" delete clusterrolebinding local-path-provisioner-bind --ignore-not-found
}

arch_workaround_apply() {
  if [[ "$ARCH_WORKAROUND" != "true" ]]; then
    return 0
  fi

  echo "Creating amd64 scheduling policy file (${CONFIG_DIR}/arch-policy.yml)..."

  # The policy only touches Pods that require amd64 and drops the whole nodeAffinity, since that is
  # the only thing Deckhouse charts put there (a podAntiAffinity, if any, is a separate field and is
  # left intact).
  cat <<EOF >${CONFIG_DIR}/arch-policy.yml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicy
metadata:
  name: kind-d8-strip-arch-node-affinity
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["pods"]
  matchConditions:
  - name: requires-amd64-arch
    expression: >-
      has(object.spec.affinity) &&
      has(object.spec.affinity.nodeAffinity) &&
      has(object.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution) &&
      object.spec.affinity.nodeAffinity.requiredDuringSchedulingIgnoredDuringExecution.nodeSelectorTerms.exists(t,
        has(t.matchExpressions) &&
        t.matchExpressions.exists(e, e.key == 'kubernetes.io/arch' && e.operator == 'In' && e.values.exists(v, v == 'amd64')))
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded
  mutations:
  - patchType: JSONPatch
    jsonPatch:
      expression: >-
        [JSONPatch{op: "remove", path: "/spec/affinity/nodeAffinity"}]
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: kind-d8-strip-arch-node-affinity
spec:
  policyName: kind-d8-strip-arch-node-affinity
  matchResources: {}
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicy
metadata:
  name: kind-d8-libevent-no-epoll
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["pods"]
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded
  mutations:
  # An amd64 binary that calls epoll_wait(2) directly cannot work under emulation on an arm64 host:
  # arm64 kernels only implement epoll_pwait(2), so the emulator (both Rosetta and QEMU) fails the
  # call with ENOSYS. libevent then logs "epoll_wait: Function not implemented" in a tight loop and
  # the process never serves anything - this is what makes the memcached container of the prometheus
  # module crash-loop. EVENT_NOEPOLL makes libevent pick its poll backend instead, which works.
  # It is set for every container: the variable is only meaningful to libevent-based programs, and
  # any of them hits the same wall here.
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >-
        Object{
          spec: Object.spec{
            containers: object.spec.containers.map(c, Object.spec.containers{
              name: c.name,
              env: [Object.spec.containers.env{name: "EVENT_NOEPOLL", value: "1"}]
            })
          }
        }
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: kind-d8-libevent-no-epoll
spec:
  policyName: kind-d8-libevent-no-epoll
  matchResources: {}
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicy
metadata:
  name: kind-d8-relax-resource-limits
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["pods"]
  matchConditions:
  - name: has-a-tight-limit
    expression: >-
      (object.spec.containers + (has(object.spec.initContainers) ? object.spec.initContainers : [])).exists(c,
        has(c.resources) && has(c.resources.limits) && (
          ('cpu' in c.resources.limits && quantity(string(c.resources.limits.cpu)).isLessThan(quantity("1"))) ||
          ('memory' in c.resources.limits && quantity(string(c.resources.limits.memory)).isLessThan(quantity("256Mi")))))
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded
  mutations:
  # A limit that is generous natively can be far too tight under emulation, where the same work costs
  # several times more CPU and memory. Two cases seen here:
  #   - the config-reloader containers of the prometheus module carry a 10m CPU limit and, throttled
  #     to 1% of a core, never even finish starting the Go runtime - no log line, Pod stuck in Init;
  #   - the self-signed-generator init container of user-api carries a 25Mi memory limit and is
  #     OOMKilled while generating its certificate.
  # Only limits that are already set and smaller than the target are raised, so nothing that was
  # unlimited becomes limited and no bigger limit is ever lowered (the Deckhouse controller itself
  # runs with 6Gi). Requests are left alone.
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >-
        Object{
          spec: Object.spec{
            containers: object.spec.containers.filter(c,
                has(c.resources) && has(c.resources.limits) && 'cpu' in c.resources.limits &&
                quantity(string(c.resources.limits.cpu)).isLessThan(quantity("1"))
              ).map(c, Object.spec.containers{
                name: c.name,
                resources: Object.spec.containers.resources{limits: {"cpu": "1"}}
              }),
            initContainers: (has(object.spec.initContainers) ? object.spec.initContainers : []).filter(c,
                has(c.resources) && has(c.resources.limits) && 'cpu' in c.resources.limits &&
                quantity(string(c.resources.limits.cpu)).isLessThan(quantity("1"))
              ).map(c, Object.spec.initContainers{
                name: c.name,
                resources: Object.spec.initContainers.resources{limits: {"cpu": "1"}}
              })
          }
        }
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >-
        Object{
          spec: Object.spec{
            containers: object.spec.containers.filter(c,
                has(c.resources) && has(c.resources.limits) && 'memory' in c.resources.limits &&
                quantity(string(c.resources.limits.memory)).isLessThan(quantity("256Mi"))
              ).map(c, Object.spec.containers{
                name: c.name,
                resources: Object.spec.containers.resources{limits: {"memory": "256Mi"}}
              }),
            initContainers: (has(object.spec.initContainers) ? object.spec.initContainers : []).filter(c,
                has(c.resources) && has(c.resources.limits) && 'memory' in c.resources.limits &&
                quantity(string(c.resources.limits.memory)).isLessThan(quantity("256Mi"))
              ).map(c, Object.spec.initContainers{
                name: c.name,
                resources: Object.spec.initContainers.resources{limits: {"memory": "256Mi"}}
              })
          }
        }
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: kind-d8-relax-resource-limits
spec:
  policyName: kind-d8-relax-resource-limits
  matchResources: {}
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicy
metadata:
  name: kind-d8-drop-rosetta-broken-containers
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE"]
      resources: ["pods"]
  # Rosetta's own path resolution breaks for this container as soon as the Pod shares the host PID
  # namespace: it dies with "assertion failed [true_path_length_other >= 0] ... is_rosetta_process"
  # and crash-loops forever. Bisected on identical Pods - same image, same mounts, same
  # securityContext - the container runs fine without hostPID and always fails with it, so the
  # condition below is deliberately narrow: with hostPID removed nothing would be dropped.
  # Per-container emulation cannot be chosen instead: OrbStack only exposes Rosetta as a global
  # setting, and QEMU - the only alternative - starves the metrics vault lock the shell-operator task
  # queues take for every measured action, so the controller never finishes installing modules.
  # The cost is losing the kubelet eviction threshold metrics; the other node-exporter containers
  # keep working.
  matchConditions:
  - name: broken-under-rosetta-with-hostpid
    expression: >-
      (has(object.spec.hostPID) ? object.spec.hostPID : false) &&
      object.spec.containers.exists(c, c.name == 'kubelet-eviction-thresholds-exporter')
  failurePolicy: Fail
  reinvocationPolicy: IfNeeded
  mutations:
  # A JSONPatch removes list entries by index, which CEL cannot enumerate directly - hence the fixed
  # range, generous enough for any Pod this matches (node-exporter has three containers).
  # The per-container AppArmor annotation has to go as well: the API server rejects the whole Pod
  # when an annotation names a container that no longer exists. ~1 is the JSON Pointer escape for /.
  - patchType: JSONPatch
    jsonPatch:
      expression: >-
        [0, 1, 2, 3, 4, 5, 6, 7].filter(i,
            i < object.spec.containers.size() &&
            object.spec.containers[i].name == 'kubelet-eviction-thresholds-exporter'
          ).map(i, JSONPatch{op: "remove", path: "/spec/containers/" + string(i)}) +
        (has(object.metadata.annotations) &&
         'container.apparmor.security.beta.kubernetes.io/kubelet-eviction-thresholds-exporter' in object.metadata.annotations
          ? [JSONPatch{op: "remove", path: "/metadata/annotations/container.apparmor.security.beta.kubernetes.io~1kubelet-eviction-thresholds-exporter"}]
          : [])
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingAdmissionPolicyBinding
metadata:
  name: kind-d8-drop-rosetta-broken-containers
spec:
  policyName: kind-d8-drop-rosetta-broken-containers
  matchResources: {}
EOF
  print_dry_run_file "${CONFIG_DIR}/arch-policy.yml"

  run_cmd ${KUBECTL_PATH} --context "kind-${KIND_CLUSTER_NAME}" apply -f "${CONFIG_DIR}/arch-policy.yml"

  if [ "$?" -ne "0" ]; then
    echo "Error applying the amd64 scheduling policy!"
    cluster_deletion_info
    exit 1
  fi
}

# Installs the kubernetes-api-proxy static Pod the way bashible does on a normal node (its steps
# 001_configure_kubernetes_api_proxy and 051_pull_and_configure_kubernetes_api_proxy), because
# nothing runs bashible here. It has to exist before control-plane-manager is enabled: from that
# moment Deckhouse points itself at 127.0.0.1:6445, and without the proxy the controller cannot even
# start - so it can never deploy anything, including the proxy itself.
api_proxy_install() {
  local node="${KIND_CLUSTER_NAME}-control-plane"
  local node_ip=
  local digest=
  local image=

  echo "Installing the kubernetes-api-proxy static Pod on ${node}..."

  if [[ "$DRY_RUN" == "true" ]]; then
    printf '+ %s\n' "docker exec ${node} sh -c 'write /etc/kubernetes/kubernetes-api-proxy/{upstreams.json,ca.crt}'"
    printf '+ %s\n' "docker exec ${node} crictl pull --creds license-token:<key> <imagesRepo>@<kubernetesApiProxy digest>"
    printf '+ %s\n' "docker exec ${node} sh -c 'write /etc/kubernetes/manifests/kubernetes-api-proxy.yaml'"
    return 0
  fi

  node_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$node")
  if [[ -z "$node_ip" ]]; then
    echo "Error getting the IP address of ${node}!"
    cluster_deletion_info
    exit 1
  fi

  # The image reference is built exactly like the bashible step does: the images repo plus the
  # nodeManager.kubernetesApiProxy digest, which is shipped inside the Deckhouse image itself.
  digest=$(docker run --rm --entrypoint sh "${D8_REGISTRY_PATH}:${D8_RELEASE_CHANNEL_TAG}" \
    -c 'jq -r ".nodeManager.kubernetesApiProxy" /deckhouse/modules/images_digests.json' 2>"${CONFIG_DIR}/api-proxy-digest.err" | tr -d '\r')
  if [[ -z "$digest" || "$digest" == "null" ]]; then
    echo "Error getting the kubernetes-api-proxy image digest from ${D8_REGISTRY_PATH}:${D8_RELEASE_CHANNEL_TAG}!"
    tail -2 "${CONFIG_DIR}/api-proxy-digest.err" 2>/dev/null
    cluster_deletion_info
    exit 1
  fi
  image="${D8_REGISTRY_PATH}@${digest}"

  docker exec "$node" sh -c "mkdir -p /etc/kubernetes/kubernetes-api-proxy && \
    printf '[\"%s:6443\"]' '${node_ip}' > /etc/kubernetes/kubernetes-api-proxy/upstreams.json && \
    cp /etc/kubernetes/pki/ca.crt /etc/kubernetes/kubernetes-api-proxy/ca.crt"

  # A static Pod cannot use imagePullSecrets, and the node's containerd has no registry credentials
  # (bashible would have configured them), so the image is pulled up front - the manifest below uses
  # imagePullPolicy: IfNotPresent.
  if [[ -n "$D8_LICENSE_KEY" ]]; then
    docker exec "$node" crictl pull --creds "license-token:${D8_LICENSE_KEY}" "$image" >/dev/null
  else
    docker exec "$node" crictl pull "$image" >/dev/null
  fi

  if [ "$?" -ne "0" ]; then
    echo "Error pulling the kubernetes-api-proxy image on ${node}!"
    cluster_deletion_info
    exit 1
  fi

  docker exec -i "$node" sh -c 'cat > /etc/kubernetes/manifests/kubernetes-api-proxy.yaml' <<EOF
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kubernetes-api-proxy
    tier: control-plane
  name: kubernetes-api-proxy
  namespace: kube-system
spec:
  priorityClassName: system-node-critical
  priority: 2000001000
  hostNetwork: true
  dnsPolicy: ClusterFirstWithHostNet
  securityContext:
    fsGroup: 64535
  volumes:
    - name: certs
      hostPath:
        path: /etc/kubernetes/kubernetes-api-proxy
        type: Directory
    - name: upstreams
      hostPath:
        path: /etc/kubernetes/kubernetes-api-proxy/upstreams.json
        type: FileOrCreate
  containers:
    - name: kubernetes-api-proxy
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop:
          - ALL
          add:
          - DAC_OVERRIDE
          - SETGID
          - SETUID
        readOnlyRootFilesystem: true
        runAsGroup: 0
        runAsNonRoot: false
        runAsUser: 0
        seccompProfile:
          type: RuntimeDefault
      image: ${image}
      imagePullPolicy: IfNotPresent
      args:
        - "--listen-address=127.0.0.1"
        - "--listen-port=6445"
        - "--health-listen=127.0.0.1:6480"
        - "--log-level=debug"
        - "--as-static-pod=true"
        - "--fallback-file=/var/run/kubernetes.io/kubernetes-api-proxy/upstreams.json"
      ports:
        - name: https
          containerPort: 6445
          hostPort: 6445
      livenessProbe:
        httpGet:
          path: /healthz
          port: 6480
          host: 127.0.0.1
        initialDelaySeconds: 2
        periodSeconds: 10
      resources:
        requests:
          cpu: 50m
          memory: 64Mi
        limits:
          cpu: 500m
          memory: 256Mi
      volumeMounts:
        - name: certs
          mountPath: /var/run/kubernetes.io/kubernetes-api-proxy
          readOnly: true
        - name: upstreams
          mountPath: /var/run/kubernetes.io/kubernetes-api-proxy/upstreams.json
          readOnly: false
EOF

  local retries=0
  echo -n "Waiting for the kubernetes-api-proxy to start listening..."
  while ! docker exec "$node" sh -c 'ss -tln 2>/dev/null | grep -q "127.0.0.1:6445"'; do
    if [[ "$retries" -ge 60 ]]; then
      echo
      echo "Timeout waiting for the kubernetes-api-proxy on ${node}!"
      cluster_deletion_info
      exit 1
    fi
    echo -n "."
    retries=$((retries + 1))
    sleep 5
  done
  echo
  echo "kubernetes-api-proxy is listening on 127.0.0.1:6445."
}

deckhouse_install() {
  if [[ -n "$D8_DEV_BRANCH" ]]; then
    echo "Running Deckhouse Kubernetes Platform installation (devBranch: $D8_DEV_BRANCH, installer tag: $D8_RELEASE_CHANNEL_TAG)..."
  else
    echo "Running Deckhouse Kubernetes Platform installation (the $D8_RELEASE_CHANNEL_NAME release channel)..."
  fi

  # Both phases run in one installer container, chained with && so that the resources are only
  # created if Deckhouse itself came up. Each phase gets the file it is meant to read: the
  # installation configuration for install-deckhouse, the resources for create-resources (dhctl's
  # --config accepts either, and the deprecated --resources flag points at --config as well).
  run_cmd docker run --pull=always --rm --network kind -v "${CONFIG_DIR}/config.yml:/config.yml" -v "${CONFIG_DIR}/resources.yml:/resources.yml" \
    -v "${CONFIG_DIR}/kubeconfig:/kubeconfig" ${D8_REGISTRY_PATH}/install:$D8_RELEASE_CHANNEL_TAG \
    bash -c "dhctl bootstrap-phase install-deckhouse --kubeconfig=/kubeconfig --kubeconfig-context=kind-${KIND_CLUSTER_NAME} --config=/config.yml && \
             dhctl bootstrap-phase create-resources --kubeconfig=/kubeconfig --kubeconfig-context=kind-${KIND_CLUSTER_NAME} --config=/resources.yml --resources-timeout=${D8_RESOURCES_TIMEOUT}"

  if [ "$?" -ne "0" ]; then
    echo "Error installing Deckhouse Kubernetes Platform!"
    cluster_deletion_info
    exit 1
  fi
}

# Turns on control-plane-manager, which is what configures kube-apiserver to trust Dex - without it
# every web UI (console, Grafana) logs in but gets 401 from the Kubernetes API it proxies to.
# The module switches itself off with "ClusterConfiguration is not set" whenever Deckhouse is
# installed into an existing cluster, and it expects three things a normal bootstrap would have left
# behind, all of which are created here: the d8-cluster-configuration secret (dhctl), the d8-pki
# secret (bashible step 072_install_control_plane) and the NodeGroup CRD (the node-manager module,
# which cannot be enabled on a kind node - bashible refuses to run where containerd is already
# installed).
control_plane_manager_enable() {
  local ctx="kind-${KIND_CLUSTER_NAME}"
  local node="${KIND_CLUSTER_NAME}-control-plane"

  # Nothing to trust without Dex, and taking over the control plane costs several minutes: the module
  # reissues etcd, kube-apiserver, kube-controller-manager and kube-scheduler one after another.
  if [[ "$D8_HTTPS_MODE" == "Disabled" ]]; then
    echo "Skipping the control-plane-manager module: there is no Dex to trust with --https-mode Disabled."
    return 0
  fi

  echo "Enabling the control-plane-manager module..."

  if [[ "$DRY_RUN" == "true" ]]; then
    printf '+ %s\n' "${KUBECTL_PATH} --context ${ctx} -n kube-system create secret generic d8-pki --from-file=... (from the node PKI)"
    printf '+ %s\n' "${KUBECTL_PATH} --context ${ctx} apply -f <NodeGroup CRD>"
    printf '+ %s\n' "${KUBECTL_PATH} --context ${ctx} -n kube-system apply -f <d8-cluster-configuration secret>"
    printf '+ %s\n' "${KUBECTL_PATH} --context ${ctx} -n d8-system delete pod -l app=deckhouse (to re-read the configuration)"
    return 0
  fi

  # The module's hooks read the cluster CA and keys from this secret.
  if ! ${KUBECTL_PATH} --context "$ctx" -n kube-system get secret d8-pki >/dev/null 2>&1; then
    local pki_dir
    pki_dir=$(mktemp -d)
    local pair
    for pair in "ca.crt:pki/ca.crt" "ca.key:pki/ca.key" "sa.pub:pki/sa.pub" "sa.key:pki/sa.key" \
      "front-proxy-ca.crt:pki/front-proxy-ca.crt" "front-proxy-ca.key:pki/front-proxy-ca.key" \
      "etcd-ca.crt:pki/etcd/ca.crt" "etcd-ca.key:pki/etcd/ca.key"; do
      docker exec "$node" cat "/etc/kubernetes/${pair##*:}" >"${pki_dir}/${pair%%:*}"
    done

    ${KUBECTL_PATH} --context "$ctx" -n kube-system create secret generic d8-pki \
      --from-file=ca.crt="${pki_dir}/ca.crt" --from-file=ca.key="${pki_dir}/ca.key" \
      --from-file=sa.pub="${pki_dir}/sa.pub" --from-file=sa.key="${pki_dir}/sa.key" \
      --from-file=front-proxy-ca.crt="${pki_dir}/front-proxy-ca.crt" \
      --from-file=front-proxy-ca.key="${pki_dir}/front-proxy-ca.key" \
      --from-file=etcd-ca.crt="${pki_dir}/etcd-ca.crt" --from-file=etcd-ca.key="${pki_dir}/etcd-ca.key"
    rm -rf "$pki_dir"
  fi

  # Only the kind of CRD the module needs to build its client: it watches NodeGroups, and without the
  # kind registered its container dies with "no matches for kind \"NodeGroup\"". The schema is left
  # open on purpose - nothing here creates NodeGroups.
  cat <<EOF >${CONFIG_DIR}/nodegroup-crd.yml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodegroups.deckhouse.io
spec:
  group: deckhouse.io
  scope: Cluster
  names:
    plural: nodegroups
    singular: nodegroup
    kind: NodeGroup
  versions:
  - name: v1
    served: true
    storage: true
    subresources:
      status: {}
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-preserve-unknown-fields: true
EOF
  ${KUBECTL_PATH} --context "$ctx" apply -f "${CONFIG_DIR}/nodegroup-crd.yml"

  cat <<EOF >${CONFIG_DIR}/cluster-configuration.yml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.244.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.96.0.0/16
clusterDomain: cluster.local
EOF
  print_dry_run_file "${CONFIG_DIR}/cluster-configuration.yml"

  ${KUBECTL_PATH} --context "$ctx" -n kube-system create secret generic d8-cluster-configuration \
    --from-file=cluster-configuration.yaml="${CONFIG_DIR}/cluster-configuration.yml" \
    --dry-run=client -o yaml |
    ${KUBECTL_PATH} --context "$ctx" label --local -f - validation.deckhouse.io/selector=d8-cluster-configuration --dry-run=client -o yaml |
    ${KUBECTL_PATH} --context "$ctx" apply -f -

  # The secret is only read at startup, so the controller has to come up again to see it.
  ${KUBECTL_PATH} --context "$ctx" -n d8-system delete pod -l app=deckhouse --wait=false >/dev/null 2>&1

  local retries=0
  echo -n "Waiting for the control-plane-manager module to become ready..."
  while [[ "$(${KUBECTL_PATH} --context "$ctx" get module control-plane-manager -o jsonpath='{.status.phase}' 2>/dev/null)" != "Ready" ]]; do
    if [[ "$retries" -ge 80 ]]; then
      echo
      echo "Timeout waiting for the control-plane-manager module. The web UIs will still work, but"
      echo "everything they read from the Kubernetes API will answer 401."
      return 0
    fi
    echo -n "."
    retries=$((retries + 1))
    sleep 15
  done
  echo
  echo "control-plane-manager is ready, kube-apiserver now trusts Dex."
}

# control-plane-manager adds --kubelet-certificate-authority to kube-apiserver, so from then on the
# kubelet's serving certificate is verified against the cluster CA. The certificate kind's kubelet
# generates is self-signed and carries no IP, which breaks every kubectl exec and kubectl logs with
# "cannot validate certificate for <node ip> because it doesn't contain any IP SANs".
# The fix is the standard one: let kubelet request a serving certificate (serverTLSBootstrap) and
# approve the CSR. kube-controller-manager signs it, but deliberately does not auto-approve
# kubelet-serving requests - in a normal cluster the node-manager module approves them.
#
# This runs before control-plane-manager is enabled, even though the problem only appears afterwards,
# because restarting kubelet makes the node NotReady for a few seconds. The module creates its
# ControlPlaneOperations unapproved, and the controller that approves them watches operations only -
# not nodes - so a reconcile that lands while no ready master node exists gives up with "nodes not
# found" and is never retriggered: the operations cannot change until approved, and nothing else
# generates an event. The control plane then stays unmanaged until the module is restarted by hand.
kubelet_serving_cert_fix() {
  local ctx="kind-${KIND_CLUSTER_NAME}"
  local node="${KIND_CLUSTER_NAME}-control-plane"
  local csr=
  local retries=0

  # Only needed because control-plane-manager adds --kubelet-certificate-authority; without that
  # module kind's self-signed kubelet certificate is never verified and exec/logs keep working.
  if [[ "$D8_HTTPS_MODE" == "Disabled" ]]; then
    echo "Skipping the kubelet serving certificate: nothing verifies it without control-plane-manager."
    return 0
  fi

  echo "Requesting a serving certificate for kubelet..."

  if [[ "$DRY_RUN" == "true" ]]; then
    printf '+ %s\n' "docker exec ${node} sh -c 'set serverTLSBootstrap: true in /var/lib/kubelet/config.yaml; systemctl restart kubelet'"
    printf '+ %s\n' "${KUBECTL_PATH} --context ${ctx} certificate approve <kubernetes.io/kubelet-serving CSR>"
    return 0
  fi

  docker exec "$node" sh -c '
    C=/var/lib/kubelet/config.yaml
    cp -n "$C" /root/kubelet-config.yaml.bak 2>/dev/null || true
    if grep -q "^serverTLSBootstrap:" "$C"; then
      sed -i "s/^serverTLSBootstrap:.*/serverTLSBootstrap: true/" "$C"
    else
      echo "serverTLSBootstrap: true" >> "$C"
    fi
    systemctl restart kubelet'

  echo -n "Waiting for the kubelet serving certificate request..."
  while true; do
    csr=$(${KUBECTL_PATH} --context "$ctx" get csr -o jsonpath='{range .items[?(@.spec.signerName=="kubernetes.io/kubelet-serving")]}{.metadata.name} {.status.conditions[*].type}{"\n"}{end}' 2>/dev/null |
      awk '$2 == "" {print $1}' | head -1)
    if [[ -n "$csr" ]]; then
      break
    fi

    if [[ "$retries" -ge 40 ]]; then
      echo
      echo "No kubelet serving certificate request appeared; kubectl exec and kubectl logs may fail."
      return 0
    fi
    echo -n "."
    retries=$((retries + 1))
    sleep 5
  done
  echo

  ${KUBECTL_PATH} --context "$ctx" certificate approve "$csr"
  echo "Approved ${csr}; kubectl exec and kubectl logs work against this node."
}

# Prints the ":<port>" part to append to a URL, empty when the port is the scheme's default one.
# Deckhouse never puts a port anywhere itself - publicDomainTemplate, the Ingress hosts and the Dex
# redirect URIs are all plain DNS names - so this is only about the links the script prints and the
# endpoints it probes.
url_port_suffix() {
  if [[ "$1" == "https" ]]; then
    [[ "$KIND_HTTPS_PORT" != "443" ]] && printf ':%s' "$KIND_HTTPS_PORT"
  else
    [[ "$KIND_HTTP_PORT" != "80" ]] && printf ':%s' "$KIND_HTTP_PORT"
  fi
  return 0
}

# Prints a bcrypt hash of $1, the form the User resource wants its password in.
bcrypt_hash() {
  if command -v htpasswd >/dev/null 2>&1; then
    printf '%s' "$1" | htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n'
    return
  fi

  # htpasswd comes from apache2-utils and is not always installed, so fall back to a container -
  # docker is a hard requirement of this script anyway.
  printf '%s' "$1" | docker run --rm -i httpd:2-alpine htpasswd -BinC 10 "" | cut -d: -f2 | tr -d '\n'
}

generate_ee_access_string() {
  if [ "$OS_NAME" != "mac" ]; then B64_ARG="-w0"; else B64_ARG=""; fi
  auth_part=$(echo -n "license-token:$1" | base64 $B64_ARG)
  D8_EE_ACCESS_STRING=$(echo -n "{\"auths\": { \"$D8_REGISTRY_ADDRESS\": { \"username\": \"license-token\", \"password\": \"$1\", \"auth\": \"$auth_part\"}}}" | base64 $B64_ARG)

  if [ "$?" -ne "0" ]; then
    echo "Error generation container registry access string for Deckhouse Kubernetes Platform Enterprise Edition"
    exit 1
  fi
}

# Prints "<user>:<password>" for Grafana, or nothing at all while the prometheus module has not
# generated them yet.
# The credentials are read from the basic-auth secret the module creates for its Ingress, which holds
# an htpasswd line - "admin:{PLAIN}<password>" as long as the password is stored in the clear. The
# module values are only a fallback: the prometheus module is now delivered through a ModuleSource,
# and its newer versions no longer expose the password under internal.auth.
prometheus_credentials_get() {
  local auth=

  auth=$(${KUBECTL_PATH} --context "kind-${KIND_CLUSTER_NAME}" -n d8-monitoring get secret basic-auth -o jsonpath='{.data.auth}' 2>/dev/null |
    base64 -d 2>/dev/null | tr -d '\r')

  if [[ "$auth" == *":{PLAIN}"* ]]; then
    echo "${auth/\{PLAIN\}/}"
    return 0
  fi

  local password=
  password=$(${KUBECTL_PATH} --context "kind-${KIND_CLUSTER_NAME}" -n d8-system exec deploy/deckhouse -c deckhouse -- \
    sh -c "deckhouse-controller module values prometheus -o json | jq -r '.internal.auth.password'" 2>/dev/null |
    tr -d '\r' | grep -vx "null")

  if [[ -n "$password" ]]; then
    echo "admin:${password}"
  fi
}

install_show_credentials() {
  local retries_max=120
  local retries_count=0
  local prometheus_enabled=
  local prometheus_credentials=
  local scheme=http

  # With https enabled the user-authn module is running, so every web UI sits behind Dex and is
  # entered with the user created during the installation - the per-module basic auth of the
  # prometheus module is not used at all in that case.
  if [[ "$D8_HTTPS_MODE" != "Disabled" ]]; then
    if [[ "$D8_HTTPS_MODE" == "SelfSigned" ]]; then
      scheme=https
    fi

    printf "
Log in to the web UIs with the user created during the installation:

    Username: %s
    Password: %s

    Console: %s://console.%s%s/
" "$D8_ADMIN_EMAIL" "$D8_ADMIN_PASSWORD" "$scheme" "$D8_PUBLIC_DOMAIN" "$(url_port_suffix "$scheme")"

    # Grafana only exists together with the Prometheus stack, see --with-metrics.
    if [[ "$D8_WITH_METRICS" == "true" ]]; then
      printf "    Grafana: %s://grafana.%s%s/\n" "$scheme" "$D8_PUBLIC_DOMAIN" "$(url_port_suffix "$scheme")"
    fi

    if [[ "$D8_HTTPS_MODE" == "SelfSigned" ]]; then
      printf "
The certificates are issued by the cluster's own self-signed CA, so a browser will warn about them.
"
    fi

    echo
    return
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "+ ${KUBECTL_PATH} --context kind-${KIND_CLUSTER_NAME} -n d8-monitoring get secret basic-auth -o jsonpath={.data.auth} | base64 -d (retried until the credentials exist)"
    return
  fi

  prometheus_enabled=$(${KUBECTL_PATH} --context "kind-${KIND_CLUSTER_NAME}" get moduleconfig prometheus -o jsonpath='{.spec.enabled}' 2>/dev/null)
  if [[ "$prometheus_enabled" != "true" ]]; then
    printf "
The prometheus module is not enabled, so there are no Grafana credentials to show.

"
    return
  fi

  # This function's stdout ends up in info.txt, so progress goes to stderr. The prometheus module is
  # deployed long after the Ingress controller is ready, so its credentials do not exist yet at this
  # point - asking once would print an empty password.
  echo -n "Waiting for the Grafana credentials to become available..." >&2

  while true; do
    prometheus_credentials=$(prometheus_credentials_get)
    if [[ -n "$prometheus_credentials" ]]; then
      break
    fi

    if [[ "$retries_count" -ge "$retries_max" ]]; then
      echo >&2
      printf "
Timeout waiting for the Grafana credentials.

The prometheus module is probably still being deployed. Run the following command to get them:

    %s --context %s -n d8-monitoring get secret basic-auth -o jsonpath='{.data.auth}' | base64 -d

" "${KUBECTL_PATH}" "kind-${KIND_CLUSTER_NAME}"
      return
    fi

    echo -n "." >&2
    retries_count=$((retries_count + 1))
    sleep 10
  done

  echo >&2

  printf "
Provide following credentials to access Grafana at http://grafana.%s%s/ :

    Username: %s
    Password: %s

" "${D8_PUBLIC_DOMAIN}" "$(url_port_suffix http)" "${prometheus_credentials%%:*}" "${prometheus_credentials#*:}"
}

# Pins modules delivered through a ModuleSource to an exact version with ModulePullOverrides
# (--module-version-override). Release channel tags in a development registry are not curated - any
# branch's CI can move them, so a module can silently go backwards: every channel of the console was
# rewritten from v1.60.x to v1.45.11 at once, which is why a stand sometimes has to name the version
# it wants. While the override exists the module stops following its release channel, and that is
# what makes the version stick.
module_versions_apply() {
  if [[ -z "$D8_MODULE_VERSIONS" ]]; then
    return 0
  fi

  local ctx="kind-${KIND_CLUSTER_NAME}"
  local pair= name= version= retries= reported=

  echo "Creating module version override file (${CONFIG_DIR}/module-overrides.yml)..."
  : >${CONFIG_DIR}/module-overrides.yml

  for pair in ${D8_MODULE_VERSIONS//,/ }; do
    name="${pair%%[=:]*}"
    version="${pair#*[=:]}"
    if [[ "$name" == "$pair" || -z "$name" || -z "$version" ]]; then
      echo "Cannot parse '${pair}' as a module version. Use MODULE=VERSION pairs, e.g. console=v1.60.1."
      usage
      exit 1
    fi

    cat <<EOF >>${CONFIG_DIR}/module-overrides.yml
---
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: $name
spec:
  imageTag: $version
  scanInterval: 60s
EOF
  done

  print_dry_run_file "${CONFIG_DIR}/module-overrides.yml"

  run_cmd ${KUBECTL_PATH} --context "$ctx" apply -f "${CONFIG_DIR}/module-overrides.yml"

  if [ "$?" -ne "0" ]; then
    echo "Error applying the module version overrides!"
    cluster_deletion_info
    exit 1
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    return 0
  fi

  # A module switches version by redeploying, so waiting on the version it reports is what tells us
  # the override was actually picked up rather than just accepted by the API.
  for pair in ${D8_MODULE_VERSIONS//,/ }; do
    name="${pair%%[=:]*}"
    version="${pair#*[=:]}"
    retries=0
    reported=false

    echo -n "Waiting for the ${name} module to report ${version}..."
    while true; do
      if [[ "$(${KUBECTL_PATH} --context "$ctx" get module "$name" -o jsonpath='{.properties.version}' 2>/dev/null)" == "$version" ]]; then
        reported=true
        break
      fi
      if [[ "$retries" -ge 30 ]]; then
        break
      fi
      echo -n "."
      retries=$((retries + 1))
      sleep 10
    done
    echo

    if [[ "$reported" == "true" ]]; then
      echo "The ${name} module is pinned to ${version}."
    else
      echo "Timeout waiting for the ${name} module to switch to ${version}. The installation continues;"
      echo "check '${KUBECTL_PATH} --context ${ctx} get modulepulloverride ${name} -o yaml' for the reason"
      echo "(a tag that does not exist in the registry is the usual one)."
    fi
  done
}

# Waits until the cluster has stopped moving, so that the credentials are not printed while the web
# UIs are still answering 500.
#
# On a single control-plane node the installation ends with two consecutive kube-apiserver renders:
# control-plane-manager writes its own manifest when it takes over, and once the user-authn module
# has published the ClusterIP of its Dex Service, it renders the manifest again to add a hostAliases
# entry for the Dex domain (that is the only difference between the two - see the diffs the module
# keeps under /etc/kubernetes/deckhouse/diffs). The second render recreates the kube-apiserver Pod,
# and while the API is gone the kube-rbac-proxy of Dex cannot post TokenReviews, so Dex goes NotReady
# and the console and Grafana DexAuthenticators are rolled at the same time (their config checksum
# changes). Their Ingresses authenticate every request through an nginx auth subrequest, which has no
# ready endpoint to talk to at that moment - and nginx answers 500 rather than degrading gracefully.
# Nothing here is broken, but a check run right after the script printed its credentials used to land
# exactly in that window. So instead of trying to prevent the restart (the hostAliases value cannot
# exist before the Dex Service does), wait for the whole thing to converge: no control plane
# operation in flight, every Pod ready, and the web UIs actually answering - and require all of it to
# hold for a minute, since a new render can appear right after the previous one completed.
cluster_settled_wait() {
  local ctx="kind-${KIND_CLUSTER_NAME}"
  local scheme=http
  local retries_max=80
  local retries_count=0
  local stable_count=0
  local pending_ops= unready_pods= hosts= host= code= http_ok= ingress_ready=
  # The minute-long window exists to catch the second kube-apiserver render described above, which
  # only happens when control-plane-manager runs. Without it there is nothing that can restart the
  # control plane behind our back, so half the window is enough.
  local stable_needed=2
  if [[ "$D8_HTTPS_MODE" != "Disabled" ]]; then
    stable_needed=4
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "+ (would poll: control plane operations, Pod readiness, the Ingress controller and the web UI"
    echo "   endpoints, until all of them stay quiet for $((stable_needed * 15)) seconds or timeout)"
    return 0
  fi

  if [[ "$D8_HTTPS_MODE" == "SelfSigned" ]]; then
    scheme=https
  fi

  echo -n "Waiting for the cluster to settle..."

  while true; do
    # Operations that have not reached a terminal state, read from the reason of their Completed
    # condition - the same value the PHASE column shows. OperationCompleted is the successful end;
    # OperationAbandoned means control-plane-manager superseded that operation with a newer one
    # (its message reads "config checksum changed") and left it behind forever, so counting it as
    # in flight would make this wait time out on every cluster where a render was superseded - which
    # is exactly what happens when the Dex hostAliases entry appears. The resource only exists once
    # control-plane-manager is enabled, so a failing query counts as nothing in flight.
    pending_ops=$(${KUBECTL_PATH} --context "$ctx" get controlplaneoperation -A \
      -o jsonpath='{range .items[*]}{range .status.conditions[?(@.type=="Completed")]}{.reason}{"\n"}{end}{end}' 2>/dev/null |
      grep -c -v -E '^(OperationCompleted|OperationAbandoned)$')

    # A Pod counts as unready when it is neither running with all of its containers ready nor done.
    unready_pods=$(${KUBECTL_PATH} --context "$ctx" get pods -A --no-headers 2>/dev/null |
      awk '{split($3, r, "/"); if ($4 == "Completed" || $4 == "Succeeded") next; if ($4 != "Running" || r[1] != r[2]) c++} END {print c + 0}')

    # With https Disabled the modules get no Ingress at all, so there is nothing to probe.
    http_ok=true
    if [[ "$D8_HTTPS_MODE" != "Disabled" ]]; then
      hosts=$(${KUBECTL_PATH} --context "$ctx" get ingress -A \
        -o jsonpath='{range .items[*]}{range .spec.rules[*]}{.host}{"\n"}{end}{end}' 2>/dev/null |
        sort -u | grep -E '^(console|grafana)\.' || true)
      for host in $hosts; do
        code=$(curl -sk -o /dev/null -w '%{http_code}' --max-time 15 "${scheme}://${host}$(url_port_suffix "$scheme")/" 2>/dev/null)
        # Anything below 400 means the Ingress and the Dex authenticator in front of it are working:
        # an unauthenticated request is answered with a redirect to the Dex login page. curl reports
        # a connection that never got an answer as 000, which is exactly the state to keep waiting on.
        if [[ -z "$code" || "$code" == "000" || "$code" -ge 400 ]]; then
          http_ok=false
          break
        fi
      done
    fi

    # Counting unready Pods says nothing about a Pod that does not exist yet, so the Ingress
    # controller is checked explicitly - it is the last thing the modules bring up, and the web UIs
    # are unreachable until it runs. Its AdvancedDaemonSet is managed by the kruise controller, hence
    # "ads" rather than a plain DaemonSet.
    ingress_ready=$(${KUBECTL_PATH} --context "$ctx" -n d8-ingress-nginx get ads/controller-nginx \
      -o jsonpath='{.status.numberReady}' 2>/dev/null | tr -d '\r')

    if [[ "$pending_ops" -eq 0 && "$unready_pods" -eq 0 && "$http_ok" == "true" && "$ingress_ready" == "1" ]]; then
      stable_count=$((stable_count + 1))
      if [[ "$stable_count" -ge "$stable_needed" ]]; then
        break
      fi
    else
      stable_count=0
    fi

    if [[ "$retries_count" -ge "$retries_max" ]]; then
      echo
      echo "The cluster is still changing after $((retries_max * 15 / 60)) minutes: ${pending_ops} control plane"
      echo "operation(s) in flight, ${unready_pods} Pod(s) not ready, Ingress controller replicas ready:"
      echo "${ingress_ready:-0}, web UIs reachable: ${http_ok}."
      echo "The installation is complete either way - but a web UI may answer 500 for another minute"
      echo "while the Dex authenticators come back up."
      return 0
    fi

    echo -n "."
    retries_count=$((retries_count + 1))
    sleep 15
  done

  echo
  echo "The cluster has settled: no control plane operations in flight and every Pod is ready."
}

installation_finish() {
  printf "
You have installed Deckhouse Kubernetes Platform in kind!

Don't forget that the default kubectl context has been changed to 'kind-${KIND_CLUSTER_NAME}'.

Run '%s --context %s cluster-info' to see cluster info.
Run '%s delete cluster --name %s' to delete the cluster.
" "${KUBECTL_PATH}" "kind-${KIND_CLUSTER_NAME}" "${KIND_PATH}" "${KIND_CLUSTER_NAME}" >${CONFIG_DIR}/info.txt

  install_show_credentials >>${CONFIG_DIR}/info.txt

  if [[ -n "$D8_MODULE_VERSIONS" ]]; then
    printf "
Module versions pinned with ModulePullOverride (these modules no longer follow the release channel):

    %s

Let a module follow its channel again with '%s --context %s delete modulepulloverride <MODULE>'.
" "${D8_MODULE_VERSIONS}" "${KUBECTL_PATH}" "kind-${KIND_CLUSTER_NAME}" >>${CONFIG_DIR}/info.txt
  fi

  cat ${CONFIG_DIR}/info.txt
  printf "The information above is saved to %s file.

Good luck!
" "${CONFIG_DIR}/info.txt"
}

main() {

  parse_args "$@"
  registry_path_default

  os_detect
  prerequisites_check
  ports_check
  configs_create
  cluster_create
  loop_devices_drop
  default_storage_drop
  arch_workaround_apply
  api_proxy_install
  deckhouse_install
  module_versions_apply
  kubelet_serving_cert_fix
  control_plane_manager_enable
  cluster_settled_wait
  installation_finish
}

main "$@"
