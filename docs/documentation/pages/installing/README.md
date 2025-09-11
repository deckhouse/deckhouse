---
title: "Installation"
permalink: en/installing/
description: |
  Information on installing the Deckhouse Kubernetes Platform, including infrastructure preparation, configuration, and installer run.
---

{% alert level="warning" %}
This page is under active development and may contain incomplete information. Below is an overview of the Deckhouse installation process. For more detailed instructions, we recommend visiting the [Getting Started](/products/kubernetes-platform/gs/) section, where step-by-step guides are available.
{% endalert %}

The Deckhouse installer is available as a container image and is based on the [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>) utility, which is responsible for:

* Creating and configuring cloud infrastructure objects using Terraform;
* Installing necessary OS packages on nodes (including Kubernetes packages);
* Installing Deckhouse;
* Creating and configuring nodes for the Kubernetes cluster;
* Maintaining the cluster state according to the defined configuration.

Deckhouse installation options:

* **In a supported cloud.** The `dhctl` utility automatically creates and configures all necessary resources, including virtual machines, deploys the Kubernetes cluster, and installs Deckhouse. A full list of supported cloud providers is available in the [Kubernetes Cluster](../kubernetes.html) section.

* **On bare-metal servers or in unsupported clouds.** In this option, `dhctl` configures the server or virtual machine, deploys the Kubernetes cluster with a single master node, and installs Deckhouse. Additional nodes can be added to the cluster using pre-existing setup scripts.

* **In an existing Kubernetes cluster.** If a Kubernetes cluster is already deployed, `dhctl` installs Deckhouse and integrates it with the existing infrastructure.

## Preparing the Infrastructure

Before installation, ensure the following:

* **For bare-metal clusters and unsupported clouds**: The server is running an operating system from the [supported OS list](../supported_versions.html) or a compatible version, and it is accessible via SSH using a key.

* **For supported clouds**: Ensure that necessary quotas are available for resource creation and that access credentials to the cloud infrastructure are prepared (these depend on the specific provider).

* **For all installation options**: Access to the container registry with Deckhouse images (`registry.deckhouse.io` or `registry.deckhouse.ru`) is configured.

## Preparing the Configuration

Before starting the Deckhouse installation, you need to prepare the [configuration YAML file](#installation-config). This file contains the main parameters for configuring Deckhouse, including information about cluster components, network settings, and integrations, as well as a description of resources to be created after installation (node settings and Ingress controller).

Make sure that the configuration files meet the requirements of your infrastructure and include all the necessary parameters for a correct deployment.

### Installation config

The installation configuration YAML file contains parameters for several resources (manifests):

1. [InitConfiguration](configuration.html#initconfiguration) — initial parameters for [Deckhouse configuration](../#deckhouse-configuration), necessary for the proper startup of Deckhouse after installation.

   Key settings specified in this resource:
   * [Component placement parameters](../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys);
   * The [StorageClass](../deckhouse-configure-global.html#parameters-storageclass) (storage parameters);
   * Access parameters for the [container registry](configuration.html#initconfiguration-deckhouse-registrydockercfg);
   * Template for [DNS names](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate);
   * Other essential parameters required for Deckhouse to function correctly.

1. [ClusterConfiguration](configuration.html#clusterconfiguration) — general cluster parameters, such as control plane version, network settings, CRI parameters, etc.
    > This resource is needed only when Deckhouse is being installed with a pre-deployed Kubernetes cluster. If Deckhouse is being installed in an already existing cluster, this resource is not required.

1. [StaticClusterConfiguration](configuration.html#staticclusterconfiguration) — parameters for Kubernetes clusters deployed on bare-metal servers or virtual machines in unsupported clouds.
   > This resource is needed only when Deckhouse is being installed with a pre-deployed Kubernetes cluster. If Deckhouse is being installed in an already existing cluster, this resource is not required.

1. `<CLOUD_PROVIDER>ClusterConfiguration` — a set of resources containing configuration parameters for supported cloud providers. These include:
   * Cloud infrastructure access settings (authentication parameters);
   * Resource placement scheme type and parameters;
   * Network settings;
   * Node group creation settings.

   List of cloud provider configuration resources:
   * [AWSClusterConfiguration](../modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) — Amazon Web Services;
   * [AzureClusterConfiguration](../modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) — Microsoft Azure;
   * [GCPClusterConfiguration](../modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) — Google Cloud Platform;
   * [OpenStackClusterConfiguration](../modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) — OpenStack;
   * [VsphereClusterConfiguration](../modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) — VMware vSphere;
   * [VCDClusterConfiguration](../modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration) — VMware Cloud Director;
   * [YandexClusterConfiguration](../modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — Yandex Cloud;
   * [ZvirtClusterConfiguration](../modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) — zVirt.

1. `ModuleConfig` — a set of resources containing configuration parameters for [Deckhouse built-in modules](../).

   If the cluster is initially created with nodes dedicated to specific types of workloads (e.g., system nodes or monitoring nodes), it is recommended to explicitly set the `nodeSelector` parameter in the configuration of modules that use persistent storage volumes.

   For example, for the `prometheus` module, the configuration is specified in the [nodeSelector](../modules/prometheus/configuration.html#parameters-nodeselector) parameter.

1. `IngressNginxController` — deploying the Ingress controller.

1. `NodeGroup` — creating additional node groups.

1. `InstanceClass` — adding configuration resources.

1. `ClusterAuthorizationRule`, `User` — setting up roles and users.

{% offtopic title="An example of the installation config..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Azure
  prefix: cloud-demo
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: <SSH_PUBLIC_KEY>
vNetCIDR: 10.241.0.0/16
subnetCIDR: 10.241.0.0/24
masterNodeGroup:
  replicas: 3
  instanceClass:
    machineSize: Standard_D4ds_v4
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    enableExternalIP: true
provider:
  subscriptionId: <SUBSCRIPTION_ID>
  clientId: <CLIENT_ID>
  clientSecret: <CLIENT_SECRET>
  tenantId: <TENANT_ID>
  location: westeurope
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-flannel
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    releaseChannel: Stable
    bundle: Default
    logLevel: Info
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true
  settings:
    modules:
      publicDomainTemplate: "%s.k8s.example.com"
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-manager
spec:
  version: 1
  enabled: true
  settings:
    earlyOomEnabled: false
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  # Specify, in case of using dedicated nodes for monitoring.
  # settings:
  #   nodeSelector:
  #     node.deckhouse.io/group: monitoring
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  controllerVersion: "1.1"
  inlet: "LoadBalancer"
  nodeSelector:
    node.deckhouse.io/group: worker
---
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: worker
spec:
  machineSize: Standard_F4
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AzureInstanceClass
      name: worker
    maxPerZone: 3
    minPerZone: 1
    zones: ["1"]
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@deckhouse.io
  accessLevel: SuperAdmin
  portForwarding: true
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@deckhouse.io
  password: '$2a$10$isZrV6uzS6F7eGfaNB1EteLTWky7qxJZfbogRs1egWEPuT1XaOGg2'
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse-admin
spec:
  enabled: true
```

{% endofftopic %}

### Post-bootstrap script

After Deckhouse installation is complete, the installer offers the option to run a custom script on one of the master nodes. This script can be used for:

* Performing additional cluster configurations;
* Collecting diagnostic information;
* Integrating with external systems or other tasks.

The path to the post-bootstrap script can be specified using the `--post-bootstrap-script-path` parameter during the installation process.

{% offtopic title="Example: a script that retrieves the IP address of the load balancer..." %}
This sample script retrieves the IP address of the load balancer after the cluster is deployed in the cloud and Deckhouse is installed:

```shell
#!/usr/bin/env bash

set -e
set -o pipefail


INGRESS_NAME="nginx"


echo_err() { echo "$@" 1>&2; }

# declare the variable
lb_ip=""

# get the load balancer IP
for i in {0..100}
do
  if lb_ip="$(kubectl -n d8-ingress-nginx get svc "${INGRESS_NAME}-load-balancer" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"; then
    if [ -n "$lb_ip" ]; then
      break
    fi
  fi

  lb_ip=""

  sleep 5
done

if [ -n "$lb_ip" ]; then
  echo_err "The load balancer external IP: $lb_ip"
else
  echo_err "Could not get the external IP of the load balancer"
  exit 1
fi

outContent="{\"frontend_ips\":[\"$lb_ip\"]}"

if [ -z "$OUTPUT" ]; then
  echo_err "The OUTPUT env is empty. The result was not saved to the output file."
else
  echo "$outContent" > "$OUTPUT"
fi
```

{% endofftopic %}

## Installing Deckhouse

{% alert level="info" %}
When installing a commercial edition of Deckhouse Kubernetes Platform from the official container registry `registry.deckhouse.io`, you must first log in with your license key:

```shell
docker login -u license-token registry.deckhouse.io
```

{% endalert %}

The command to pull the installer container from the Deckhouse public registry and run it looks as follows:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

Where:

1. `<DECKHOUSE_REVISION>` — the [Deckhouse edition](../revision-comparison.html), such as `ee` for Enterprise Edition, `ce` for Community Edition, etc.
1. `<MOUNT_OPTIONS>` — parameters for mounting files into the installer container, such as:
   - SSH access keys;
   - Configuration file;
   - Resource file, etc.
1. `<RELEASE_CHANNEL>` — the [release channel](../modules/deckhouse/configuration.html#parameters-releasechannel) in kebab-case format:
   - `alpha` — for the Alpha release channel;
   - `beta` — for the Beta release channel;
   - `early-access` — for the Early Access release channel;
   - `stable` — for the Stable release channel;
   - `rock-solid` — for the Rock Solid release channel.

Here is an example of a command to run the installer container for Deckhouse CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

Deckhouse installation is performed within the installer container using the `dhctl` utility:

* To start the installation of Deckhouse with the deployment of a new cluster (for all cases except installing into an existing cluster), use the command `dhctl bootstrap`.
* To install Deckhouse into an already existing cluster, use the command `dhctl bootstrap-phase install-deckhouse`.

{% alert level="info" %}
Run `dhctl bootstrap -h` to learn more about the parameters available.
{% endalert %}

Example of running the Deckhouse installation with cloud cluster deployment:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
```

Where:

- `/config.yml` — the installation configuration file;
- `<SSH_USER>` — the username for SSH connection to the server;
- `--ssh-agent-private-keys` — the private SSH key file for SSH connection.

### Pre-Installation Checks

{% offtopic title="Diagram of pre-installation checks execution..." %}
![Diagram of pre-installation checks execution](../images/installing/preflight-checks.png)
{% endofftopic %}

List of checks performed by the installer before starting Deckhouse installation:

1. General checks:
   - The values of the parameters [PublicDomainTemplate](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) and [clusterDomain](configuration.html#clusterconfiguration-clusterdomain) do not match.
   - The authentication data for the container image registry specified in the installation configuration is correct.
   - The host name meets the following requirements:
     - The length does not exceed 63 characters;
     - It consists only of lowercase letters;
     - It does not contain special characters (hyphens `-` and periods `.` are allowed, but they cannot be at the beginning or end of the name).
   - The server (VM) has a supported container runtime (`containerd`) installed.
   - The host name is unique within the cluster.
   - The server's system time is correct.
   - The address spaces for Pods (`podSubnetCIDR`) and services (`serviceSubnetCIDR`) do not intersect.


1. Checks for static and hybrid cluster installation:
   - Only one `--ssh-host` parameter is specified. For static cluster configuration, only one IP address can be provided for configuring the first master node.
   - SSH connection is possible using the specified authentication data.
   - SSH tunneling to the master node server (or VM) is possible.
   - The server (VM) selected for the master node installation must meet the [minimum system requirements](/products/kubernetes-platform/guides/hardware-requirements.html):
     - at least 4 CPU cores;
     - at least 8 GB of RAM;
     - at least 60 GB of disk space with 400+ IOPS performance;
     - Linux kernel version 5.8 or newer;
     - one of the package managers installed: `apt`, `apt-get`, `yum`, or `rpm`;
     - access to standard OS package repositories.
   - Python is installed on the master node server (VM).
   - The container image registry is accessible through a proxy (if proxy settings are specified in the installation configuration).
   - Required installation ports are free on the master node server (VM) and the installer host.
   - DNS must resolve `localhost` to IP address 127.0.0.1.
   - The user has `sudo` privileges on the server (VM).
   - Required ports for the installation must be open:
     - port 22/TCP between the host running the installer and the server;
     - no port conflicts with those used by the installation process.
   - The server (VM) has the correct time.
   - The user `deckhouse` must not exist on the server (VM).
   - The address spaces for Pods (`podSubnetCIDR`) services (`serviceSubnetCIDR`) and internal network (`internalNetworkCIDRs`) do not intersect.

1. Checks for cloud cluster installation:
   - The configuration of the virtual machine for the master node meets the minimum requirements.
   - The cloud provider API is accessible from the cluster nodes.
   - For Yandex Cloud deployments with NAT Instance, the configuration for Yandex Cloud with NAT Instance is verified.

{% offtopic title="List of preflight skip flags..." %}

- `--preflight-skip-all-checks` — skip all preflight checks.
- `--preflight-skip-ssh-forward-check` — skip the SSH forwarding check.
- `--preflight-skip-availability-ports-check` — skip the check for the availability of required ports.
- `--preflight-skip-resolving-localhost-check` — skip the `localhost` resolution check.
- `--preflight-skip-deckhouse-version-check` — skip the Deckhouse version check.
- `--preflight-skip-registry-through-proxy` — skip the check for access to the registry through a proxy server.
- `--preflight-skip-public-domain-template-check` — skip the check for the `publicDomain` template.
- `--preflight-skip-ssh-credentials-check` — skip the check for SSH user credentials.
- `--preflight-skip-registry-credential` — skip the check for registry access credentials.
- `--preflight-skip-containerd-exist` — skip the check for the existence of `containerd`.
- `--preflight-skip-python-checks` — skip the check for Python installation.
- `--preflight-skip-sudo-allowed` — skip the check for `sudo` privileges.
- `--preflight-skip-system-requirements-check` — skip the system requirements check.
- `--preflight-skip-one-ssh-host` — skip the check for the number of specified SSH hosts.
- `--preflight-cloud-api-accesibility-check` — skip the Cloud API accessibility check.
- `--preflight-time-drift-check` — skip the time drift check.
- `--preflight-skip-cidr-intersection` — skip the CIDR intersection check.
- `--preflight-skip-deckhouse-user-check` — skip deckhouse user existence check.
- `--preflight-skip-yandex-with-nat-instance-check` — skip the Yandex Cloud with NAT Instance configuration check.

Example of using the preflight skip flag:

  ```shell
      dhctl bootstrap \
      --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
      --config=/config.yml \
      --preflight-skip-all-checks
  ```

{% endofftopic %}

### Aborting the installation

If the installation was interrupted or issues occurred during the installation process in a supported cloud, there might be leftover resources created during the installation. To remove them, use the `dhctl bootstrap-phase abort` command within the installer container.

{% alert level="warning" %}
The configuration file provided through the `--config` parameter when running the installer must be the same one used during the initial installation.
{% endalert %}
