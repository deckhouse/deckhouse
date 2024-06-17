---
title: "Installation"
permalink: en/installing/
---

<div class="docs__information warning active">
This page is under active development and may contain incomplete information. It provides an overview of the steps required to install Deckhouse. Please refer to the [Getting Started guide](/gs/) section for detailed step-by-step instructions.
</div>

The Deckhouse installer is available as a container image. It is based on the [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>) tool which is responsible for:
* Creating and configuring objects in the cloud infrastructure using Terraform;
* Installing the required OS packages on the nodes (including Kubernetes packages);
* Installing Deckhouse;
* Creating and configuring Kubernetes cluster nodes;
* Keeping the cluster in (or bringing it to) the state described in the configuration.

Deckhouse installation options:
- **Supported cloud:** In this case, dhctl creates and configures all the required resources (including virtual machines), deploys the Kubernetes cluster and installs Deckhouse. For information on supported cloud providers, see the [Kubernetes Cluster](../kubernetes.html) section.
- **Bare metal cluster or unsupported cloud:** In this case, dhctl configures the server (virtual machine), deploys a Kubernetes cluster with a single master node and installs Deckhouse. You can then manually add more nodes to the cluster using the pre-made configuration scripts.
- **Existing Kubernetes cluster:** In this case, dhctl installs Deckhouse to the existing cluster.

## Preparing the infrastructure

Before installing, ensure that:
- *(for bare metal clusters and clusters in unsupported clouds)* the server's OS is in the [list of supported OS](../supported_versions.html) (or compatible with them) and SSH access to the server with key-based authentication is configured;
- *(for supported clouds)* you have the quotas needed to create resources as well as cloud access credentials (the exact set depends on the specific cloud infrastructure or cloud provider);
- you have access to the container registry with Deckhouse images (default is `registry.deckhouse.io`).

## Preparing the configuration

To install Deckhouse, you have to create a YAML file containing the installation configuration and, if necessary, a YAML config for the resources that must be created after a successful Deckhouse installation.

### Installation config

The YAML installation config contains multiple resource configurations (manifests):
- [InitConfiguration](configuration.html#initconfiguration) — the initial [Deckhouse configuration](../#deckhouse-configuration). Deckhouse will use it to start after the installation.

  This resource contains the parameters Deckhouse needs to start or run smoothly, such as the [placement-related parameters for Deckhouse components](../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys), the [storageClass](../deckhouse-configure-global.html#parameters-storageclass) used, the [container registry](configuration.html#initconfiguration-deckhouse-registrydockercfg) credentials, the [DNS naming template](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate), and more.

  **Caution!** The [configOverrides](configuration.html#initconfiguration-deckhouse-configoverrides) parameter is deprecated and will be removed. Use the [ModuleConfig](../#configuring-the-module) in the installation config to set parameters for **built-in** Deckhouse modules. For other modules, use [installation resource file](#installation-resource-config).  

- [ClusterConfiguration](configuration.html#clusterconfiguration) — general cluster parameters, such as network parameters, CRI parameters, control plane version, etc.
  
  > The `ClusterConfiguration` resource is only required if a Kubernetes cluster has to be pre-deployed when installing Deckhouse. That is, `ClusterConfiguration` is not required if Deckhouse is installed into an existing Kubernetes cluster.

- [StaticClusterConfiguration](configuration.html#staticclusterconfiguration) — parameters of a Kubernetes cluster deployed to bare metal servers or virtual machines in an unsupported clouds.

  > As with the `ClusterConfiguration` resource, the `StaticClusterConfiguration` resource is not required if Deckhouse is installed into an existing Kubernetes cluster.  

- `<CLOUD_PROVIDER>ClusterConfiguration` — a set of resources with the configuration parameters for the supported cloud providers.
  
  These resources contain parameters required to access the cloud infrastructure (authentication credentials), resource layout type and configuration, network settings, parameters of node groups to be created, etc.

  Below is the list of configuration resources for supported cloud providers:
  - [AWSClusterConfiguration](../modules/030-cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) — Amazon Web Services
  - [AzureClusterConfiguration](../modules/030-cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) — Microsoft Azure
  - [GCPClusterConfiguration](../modules/030-cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) — Google Cloud Platform
  - [OpenStackClusterConfiguration](../modules/030-cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) — OpenStack
  - [VsphereInstanceClass](../modules/030-cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) — VMware vSphere
  - [YandexInstanceClass](../modules/030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — Yandex Cloud

- `ModuleConfig` — a set of resources containing [Deckhouse configuration](../) parameters.

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
kind: InitConfiguration
deckhouse:
  releaseChannel: Stable
  configOverrides:
    global:
      modules:
        publicDomainTemplate: "%s.example.com"
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
    allowedBundles: ["ubuntu-lts"]
```

{% endofftopic %}

### Installation resource config

The optional YAML installation resource config contains the Kubernetes resource manifests that will be applied after a successful Deckhouse installation.

This file can help you with the additional cluster configuration once Deckhouse is installed: deploying the Ingress controller, creating additional node groups and configuration resources, assigning permissions and managing users, etc.

**Caution!** You cannot use [ModuleConfig](../) for **built-in** modules in the installation resource file. To configure built-in modules, use [configuration file](#installation-config).

{% offtopic title="An example of the resource config... " %}

```yaml
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

After successfully installing Deckhouse, the installer provides an option to run the script on one of the master nodes. This script can be used for additional customization, collecting configuration information, etc.

To take advantage of this feature, create the script and specify the path to it using the `--post-bootstrap-script-path` flag when when you start the installation (see below).

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

> When you install Deckhouse Enterprise Edition from the official `registry.deckhouse.io` container registry, you must first log in with your license key:
>
> ```shell
> docker login -u license-token registry.deckhouse.io
> ```

The command to pull the installer container from the Deckhouse public registry and run it looks as follows:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

, where:
- `<DECKHOUSE_REVISION>` — [edition](../revision-comparison.html) of Deckhouse (e.g., `ee` for Enterprise Edition, `ce` for Community Edition, etc.);
- `<MOUNT_OPTIONS>` — options for mounting files in the installer container, such as:
  - SSH authentication keys;
  - config file;
  - resource file, etc.
- `<RELEASE_CHANNEL>` — Deckhouse [release channel](../modules/002-deckhouse/configuration.html#parameters-releasechannel) in kebab-case. Should match with the option set in `config.yml`:
  - `alpha` — for the *Alpha* release channel;
  - `beta` — for the *Beta* release channel;
  - `early-access` — for the *Early Access* release channel;
  - `stable` — for the *Stable* release channel;
  - `rock-solid` — for the *Rock Solid* release channel.

Here is an example of a command to run the installer container for Deckhouse CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/resources.yml:/resources.yml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

The installation of Deckhouse in the installer container can be started using the `dhctl` command:
- Use the `dhctl bootstrap` command, to start a Deckhouse installation including cluster deployment (these are all cases, except for installation Deckhouse in an existing cluster);
- Use the `dhctl bootstrap-phase install-deckhouse` command, to start a Deckhouse installation in an existing cluster;

> Run `dhctl bootstrap -h` to learn more about the parameters available.

This command will start the Deckhouse installation in a cloud:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml --resources=/resources.yml
```

, where:
- `/config.yml` — installation config;
- `/resources.yml` — file with the resource manifests;
- `<SSH_USER>` — SSH user on the server;
- `--ssh-agent-private-keys` — file with the private SSH key for connecting via SSH.

### Aborting the installation and uninstalling Deckhouse

When installing in a supported cloud, the resources created may remain in the cloud if the installation is interrupted or there are problems during the installation. Use the `dhctl bootstrap-phase abort` command to delete those resources. Note that the configuration file passed via the `--config` flag must be the same as the one used for the installation.

To delete a cluster running in a supported cloud and deployed after the Deckhouse installation, use the `dhctl destroy` command. In this case, `dhctl` will connect to the master node, retrieve the terraform state, and delete the resources created in the cloud in the correct fashion.
