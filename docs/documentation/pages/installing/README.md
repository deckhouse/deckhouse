---
title: "Installation"
permalink: en/installing/
description: |
  Installing Deckhouse Kubernetes Platform (DKP), preparing the installation infrastructure, and running the installer.
search: requirements, system requirements, platform setup, infrastructure preparation, installer configuration, deckhouse setup, platform configuration, infrastructure preparation, installer configuration, installer setup, dhctl, dhctl bootstrap
extractedLinksMax: 2
relatedLinks:
  - title: "Getting started"
    url: /products/kubernetes-platform/gs/
  - title: "Supported Kubernetes and OS versions"
    url: ../reference/supported_versions.html
  - title: "Integration with IaaS providers"
    url: ../admin/integrations/integrations-overview.html
  - title: "Installing DKP in a private environment"
    url: /products/kubernetes-platform/guides/private-environment.html
  - title: "Going to Production"
    url: /products/kubernetes-platform/guides/production.html 
---

{% alert %}
Step-by-step installation instructions are available in the {% if site.mode == 'module' %}[Getting started]({{ site.urls[page.lang] }}/products/kubernetes-platform/gs/){% else %}[Getting started](/products/kubernetes-platform/gs/){% endif %} section.
{% endalert %}

This page provides an overview of installing Deckhouse Kubernetes Platform (DKP).

## Installation methods

You can install DKP using a CLI installer, which is available as a container image and based on the [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>) utility.

## Installation options

You can install DKP in the following ways::

- **In a supported cloud.** The installer automatically creates and configures all required resources (including virtual machines, network objects, etc.), deploys a Kubernetes cluster, and installs DKP. A full list of supported cloud providers is available in the [Integration with IaaS](../admin/integrations/public/overview.html) section.

- **On bare-metal servers (including hybrid clusters) or in unsupported clouds.** The installer configures the servers or virtual machines specified in the configuration, deploys a Kubernetes cluster, and installs DKP. Step-by-step instructions for bare metal are available in [Getting started → Deckhouse Kubernetes Platform for bare metal]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm/step2.html).

- **In an existing Kubernetes cluster.** The installer deploys DKP and integrates it with the current infrastructure. Step-by-step instructions for an existing cluster are available in [Getting started → Deckhouse Kubernetes Platform in existing cluster]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/existing/step2.html).

## Installation requirements

To estimate the resources required for Deckhouse Kubernetes Platform installation, refer to the following guides:

- [Bare-metal cluster sizing guide](/products/kubernetes-platform/guides/hardware-requirements.html)
- [Disk layout and sizing guide](/products/kubernetes-platform/guides/fs-requirements.html)
- [Production preparation guide](/products/kubernetes-platform/guides/production.html)

Before installation, ensure the following:

- For bare-metal clusters (including hybrid clusters) and installations in unsupported clouds: the server runs an OS from the [supported OS list](../reference/supported_versions.html) (or a compatible version) and is accessible via SSH with a key.

- For supported clouds: the required resource quotas are available and access credentials to the cloud infrastructure are prepared (provider-specific).

- If there are infrastructure-level restrictions on network communication, ensure that the requirements described in the [Network interaction of the platform components](../reference/network_interaction.html) section are met.
  
- There is access to the Deckhouse container registry (official `registry.deckhouse.io`, or a mirror).

## Preparing the Configuration

Before installation, you need to prepare the [installation configuration file](#installation-configuration-file) and, if needed, a [post-bootstrap script](#post-bootstrap-script).

### Installation configuration file

The installation configuration file is a set of YAML documents that contains DKP settings and manifests for cluster objects and resources to be created after installation. The configuration file is used by the CLI installer and is passed via the `--config` parameter (see below).

Required and optional objects/resources that may be needed in the installation configuration file:

1. [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) (**required**): Initial [configuration parameters](../admin/configuration/) necessary to start DKP.

   > Starting with DKP 1.75, use the ModuleConfig `deckhouse` to configure access to the DKP container registry. Configuring access with InitConfiguration (via `imagesRepo`, `registryDockerCfg`, `registryScheme`, and `registryCA` parameters) is considered a legacy method.

1. [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration): General cluster parameters, such as Kubernetes (control plane components) version, network settings, CRI parameters, etc. **Required**, except when DKP is installed into an already existing Kubernetes cluster.

1. [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration): Parameters for a cluster deployed on bare-metal servers (including hybrid clusters) or in unsupported clouds. **Required**, except when DKP is installed into an already existing Kubernetes cluster.

   To add worker node groups (the [NodeGroup](/modules/node-manager/cr.html#nodegroup) object), you may also need [StaticInstance](/modules/node-manager/cr.html#staticinstance) and [SSHCredentials](/modules/node-manager/cr.html#sshcredentials).

1. &lt;PROVIDER&gt;ClusterConfiguration: Parameters for integration with a cloud provider. **Required** when integrating DKP with a [supported cloud infrastructure](../admin/integrations/public/overview.html).

   Examples of resources configuring integration with a cloud provider:

   * [AWSClusterConfiguration](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration): Amazon Web Services
   * [AzureClusterConfiguration](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration): Microsoft Azure
   * [DVPClusterConfiguration](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration): Deckhouse Virtualization Platform
   * [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration): Google Cloud Platform
   * [HuaweiCloudClusterConfiguration](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration): Huawei Cloud
   * [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration): OpenStack, OVHcloud, Selectel, VK Cloud
   * [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration): VMware vSphere
   * [VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration): VMware Cloud Director
   * [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration): Yandex Cloud
   * [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration): zVirt

   To add cloud nodes, you also need &lt;PROVIDER&gt;InstanceClass objects (for example [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) for Yandex Cloud) that describe VM configuration in the node group (the [NodeGroup](/modules/node-manager/cr.html#nodegroup) object).

1. DKP module configurations.

   Each module is configured (and can be enabled or disabled) with its own [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) (for example, ModuleConfig `user-authn` for the [`user-authn`](/modules/user-authn/) module). Parameters that are allowed in the ModuleConfig object are described in the respective module documentation under "Configuration" (for example, [configuration of the `user-authn` module](/modules/user-authn/configuration.html)).

   A list of all Deckhouse Kubernetes Platform modules is available in the [Modules](/modules/) section.

   Some modules may be enabled and preconfigured automatically depending on the selected installation option and cluster configuration (for example, modules that provide control plane and networking functionality).

   Modules often configured during installation:

   * [`global`](/products/kubernetes-platform/documentation/v1/reference/api/global.html): Global DKP settings for parameters used by default by all modules and components (DNS name template, StorageClass, module component placement settings, etc.).
   * [`deckhouse`](/modules/deckhouse/configuration.html): Container registry access settings, the desired release channel, and other parameters.
   * [`user-authn`](/modules/user-authn/configuration.html): Unified authentication.
   * [`cni-cilium`](/modules/cni-cilium/configuration.html): Cluster networking (for example, used when installing DKP on bare metal or in an air-gapped environment).

   If the cluster is created with nodes dedicated to specific workload types (for example, system or monitoring nodes), it is recommended to explicitly set the `nodeSelector` parameter in module configurations that use persistent storage volumes (for example, in the [`nodeSelector`](/modules/prometheus/configuration.html#parameters-nodeselector) parameter of the `prometheus` ModuleConfig for the `prometheus` module).

1. [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller): Parameters of the HTTP/HTTPS load balancer (Ingress controller).

1. [NodeGroup](/modules/node-manager/cr.html#nodegroup): Node group parameters. Required to add worker nodes.

1. Objects for authentication and authorization such as [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule), [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule), [User](/modules/user-authn/cr.html#user), [Group](/modules/user-authn/cr.html#group), and [DexProvider](/modules/user-authn/cr.html#dexprovider).

   See [authentication](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authentication/) and [authorization](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/) documentation for details.

{% offtopic title="An example of the installation config..." %}

{% tabs variant %}
{% tab "Configuration applicable since DKP 1.75" %}
In this example, access to the DKP container registry is configured using ModuleConfig `deckhouse`.

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
  name: deckhouse
spec:
  enabled: true
  settings:
    releaseChannel: Stable
    bundle: Default
    logLevel: Info
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: test-registry.io/some/path
        scheme: HTTPS
        username: <username>
        password: <password>
        ca: <CA>
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    modules:
      publicDomainTemplate: "%s.k8s.example.com"
  version: 2
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
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-manager
spec:
  version: 1
  enabled: true
  settings: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  # Specify if you plan to use dedicated nodes for monitoring.
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
```

{% endtab %}
{% tab "Legacy configuration" %}
In this example, access to the DKP container registry is configured using InitConfiguration.

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
  imagesRepo: registry.deckhouse.io/deckhouse/ee
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2zzzmRlY2tob3Vxxcxxxc5ydSI6IsssfX0K
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
  settings:
    modules:
      publicDomainTemplate: "%s.k8s.example.com"
  version: 2
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
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-manager
spec:
  version: 1
  enabled: true
  settings: {}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  # Specify if you plan to use dedicated nodes for monitoring.
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
```

{% endtab %}
{% endtabs %}

{% endofftopic %}

### Post-bootstrap script

The installer allows you to run a custom script on one of the master nodes after installation (post-bootstrap script). This script can be used for:

* Additional cluster configuration
* Collecting diagnostic information
* Integrating with external systems or other tasks

The path to the post-bootstrap script can be specified using the `--post-bootstrap-script-path` parameter when running the CLI installer.

{% offtopic title="Example: a script that retrieves the IP address of the load balancer..." %}
This sample script retrieves the IP address of the load balancer after DKP is installed:

```shell
#!/usr/bin/env bash

set -e
set -o pipefail


INGRESS_NAME="nginx"


echo_err() { echo "$@" 1>&2; }

# Declare the variable.
lb_ip=""

# Get the load balancer IP address.
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

## Installing

{% alert level="info" %}
When installing a commercial edition of Deckhouse Kubernetes Platform from the official container registry `registry.deckhouse.io`, you must first log in with your license key:

```shell
docker login -u license-token registry.deckhouse.io
```

{% endalert %}

The command to run the installer container from the public Deckhouse container registry:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

Where:

1. `<DECKHOUSE_REVISION>`: [DKP edition](../reference/revision-comparison.html). For example, `ee` for Enterprise Edition, `ce` for Community Edition, etc.
1. `<MOUNT_OPTIONS>`: Parameters for mounting files into the installer container, such as:
   - SSH access keys
   - Configuration file
   - Resource file, etc.
1. `<RELEASE_CHANNEL>`: [Release channel](/modules/deckhouse/configuration.html#parameters-releasechannel) in kebab-case format:
   - `alpha`: Alpha channel
   - `beta`: Beta channel
   - `early-access`: Early Access channel
   - `stable`: Stable channel
   - `rock-solid`: Rock Solid channel

Here is an example of a command to run the DKP Community Edition installer container from the Stable release channel:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

DKP installation is performed within the installer container using the `dhctl` command:

* To start the installation of DKP with the deployment of a new cluster (for all cases except installing into an existing cluster), use the command `dhctl bootstrap`.
* To install DKP into an already existing cluster, use the command `dhctl bootstrap-phase install-deckhouse`.

{% alert level="info" %}
To learn more about the available parameters, run `dhctl bootstrap -h`.
{% endalert %}

Example of running a DKP installation with cloud cluster deployment:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> \
  --config=/config.yml
```

Where:

- `/config.yml`: Installation configuration file.
- `<SSH_USER>`: Username for SSH connection to the server.
- `--ssh-agent-private-keys`: Private SSH key file for SSH connection.
- `<SSH_PRIVATE_KEY_FILE>`: Name of private key. For example, for a key with RSA encryption it can be `id_rsa`, and for a key with ED25519 encryption it can be `id_ed25519`.

### Installing a static cluster on machines with an immutable OS

A [NodeGroup](/modules/node-manager/cr.html#nodegroup) with `systemType: Immutable` describes machines running an immutable operating system: they have no SSH access and no bashible, so the installer neither creates nor configures them over SSH. The installation goes as follows:

1. You write the OS image to the disk of every machine and power the machines on. A machine that has no configuration yet waits for one on port `50000/TCP`.
1. The installer hands each machine its configuration over that port. Everything else — the disk layout, kubelet, the control plane components — the machine sets up itself from that document.
1. The machine named first starts the cluster. The rest are handed their configurations after DKP has been installed, and they join one at a time, because etcd accepts one new member at a time.

{% alert level="warning" %}
The provisioning network must be isolated. The endpoint on port `50000/TCP` is not authenticated: a machine that has no configuration holds no secret to authenticate with. Anyone who can reach that port can install the machine as a node of a foreign cluster. The inventory the machine serves on the same port is not authenticated either: anyone who can reach it reads the machine's disks, their serial numbers and its interfaces. Keep the machines in a separate network segment until the installation is over.
{% endalert %}

A machine that is waiting for its configuration describes itself on the same port, so you do not have to guess what it has:

```shell
curl http://<address>:50000/inventory        # for reading
curl http://<address>:50000/inventory.json   # for scripts
```

`/inventory` is the short form. Per disk it gives the kernel name, the size, the state — `blank`, `formatted` or `system-layout` — the link under `/dev/disk/by-path`, and the partitions with their filesystems and labels; disks that a size selector cannot tell apart get a warning line of their own. Under every disk it prints a line to paste into a `NodeConfig`: that line names the disk uniquely when anything on the machine distinguishes it, and says outright that nothing does when nothing does. `/inventory.json` carries the same plus everything the short form leaves out — model, vendor, serial, `wwid`, every `/dev/disk/by-id` link, the bus path, the transport and whether the disk rotates. Both forms list the interfaces with their MAC addresses, link state, addresses and gateway. The short form reads like this:

```
Disks:
  sda       30G  blank          pci-0000:0d:00.0-scsi-0:0:0:0
        diskSelector: {serial: "S3Z8NB0K700001"}
  sdb       30G  system-layout  pci-0000:0d:00.0-scsi-0:0:0:1
        diskSelector: {serial: "S3Z8NB0K700002"}
        sdb1         1G vfat   BOOT
        sdb2       256M ext4   CONFIG
        sdb3        28G ext4   DATA
  sdc       10G  blank          pci-0000:0d:00.0-scsi-0:0:0:2
        diskSelector: {serial: "S3Z8NB0K700003"}
  ! sda and sdb have the same size — a size selector cannot tell them apart
Interfaces:
  eth0   f2:4e:c6:60:03:72 up   192.168.199.11/24 gw 192.168.199.1 (dhcp)
```

Both endpoints answer only while the machine is waiting. The moment it accepts a configuration, the server that served them is shut down.

Besides [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) with `clusterType: Static` and [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration), the installation configuration file must contain:

- A NodeGroup named `master` with `nodeType: Static` and `systemType: Immutable`. This is what tells the installer that the machines are immutable.
- One `NodeConfig` document per machine, describing what the cluster cannot know about it.

Registry access has to be configured in the `Unmanaged` mode: an immutable node pulls the control plane images and the system extensions from the registry directly, without the in-cluster proxy that does not exist during installation.

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.199.0/24
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    releaseChannel: Stable
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: registry.deckhouse.io/deckhouse/ee
        scheme: HTTPS
        username: license-token
        password: <LICENSE_KEY>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
---
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  # A machine with a single disk, so spec.storage is left out; see the note below.
  network:
    interfaces:
    - name: eth0
      dhcp: false
      addresses:
      - 192.168.199.11/24
      gateway: 192.168.199.1
    dns:
      servers:
      - 192.168.199.1
  kubelet:
    nodeIP: 192.168.199.11
```

Every machine needs a document of its own: `master-1` and `master-2` differ from the one above only in the name and the addresses.

A `NodeConfig` document sets only the three things about a machine that the cluster cannot know:

- `spec.network`: the interfaces, the DNS servers and the static routes.
- `spec.storage`: which disk the OS is installed onto (`diskSelector` or `device`) and the extra mounts.
- `spec.kubelet.nodeIP`: the address the node registers with.

A machine that takes its address over DHCP needs no `spec.network` at all, whatever its NIC is called: the document the installer renders names `eth0` on DHCP, which is a guess and not knowledge about the machine, and the machine brings DHCP up on the interface it does have — so at most you see one warning that the guessed name is absent. Static addressing is the opposite: name the interface the machine really has (`enp3s0`, `eno1` or `ens18` far more often than `eth0`), because an address hung on a name the machine lacks is refused before anything is pushed, with the machine's own list of interfaces in the message. The inventory is where that name is read off.

Any other field is refused with an explanation: the installer puts nothing else from this document in the payload, and the rest of a node's settings come from its NodeGroup. `spec.storage.wipe` is not accepted at all — on a machine with a single disk, wiping the disk destroys the installation media the OS is being copied from.

Leave `spec.storage` out only on a machine with a single disk. Left out, it renders as `diskSelector` with `size: ">=20Gi"` for the system disk, and the mount for etcd is rendered with a `partitionSelector` that fits a blank disk of 10 GB or more.

That `size: ">=20Gi"` is not a choice of disk, it is a filter, and on a machine where more than one disk is that large nothing gets installed. Before handing a machine anything, the installer reads that machine's inventory and refuses a configuration the machine cannot satisfy: a disk that does not exist, a selector that matches several disks, an interface the machine does not have. The refusal names the field, lists what the machine actually has and prints the line to write instead, and it ends the installation instead of retrying — no amount of waiting grows the machine another disk.

If that check cannot be run, the installer warns and hands over the configuration anyway: a machine whose image is too old to serve an inventory, or one that answers nothing at all, must not be a bootstrap that fails. Then the refusal falls to the machine itself, which installs nothing, prints the disks that matched together with its inventory, and drops to a shell on its console — the only place that message can be read, because a machine that is not a node yet sends its logs nowhere. Before either refusal existed, the first matching disk was taken silently. That is how a node once had the OS written onto a data disk, registered as normal, and failed only at the next boot.

So on a machine with more than one disk, name the disk. `spec.storage.diskSelector` matches on `serial`, `wwid`, `busPath`, `name` and `model` — all five are shell-style patterns — and on `size`, `type` (`nvme`, `ssd`, `hdd`, `sd`) and `rotational`; every field that is set has to match. Instead of a selector, `spec.storage.device` names the disk by path — use a stable one, `/dev/disk/by-path/…` or `/dev/disk/by-id/…`, because `/dev/sda` is handed out in the order the kernel finds the disks and can change between boots. When both are set, the machine reads `diskSelector` and ignores `device`.

Two details are worth knowing when writing that selector. `busPath` matches both spellings the machine reports: the bus path the kernel gives the device, and the name of the link under `/dev/disk/by-path`. And on a hypervisor the disks often have no serial and no `wwid` of their own — for a `virtio-scsi` disk the kernel publishes neither — so the machine reads both from the links under `/dev/disk/by-id`. What a given machine can be matched on at all is what its inventory answers, which is why it is worth reading before the document is written.

`spec.storage.diskSelector` pins the OS disk only. The rendered mount for etcd is checked the same way, so a machine that has more than one device fitting it — a second blank disk of 10 GB or more beside the system disk, say — is refused as well, with those devices named. Name that mount too: add an entry to `spec.storage.mounts` under the name `kubernetes-data` with a `partitionSelector` or a `device` of your own. That entry may not carry `bindTo` or `mode` — the installer refuses the document if it does: `/var/lib/etcd` is the path the etcd static pod carries as a hostPath, so a `bindTo` of your own would leave the node without a working control plane.

{% alert level="warning" %}
Do not copy a `NodeConfig` out of a running cluster: the output of `kubectl get nodeconfig <name> -o yaml` does not parse. The installer reads these documents strictly and refuses the fields the API server adds to them (`creationTimestamp`, `uid`, `resourceVersion`, `status`). Write a minimal document instead, like the one above.
{% endalert %}

Example of the installation command:

```shell
dhctl bootstrap \
  --config=/config.yml \
  --master-host master-0=192.168.199.11 \
  --master-host master-1=192.168.199.12 \
  --master-host master-2=192.168.199.13
```

Where:

- `--master-host`: A control-plane machine as `<node-name>=<address>`, given once per machine. The node name must match `metadata.name` of that machine's `NodeConfig`, and it is the name the node registers under. The machine named first is the one the cluster starts on.
- `--ssh-user` and `--ssh-host` are not used: the machines answer no SSH. An `--ssh-host` (or an `SSHHost` resource in `--connection-config`) is refused before the installation starts, because the checks it turns on would try to log in over SSH to a machine that has no sshd.

The address given in `--master-host` is the one the machine waits for its configuration on, and it may be a lease. If the `NodeConfig` of the first master gives an interface a static address, the installer follows the machine to that address once it has installed itself — to `spec.kubelet.nodeIP` when the document sets one, and otherwise to the first static address the document assigns: the bootstrap channel and the API server are reached there, not at the address the configuration was handed to. Two rules apply to the addresses in `spec.network`:

- An address must carry a prefix length, written as `192.168.0.101/24`. Without one the document is refused, because the machine would configure the interface for a single host and end up off its own subnet with no way to say so.
- Addresses on an interface with `dhcp: true` are ignored by the machine, which takes its address from the lease. The installer does not refuse them, it warns: write `dhcp: false` to have them configured.

If the installation is restarted, the installer skips every machine it has already handed a configuration to — a machine that is a node already refuses a second one. That record is one file per machine, named `immutable-control-plane-pushed-payload-<node-name>`, in the state cache directory the installer prints on start (`State cache directory: …`; the path is a hashed subdirectory of `--cache-dir`, `/tmp/dhctl` by default). If a single machine was reinstalled between the attempts, delete the file of that machine alone. A restart against a cluster where DKP is already installed no longer stops at the `d8-system` namespace: the installer finds the namespace in place and leaves it alone.

{% alert level="warning" %}
Do not reach for `--yes-i-want-to-drop-cache` to do that: it empties the whole cache, including the records of the machines that were not reinstalled and the path to the credentials collected from the first master. The first master has been told those credentials are stored and has closed its bootstrap channel, so they cannot be collected a second time. Dropping the cache is only right when every machine has been reinstalled.
{% endalert %}

Worker nodes are not created by the installer yet. To add one, prepare its `NodeConfig` from the `/config/nodeconfig.yaml` of a node that already runs — change `metadata.name`, `spec.nodeName` and `spec.network`, name the node's group in both `metadata.labels[node.deckhouse.io/group]` and `spec.kubelet.nodeLabels[node.deckhouse.io/group]` (a document copied from a master carries `master` in both), remove the `kubernetes-data` mount of the control plane and the `status` section, and put a fresh `<token-id>.<token-secret>` from a `bootstrap-token-*` Secret of the `kube-system` namespace into `spec.kubelet.bootstrapToken` (such a token is short-lived). Then push the document to the waiting machine:

```shell
curl -X PUT --data-binary @nodeconfig.yaml http://<address>:50000/config
```

An OS image built before the `/config` path became the canonical one serves the document at `/nodeconfig.yml` only. The installer falls back to that path on its own; a hand-written `curl` does not.

### Pre-installation checks

{% alert level="info" %}
Starting with version 1.74, DKP modules are installed as images in the EROFS format that are mounted read-only, which protects them from being modified after installation. This mechanism is enabled automatically if the `erofs` filesystem is registered in the kernel on the node running the DKP controller (a master node by default). DKP loads this kernel module only on nodes with containerd v2, so if master nodes use containerd v1, the operating system has to load `erofs` on its own. Otherwise, DKP will install modules the regular way, without protecting their integrity and without a dedicated alert. For details, refer to ["Integrity protection of DKP modules"](../architecture/security/integrity-control.html#integrity-protection-of-dkp-modules).
{% endalert %}

{% offtopic title="Diagram of checks performed by the installer before installation..." %}
![Diagram of checks performed by the installer before Deckhouse Kubernetes Platform installation](../images/installing/preflight-checks.png)
{% endofftopic %}

List of checks performed by the installer before starting Deckhouse Kubernetes Platform installation:

1. General checks:
   - The values of the parameters [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) and [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) do not match.
   - The authentication data for the container registry specified in the installation configuration is correct.
   - The host name meets the following requirements:
     - The length does not exceed 63 characters.
     - It consists only of lowercase letters.
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
     - At least 4 CPU cores.
     - At least 8 GB of RAM.
     - At least 60 GB of disk space with 400+ IOPS performance.
     - Linux kernel version 5.8 or newer.
     - One of the package managers installed: `apt`, `apt-get`, `yum`, or `rpm`.
     - Access to standard OS package repositories.
     - **When using `ContainerdV2`** as the default container runtime on cluster nodes:
        - Support for `CgroupsV2`.
        - Systemd version `244`.
        - Support for the `erofs` kernel module.
   - Python (Python 3 or Python 2) must be available on the server (VM) intended for the master node. The standard Python modules used by the installer must also be available:
     - For HTTP request handling: `urllib.request` (Python 3) or `urllib2` (Python 2)
     - For HTTP error handling: `urllib.error` (Python 3) or `urllib2` (Python 2)
     - For working with configuration files: `configparser` (Python 3) or `ConfigParser` (Python 2)
     - For HTTP server: `http.server` (Python 3) or `SimpleHTTPServer` (Python 2)
     - For TCP server: `http.server` (Python 3) or `SocketServer` (Python 2)
   - The container registry is accessible through a proxy (if proxy settings are specified in the installation configuration).
   - On the server (VM) intended for the master node and on the host from which the installer is launched, port `22/TCP` (network access from a personal computer) must be accessible, as it is required for the installation process. For details, see the [Network interaction of the platform components](../reference/network_interaction.html) section.
   - DNS must resolve `localhost` to IP address `127.0.0.1`.
   - The user has `sudo` privileges on the server (VM).
   - Required ports for the installation must be open:
     - Port `22/TCP` between the host running the installer and the server.
     - No port conflicts with those used by the installation process.
   - The server (VM) has the correct time.
   - The user `deckhouse` must not exist on the server (VM).
   - The address spaces for Pods (`podSubnetCIDR`), services (`serviceSubnetCIDR`), and internal network (`internalNetworkCIDRs`) do not intersect.

1. Checks for cloud cluster installation:
   - The configuration of the virtual machine for the master node meets the minimum requirements.
   - The cloud provider API is accessible from the cluster nodes.
   - The configuration for [Yandex Cloud with NAT Instance](/modules/cloud-provider-yandex/layouts.html#withnatinstance) is verified.

{% offtopic title="List of preflight skip flags..." %}

To skip a specific check, use the `--preflight-skip-check` flag and pass the preflight check name as its argument. The flag can be specified multiple times.

- `--preflight-skip-all-checks`: Skip all preflight checks.
- `--preflight-skip-check=static-ssh-tunnel`: Skip the SSH forwarding check.
- `--preflight-skip-check=ports-availability`: Skip the check for the availability of required ports.
- `--preflight-skip-check=resolve-localhost`: Skip the `localhost` resolution check.
- `--preflight-skip-check=dhctl-edition`: Skip the DKP version check.
- `--preflight-skip-check=registry-access-through-proxy`: Skip the check for accessing the registry through a proxy server.
- `--preflight-skip-check=public-domain-template`: Skip the check for the `publicDomain` template.
- `--preflight-skip-check=static-ssh-credential`: Skip the check for SSH user credentials.
- `--preflight-skip-check=registry-credentials`: Skip the check for registry access credentials.
- `--preflight-skip-check=python-modules`: Skip the check for Python installation.
- `--preflight-skip-check=sudo-allowed`: Skip the check for `sudo` privileges.
- `--preflight-skip-check=static-system-requirements`: Skip the check for meeting system requirements.
- `--preflight-skip-check=static-single-ssh-host`: Skip the check for the number of specified SSH hosts.
- `--preflight-skip-check=cloud-api-accessibility`: Skip the Cloud API accessibility check.
- `--preflight-skip-check=time-drift`: Skip the time drift check.
- `--preflight-skip-check=cidr-intersection`: Skip the CIDR intersection check.
- `--preflight-skip-check=deckhouse-user`: Skip the `deckhouse` user existence check.
- `--preflight-skip-check=yandex-cloud-config`: Skip the Yandex Cloud with NAT Instance configuration check.
- `--preflight-skip-check=dvp-kubeconfig`: Skip the DVP kubeconfig check.
- `--preflight-skip-check=static-instances-ssh-credentials` — skip verifying accessibility StaticInstances with SSHCredentials.

Example of using a preflight skip flag:

  ```shell
      dhctl bootstrap \
      --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> \
      --config=/config.yml \
      --preflight-skip-all-checks
  ```

> Replace `<SSH_PRIVATE_KEY_FILE>` here with the name of your private key. For example, for a key with RSA encryption it can be `id_rsa`, and for a key with ED25519 encryption it can be `id_ed25519`.

{% endofftopic %}

### Aborting the installation

If the installation was interrupted or issues occurred during the installation process in a supported cloud, there might be leftover resources created during the installation. To remove them, run the following command within the installer container:

```shell
dhctl bootstrap-phase abort
```

{% alert level="warning" %}
The configuration file provided through the `--config` parameter when running the installer must be the same that was used during the initial installation.
{% endalert %}

## Air-gapped environment, working via proxy and using external registries

<div id="installing-deckhouse-kubernetes-platform-from-an-external-registry"></div>

{% alert level="info" %}
For more details on installing and updating DKP in an air-gapped environment, see the [“Installing DKP in an air-gapped environment”](/products/kubernetes-platform/guides/private-environment.html) and [“Updating DKP in an air-gapped environment”](/products/kubernetes-platform/guides/airgapped-update.html) guides.
{% endalert %}

### Installing from an external (third-party) registry

DKP can be installed from an external container registry or via a proxy registry inside an air-gapped environment.

{% alert level="warning" %}
DKP supports Basic and Bearer token authentication schemes for container registries (Basic is tried first; if it fails, Bearer is used).

If a reverse proxy is placed in front of the registry, it must correctly forward the Registry API v2 header `Docker-Distribution-API-Version: registry/2.0`. Otherwise the Basic check may fail, and the subsequent Bearer attempt may fail with the error `couldn't find bearer realm parameter`.

The following container registries were verified and are guaranteed to work:
{%- for registry in site.data.supported_versions.registries %}
[{{- registry[1].shortname }}]({{- registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.

When working with external registries, do not use an administrator account to access them from DKP. Create a dedicated read-only account limited to the required repository in the registry. Refer to an [example of creating](#nexus-configuration-notes) such an account.
{% endalert %}

There are several options for configuring access to external container registries during cluster installation:

- Starting from DKP version 1.75 — using the `deckhouse` ModuleConfig.
- Prior to DKP version 1.75 — using InitConfiguration (considered a legacy method, refer to the example below).

To configure access using the `deckhouse` ModuleConfig, specify the external registry access parameters in the [`settings.registry`](/modules/deckhouse/configuration.html#parameters-registry) section.

<div id="configuration-using-moduleconfig-deckhouse"></div>

Specify the parameters for accessing a third-party registry in the [`settings.registry`](/modules/deckhouse/configuration.html#parameters-registry) section of the ModuleConfig `deckhouse`.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    registry:
      mode: Direct
      direct:
        imagesRepo: test-registry.io/some/path
        scheme: HTTPS
        username: <username>
        password: <password>
        ca: <CA>
```

{% offtopic title="Configuration of an external registry using InitConfiguration **(legacy method)**" %}

Set the following parameters in `InitConfiguration`:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee`: Path to the DKP EE image in an external registry. Example: `imagesRepo: registry.deckhouse.io/deckhouse/ee`.
* `registryDockerCfg: <BASE64>`: Base64-encoded access credentials to the external registry.

If anonymous access is allowed to DKP images in the external registry, the `registryDockerCfg` should look as follows:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

The provided value must be Base64-encoded.

If authentication is required to access DKP images in the external registry, the `registryDockerCfg` should look as follows:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

Where:

* `<PROXY_USERNAME>`: Username for authenticating to `<PROXY_REGISTRY>`
* `<PROXY_PASSWORD>`: Password for authenticating to `<PROXY_REGISTRY>`
* `<PROXY_REGISTRY>`: Address of the external registry in the `<HOSTNAME>[:PORT]` format
* `<AUTH_BASE64>`: A Base64-encoded string of `<PROXY_USERNAME>:<PROXY_PASSWORD>`

The final value for `registryDockerCfg` must also be Base64-encoded.

You can use the following script to generate the `registryDockerCfg`:

```shell
declare MYUSER='<PROXY_USERNAME>'
declare MYPASSWORD='<PROXY_PASSWORD>'
declare MYREGISTRY='<PROXY_REGISTRY>'

MYAUTH=$(echo -n "$MYUSER:$MYPASSWORD" | base64 -w0)
MYRESULTSTRING=$(echo -n "{\"auths\":{\"$MYREGISTRY\":{\"username\":\"$MYUSER\",\"password\":\"$MYPASSWORD\",\"auth\":\"$MYAUTH\"}}}" | base64 -w0)

echo "$MYRESULTSTRING"
```

To support non-standard configurations of external registries, InitConfiguration provides two additional parameters:

* `registryCA`: Root certificate to validate the registry's certificate (used if the registry uses self-signed certificates).
* `registryScheme`: Protocol used to access the registry (`HTTP` or `HTTPS`). The default is `HTTPS`.
{% endofftopic %}

### Nexus configuration notes

{% alert level="warning" %}
When interacting with a `docker`-type repository in Nexus (for example, using `docker pull` or `docker push` commands), you must specify the address in the following format: `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>`.

Using the `URL` value from the Nexus repository settings is **not allowed**.
{% endalert %}

When using the [Nexus](https://github.com/sonatype/nexus-public) repository manager, the following requirements must be met:

* A proxy Docker repository must be created ("Administration" → "Repository" → "Repositories"):
  * The `Maximum metadata age` parameter is set to `0`.
* Access control must be configured:
  * Create a role named **Nexus** ("Administration" → "Security" → "Roles") with the following privileges:
    * `nx-repository-view-docker-<repository>-browse`
    * `nx-repository-view-docker-<repository>-read`
  * Create a user ("Administration" → "Security" → "Users") and assign them the **Nexus** role.

To configure Nexus, follow these steps: 

1. Create a proxy Docker repository ("Administration" → "Repository" → "Repositories") that points to the [public Deckhouse container registry](https://registry.deckhouse.io/).
   ![Create Proxy Docker Repository](../images/registry/nexus/nexus-repository.png)

1. Fill out the repository creation form with the following values:
   * `Name`: the desired repository name, e.g., `d8-proxy`.
   * `Repository Connectors / HTTP` or `HTTPS`: a dedicated port for the new repository, e.g., `8123` or another.
   * `Remote storage`: must be set to `https://registry.deckhouse.io/`.
   * `Auto blocking enabled` and `Not found cache enabled`: can be disabled for debugging; otherwise, enable them.
   * `Maximum Metadata Age`: must be set to `0`.
   * If using a commercial edition of Deckhouse Kubernetes Platform, enable the `Authentication` checkbox and fill in the following:
     * `Authentication Type`: `Username`.
     * `Username`: `license-token`.
     * `Password`: Your Deckhouse Kubernetes Platform license key.

   ![Example repository settings 1](../images/registry/nexus/nexus-repo-example-1.png)  
   ![Example repository settings 2](../images/registry/nexus/nexus-repo-example-2.png)  
   ![Example repository settings 3](../images/registry/nexus/nexus-repo-example-3.png)

1. Configure Nexus access control to allow DKP to access the created repository:
   * Create a **Nexus** role ("Administration" → "Security" → "Roles") with the following privileges: `nx-repository-view-docker-<repository>-browse` and `nx-repository-view-docker-<repository>-read`.

     ![Create Nexus Role](../images/registry/nexus/nexus-role.png)

   * Create a user ("Administration" → "Security" → "Users") and assign them the role created above.

     ![Create Nexus User](../images/registry/nexus/nexus-user.png)

   * Enable **Docker Bearer Token Realm** ("Administration" → "Security" → "Realms"):
     * The **Docker Bearer Token Realm** must be in the **Active** list (on the right), not in **Available** (on the left).
     * If it is not in **Active**:
       1. Find it in the **Available** list.
       1. Move it to **Active** using the arrow button.
       1. Click **Save**.
       1. **Restart Nexus** (it is required for the changes to take effect).

     ![Docker Bearer Token Realm Configuration](../images/registry/nexus/nexus-realms.png)

As a result, DKP images will be available at a URL as follows: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

### Harbor configuration notes

Use the [Harbor Proxy Cache](https://github.com/goharbor/harbor) feature.

1. Configure the registry access:
   * In the side menu, navigate to "Administration" → "Registries"
     and click "New Endpoint" to add a new endpoint for the registry.
   * In the "Provider" dropdown list, select "Docker Registry".
   * In the "Name" field, enter an endpoint name of your choice.
   * In the "Endpoint URL" field, enter `https://registry.deckhouse.io`.
   * In the "Access ID" field, enter `license-token`.
   * In the "Access Secret" field, enter your Deckhouse Kubernetes Platform license key.
   * Set any remaining parameters as necessary.
   * Click "OK" to confirm creation of a new endpoint for the registry.

   ![Configuring registry access](../images/registry/harbor/harbor1.png)

1. Create a new project:
   * In the side menu, navigate to "Projects" and click "New Project" to add a project.
   * In the "Project Name" field, enter a project name of your choice (for example, `d8s`).
     This name will be a part of the URL.
   * In the "Access Level" field, select "Public".
   * Enable "Proxy Cache" and in the dropdown list, select the registry created earlier.
   * Set any remaining parameters as necessary.
   * Click "OK" to confirm creation of a new project.

   ![Creating a new project](../images/registry/harbor/harbor2.png)

Once Harbor is configured, DKP images will be available at a URL as follows: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Manual loading of DKP images and vulnerability DB into a private registry

{% alert level="warning" %}
The `d8 mirror` utility is not available for use with the Community Edition (CE) and Basic Edition (BE).
{% endalert %}

{% alert level="info" %}
You can check the current status of versions in the release channels at [releases.deckhouse.io](https://releases.deckhouse.io).
{% endalert %}

- [Download and install the Deckhouse CLI utility](../cli/d8/).

- Download DKP images to a dedicated directory using the `d8 mirror pull` command.

  By default, `d8 mirror pull` downloads only the current versions of DKP, vulnerability scanner databases (if included in the DKP edition), and officially delivered modules.
  For example, for Deckhouse Kubernetes Platform 1.59, only version 1.59.12 will be downloaded, as it is sufficient for upgrading the platform from 1.58 to 1.59.

  Run the following command (specify the edition code and license key) to download the current version images:

  ```shell
  d8 mirror pull \
    --source='registry.deckhouse.io/deckhouse/<EDITION>' \
    --license='<LICENSE_KEY>' /home/user/d8-bundle
  ```

  Where:

  - `--source`: Address of the Deckhouse container registry.
  - `<EDITION>`: Deckhouse Kubernetes Platform edition code (for example, `ee`, `se`, `se-plus`). By default, the `--source` parameter refers to the Enterprise Edition (`ee`) and can be omitted.
  - `--license`: Parameter for specifying the Deckhouse Kubernetes Platform license key for authentication in the official container registry.
  - `<LICENSE_KEY>`: Deckhouse Kubernetes Platform license key.
  - `/home/user/d8-bundle`: Directory where the image packages will be placed. It will be created if it does not exist.

  > If the image download is interrupted, rerunning the command will resume the download, provided no more than one day has passed since the interruption.

  Example command to download all DKP EE versions starting from version 1.59 (specify your license key):

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --since-version=1.59 /home/user/d8-bundle
  ```

  Example command to download the current DKP SE versions (specify your license key):

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --source='registry.deckhouse.io/deckhouse/se' \
  /home/user/d8-bundle
  ```

  Example command to download DKP images from an external registry:

  ```shell
  d8 mirror pull \
  --source='corp.company.com:5000/sys/deckhouse' \
  --source-login='<USER>' --source-password='<PASSWORD>' /home/user/d8-bundle
  ```

  Example command to download the vulnerability scanner database package:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-modules /home/user/d8-bundle
  ```

  Example command to download all available additional module packages:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db /home/user/d8-bundle
  ```

  Example command to download module packages `stronghold` and `secrets-store-integration`:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module stronghold \
  --include-module secrets-store-integration \
  /home/user/d8-bundle
  ```

  Example command to download `stronghold` module with semver `^` constraint (allows updates that don't change the leftmost non-zero digit) from version 1.2.0:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module stronghold@1.2.0 \
  /home/user/d8-bundle
  ```

  Example command to download `secrets-store-integration` module with semver `~` constraint (allows updates that don't change the last specified digit's place) from version 1.1.0:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module secrets-store-integration@~1.1.0 \
  /home/user/d8-bundle
  ```

  Quote the `--include-module` flag value when it contains `>=` or `<=`. Unquoted, the shell treats it as an input/output redirection.

  Example command to download the `console` module with semver `>=` constraint (any version from the specified one and above) from version 1.43.2:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module "console@>=1.43.2" \
  /home/user/d8-bundle
  ```

  Example command to download exact version of `stronghold` module 1.2.5 and publish to all release channels:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module stronghold@=v1.2.5 \
  /home/user/d8-bundle
  ```

{% offtopic title="Other command parameters available for use:" %}

- `--no-pull-resume`: Force the download to start from the beginning.
- `--force`: Overwrite existing packages if they conflict with the current pull operation.
- `--ignore-suspend`: Ignore suspended release channels and continue mirroring. Use with caution.
- `--no-platform`: Skip downloading the Deckhouse Kubernetes Platform image package (`platform.tar`).
- `--no-modules`: Skip downloading module packages (`module-*.tar`).
- `--no-security-db`: Skip downloading the vulnerability scanner database package (`security.tar`).
- `--no-packages`: Skip downloading Deckhouse packages.
- `--no-installer`: Skip downloading Deckhouse installer images.
- `--only-extra-images`: Pull only extra module images without pulling main module images.
- `--skip-vex-images`: Skip downloading VEX images.
- `--include-platform` = `CONSTRAINT`: Download Deckhouse Kubernetes Platform releases by a semver constraint. Cannot be used together with `--since-version` and `--deckhouse-tag`. Always quote the constraint value: `>` and `<` are shell redirections. Examples: `--include-platform ">=1.64 <=1.68"`, `--include-platform "~1.65.0"`, `--include-platform "^1.65.0"`, `--include-platform "1.65.0"`, `--include-platform "=v1.65.3"`, or `--include-platform "=v1.65.3+stable"`.
- `--include-module` / `-i` = `name[@Major.Minor]`: Download only a specific set of modules using a whitelist (and, if needed, their minimum versions). Use multiple times to add more modules to the whitelist. These flags are ignored if used with `--no-modules`.

  The following syntax options are supported for specifying module versions. When using `>=` or `<=`, quote the whole flag value:
  - `module-name@1.3.0`: Pulls versions with semver ^ constraint (^1.3.0), including v1.3.0, v1.3.3, v1.4.1.
  - `module-name@~1.3.0`: Pulls versions with semver ~ constraint (>=1.3.0 <1.4.0), including only v1.3.0, v1.3.3.
  - `"module-name@>=1.3.0"`: Pulls versions with semver `>=` constraint, including the explicitly specified version and newer versions matching the constraint.
  - `"module-name@>=1.3.0 <=1.4.0"`: Pulls versions in the range, honoring both the lower and upper boundaries.
  - `module-name@=v1.3.0`: Pulls exact tag match v1.3.0, publishing to all release channels.
  - `module-name@=v1.3.0+stable`: Pulls exact tag match v1.3.0, publishing to the stable release channel.
  - `module-name@=bobV1`: Pulls exact tag match "bobV1", publishing to all release channels.
- `--exclude-module` / `-e` = `name`: Skip downloading a specific set of modules using a blacklist. Use multiple times to add more modules to the blacklist. Ignored if `--no-modules` or `--include-module` is used;
- `--include-package` = `name[@version]`: Download only a specific set of packages using a whitelist. Versions and semver constraints use the same syntax as `--include-module`, including quoting rules.
- `--exclude-package` = `name[@version]`: Skip downloading a specific set of packages using a blacklist. Ignored if `--include-package` is used.
- `--modules-path-suffix`: Change the suffix of the path to the module repository in the main DKP registry. The default suffix is `/modules` (for example, the full path to the module repo will be `registry.deckhouse.io/deckhouse/EDITION/modules`).
- `--since-version=X.Y`: Download all DKP versions starting from the specified minor version. This option is ignored if the specified version is higher than the version on the Rock Solid release channel. Cannot be used with `--deckhouse-tag`.
- `--deckhouse-tag`: Download only the specific DKP version (regardless of release channels). Cannot be used with `--since-version`.
- `--installer-tag=TAG`: Download a specific Deckhouse installer tag. Defaults to `latest` if omitted.
- `--proxy-registry`: Pull from a proxy/cache registry that does not support the registry catalog API. Requires `--include-platform` unless the platform is skipped with `--no-platform`, and at least one `--include-module` unless modules are skipped with `--no-modules`. Cannot be used together with `--deckhouse-tag` or `--since-version`.
- `--dry-run`: Print what would be pulled without downloading images.
- `--verbose-summary`: Print a detailed summary for every module and package with resolved versions.
- `--gost-digest`: Calculate the checksum of the final DKP image bundle using the GOST R 34.11-2012 (Streebog) algorithm. The checksum will be displayed and written to a `.tar.gostsum` file in the folder containing the image tarball.
- `--source-login` and `--source-password`: Authentication data to access the external registry.
- `--tls-skip-verify`: Disable TLS certificate validation.
- `--insecure`: Interact with registries over HTTP.
- `--images-bundle-chunk-size=N`: The maximum file size (in GB) to split the image archive into. As a result, instead of one image archive, a set of CHUNK files will be created (for example, `d8.tar.NNNN.chunk`). To upload images from such a set, use the file name without the `.NNNN.chunk` suffix (for example, `d8.tar` for files `d8.tar.NNNN.chunk`).
- `--tmp-dir`: Path to a directory for temporary files used during image download and upload. All processing is done in this directory. It must have enough free disk space to hold the entire image bundle. Defaults to the `.tmp` subdirectory in the image bundle directory.

Additional configuration parameters for the `d8 mirror` command family are available as environment variables:

- `HTTP_PROXY` / `HTTPS_PROXY`: Proxy server URL for HTTP(S) requests not listed in the `$NO_PROXY` variable.
- `NO_PROXY`: Comma-separated list of hosts to exclude from proxying. Each entry can be an IP (`1.2.3.4`), CIDR (`1.2.3.4/8`), domain, or wildcard (`*`). IPs and domains may include a port (`1.2.3.4:80`). A domain matches itself and all subdomains. A domain starting with a `.` matches only subdomains. For example, `foo.com` matches `foo.com` and `bar.foo.com`; `.y.com` matches `x.y.com` but not `y.com`. The `*` disables proxying.
- `SSL_CERT_FILE`: Path to an SSL certificate. If set, system certificates are not used.
- `SSL_CERT_DIR`: Colon-separated list of directories to search for SSL certificate files. If set, system certificates are not used. [More info...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html)
- `MIRROR_BYPASS_ACCESS_CHECKS`: Set this variable to `1` to disable credential validation for the registry.
{% endofftopic %}

- On the host with access to the container registry where DKP images should be uploaded, copy the downloaded DKP image bundle and install the [Deckhouse CLI](../cli/d8/).

- Upload DKP images to the registry using the `d8 mirror push` command.

  The `d8 mirror push` command uploads images from all packages located in the specified directory.
  If you only want to push specific packages, you can either run the command separately for each TAR image bundle by specifying the direct path to it,
  or temporarily remove the `.tar` extension from unwanted files or move them out of the directory.

  Example command to upload image packages from the `/mnt/MEDIA/d8-images` directory (provide authentication data if required):

  ```shell
  d8 mirror push /mnt/MEDIA/d8-images 'corp.company.com:5000/sys/deckhouse' \
    --registry-login='<USER>' --registry-password='<PASSWORD>'
  ```

  Before uploading the images, make sure that the target path in the container registry exists (in the example, `/sys/deckhouse`) and that the account used has write permissions.

  If you're using Harbor, you won't be able to upload images to the root of a project. Use a dedicated repository within the project to store DKP images.

- After uploading the images to the registry, you can proceed with installing DKP. Use the [getting-started guide](/products/kubernetes-platform/gs/bm-private/step2.html).

  When running the installer, use the address of your own image registry (where the images were uploaded earlier) instead of the official public DKP container registry. For the example above, the installer image address will be `corp.company.com:5000/sys/deckhouse/install:stable` instead of `registry.deckhouse.io/deckhouse/ee/install:stable`.

  In the [`registry`](/modules/deckhouse/configuration.html#parameters-registry) section of ModuleConfig `deckhouse`, use your registry address and authorization data (starting with DKP 1.75). The legacy method is to use [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) (parameters [`imagesRepo`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), [`registryDockerCfg`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-registrydockercfg)).

### Creating a cluster and running DKP without using release channels

{% alert level="warning" %}
This method should only be used if your private (isolated) registry does not contain images with release channel metadata.
{% endalert %}

If you need to install DKP with automatic updates disabled:

1. Use the installer image tag corresponding to the desired version. For example, to install release `v1.44.3`, use the image `your.private.registry.com/deckhouse/install:v1.44.3`.
1. Specify the appropriate version number in the [`deckhouse.devBranch`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-devbranch) parameter of [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration).  
   > **Do not specify** the [`deckhouse.releaseChannel`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleupdatepolicy-v1alpha2-spec-releasechannel) parameter in [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration).

If you want to disable automatic updates in an already running DKP installation (including patch updates), remove the [`releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter from the `deckhouse` module configuration.

### Using a proxy server

{% offtopic title="Example steps for configuring a proxy server using Squid..." %}

1. Prepare a server (or virtual machine). The server must be reachable from the required cluster nodes and must have internet access.
1. Install Squid (the following examples are for Ubuntu):

   ```shell
   apt-get install squid
   ```

1. Create the Squid configuration file:

   ```shell
   cat <<EOF > /etc/squid/squid.conf
   auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
   auth_param basic realm proxy
   acl authenticated proxy_auth REQUIRED
   http_access allow authenticated
   # Specify the required port. Port 3128 is used by default.
   http_port 3128
   ```

1. Create a username and password for proxy server authentication:

   Example for user `test` with password `test` (make sure to change it):

   ```shell
   echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
   ```

1. Start Squid and enable it to start on server boot:

   ```shell
   systemctl restart squid
   systemctl enable squid
   ```

{% endofftopic %}

To configure DKP to work with a proxy server, use the [`proxy`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-proxy) parameter of the ClusterConfiguration resource.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
cri: "Containerd"
clusterDomain: "cluster.local"
proxy:
  httpProxy: "http://user:password@proxy.company.my:3128"
  httpsProxy: "https://user:password@proxy.company.my:8443"
```

{% raw %}

### Automatic proxy variable loading for users in CLI

Starting from version 1.67, the `/etc/profile.d/d8-system-proxy.sh` file is no longer configured in DKP to set proxy variables for users.  
To automatically load proxy variables for users in CLI, use the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: profile-proxy.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 99
  content: |
    {{- if .proxy }}
      {{- if .proxy.httpProxy }}
    export HTTP_PROXY={{ .proxy.httpProxy | quote }}
    export http_proxy=${HTTP_PROXY}
      {{- end }}
      {{- if .proxy.httpsProxy }}
    export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
    export https_proxy=${HTTPS_PROXY}
      {{- end }}
      {{- if .proxy.noProxy }}
    export NO_PROXY={{ .proxy.noProxy | join "," | quote }}
    export no_proxy=${NO_PROXY}
      {{- end }}
    bb-sync-file /etc/profile.d/profile-proxy.sh - << EOF
    export HTTP_PROXY=${HTTP_PROXY}
    export http_proxy=${HTTP_PROXY}
    export HTTPS_PROXY=${HTTPS_PROXY}
    export https_proxy=${HTTPS_PROXY}
    export NO_PROXY=${NO_PROXY}
    export no_proxy=${NO_PROXY}
    EOF
    {{- else }}
    rm -rf /etc/profile.d/profile-proxy.sh
    {{- end }}
```

{% endraw %}
