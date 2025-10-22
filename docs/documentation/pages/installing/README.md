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

* **In a supported cloud.** The `dhctl` utility automatically creates and configures all necessary resources, including virtual machines, deploys the Kubernetes cluster, and installs Deckhouse. A full list of supported cloud providers is available in the [Platform integration with infrastructure](../admin/integrations/integrations-overview.html) section.

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

1. [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) — initial parameters for [Deckhouse configuration](../#deckhouse-configuration), necessary for the proper startup of Deckhouse after installation.

   Key settings specified in this resource:
   * [Component placement parameters](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys);
   * The [StorageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass) (storage parameters);
   * Access parameters for the [container registry](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-registrydockercfg);
   * Template for [DNS names](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate);
   * Other essential parameters required for Deckhouse to function correctly.

1. [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — general cluster parameters, such as control plane version, network settings, CRI parameters, etc.
    > This resource is needed only when Deckhouse is being installed with a pre-deployed Kubernetes cluster. If Deckhouse is being installed in an already existing cluster, this resource is not required.

1. [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration) — parameters for Kubernetes clusters deployed on bare-metal servers or virtual machines in unsupported clouds.
   > This resource is needed only when Deckhouse is being installed with a pre-deployed Kubernetes cluster. If Deckhouse is being installed in an already existing cluster, this resource is not required.

1. `<CLOUD_PROVIDER>ClusterConfiguration` — a set of resources containing configuration parameters for supported cloud providers. These include:
   * Cloud infrastructure access settings (authentication parameters);
   * Resource placement scheme type and parameters;
   * Network settings;
   * Node group creation settings.

   List of cloud provider configuration resources:
   * [AWSClusterConfiguration](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) — Amazon Web Services;
   * [AzureClusterConfiguration](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) — Microsoft Azure;
   * [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) — Google Cloud Platform;
   * [HuaweiCloudClusterConfiguration](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration) — Huawei Cloud;
   * [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) — OpenStack;
   * [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) — VMware vSphere;
   * [VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration) — VMware Cloud Director;
   * [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — Yandex Cloud;
   * [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) — zVirt.

1. [ModuleConfig](/products/kubernetes-platform/documentation/latest/reference/api/cr.html#moduleconfig) — a set of resources containing configuration parameters for Deckhouse built-in modules.

   If the cluster is initially created with nodes dedicated to specific types of workloads (e.g., system nodes or monitoring nodes), it is recommended to explicitly set the `nodeSelector` parameter in the configuration of modules that use persistent storage volumes.

   For example, for the `prometheus` module, the configuration is specified in the [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector) parameter.

1. [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) — deploying the Ingress controller.

1. [NodeGroup](/modules/node-manager/cr.html#nodegroup) — creating additional node groups.

1. InstanceClass — adding configuration resources.

1. [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule), [User](/modules/user-authn/cr.html#user) — setting up roles and users.

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
1. `<RELEASE_CHANNEL>` — the [release channel](/modules/deckhouse/configuration.html#parameters-releasechannel) in kebab-case format:
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
   - The values of the parameters [PublicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) and [clusterDomain](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) do not match.
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

## Air-Gapped environment, working via proxy and using external registries

### Installing Deckhouse Kubernetes Platform from an external registry

{% alert level="warning" %}  
Available in the following editions: BE, SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

{% alert level="warning" %}  
DKP supports only the Bearer token authentication scheme for container registries.  
The following container registries are tested and officially supported:  
{% for registry in site.data.supported_versions.registries %}
[{{ registry[1].shortname }}]({{ registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.
{% endalert %}

During installation, DKP can be configured to work with an external registry (e.g., a proxy registry in an air-gapped environment).

Set the following parameters in the `InitConfiguration` resource:

- `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — the path to the DKP EE image in the external registry.  
  Example: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
- `registryDockerCfg: <BASE64>` — base64-encoded Docker config with access credentials to the external registry.

If anonymous access is allowed to DKP images in the external registry, the `registryDockerCfg` should look like this:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

The provided value must be Base64-encoded.

If authentication is required to access DKP images in the external registry, the `registryDockerCfg` should look like this:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

where:

* `<PROXY_USERNAME>` — the username for authenticating to `<PROXY_REGISTRY>`;
* `<PROXY_PASSWORD>` — the password for authenticating to `<PROXY_REGISTRY>`;
* `<PROXY_REGISTRY>` — the address of the external registry in the format `<HOSTNAME>[:PORT]`;
* `<AUTH_BASE64>` — a Base64-encoded string of `<PROXY_USERNAME>:<PROXY_PASSWORD>`.

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

## Custom external registry configuration

To support non-standard configurations of external registries, the `InitConfiguration` resource provides two additional parameters:

* `registryCA` — a root certificate to validate the registry's certificate (used if the registry uses self-signed certificates);
* `registryScheme` — the protocol used to access the registry (`HTTP` or `HTTPS`). Defaults to `HTTPS`.

### Nexus configuration notes

{% alert level="warning" %}
When interacting with a `docker`-type repository in Nexus (e.g., using `docker pull` or `docker push`), you must specify the address in the format `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>`.  
Using the `URL` value from the Nexus repository settings is **not supported**.
{% endalert %}

When using the [Nexus](https://github.com/sonatype/nexus-public) repository manager, the following requirements must be met:

* A proxy Docker repository must be created (`Administration` → `Repository` → `Repositories`):
  * Set the `Maximum metadata age` parameter to `0`.
* Access control must be configured:
  * Create a role named **Nexus** (`Administration` → `Security` → `Roles`) with the following privileges:
    * `nx-repository-view-docker-<repository>-browse`
    * `nx-repository-view-docker-<repository>-read`
  * Create a user (`Administration` → `Security` → `Users`) and assign them the **Nexus** role.

Setup Steps:

1. Create a proxy Docker repository (`Administration` → `Repository` → `Repositories`) that points to the [Deckhouse registry](https://registry.deckhouse.ru/):  
   ![Create Proxy Docker Repository](../images/registry/nexus/nexus-repository.png)

1. Fill out the repository creation form with the following values:
   * `Name`: the desired repository name, e.g., `d8-proxy`.
   * `Repository Connectors / HTTP` or `HTTPS`: a dedicated port for the new repository, e.g., `8123` or another.
   * `Remote storage`: must be set to `https://registry.deckhouse.ru/`.
   * `Auto blocking enabled` and `Not found cache enabled`: can be disabled for debugging; otherwise, enable them.
   * `Maximum Metadata Age`: must be set to `0`.
   * If using a commercial edition of Deckhouse Kubernetes Platform, enable the `Authentication` checkbox and fill in the following:
     * `Authentication Type`: `Username`
     * `Username`: `license-token`
     * `Password`: your Deckhouse Kubernetes Platform license key

   ![Example repository settings 1](../images/registry/nexus/nexus-repo-example-1.png)  
   ![Example repository settings 2](../images/registry/nexus/nexus-repo-example-2.png)  
   ![Example repository settings 3](../images/registry/nexus/nexus-repo-example-3.png)

1. Configure Nexus access control to allow DKP to access the created repository:
   * Create a **Nexus** role (`Administration` → `Security` → `Roles`) with the following privileges:  
     `nx-repository-view-docker-<repository>-browse` and `nx-repository-view-docker-<repository>-read`.

     ![Create Nexus Role](../images/registry/nexus/nexus-role.png)

   * Create a user (`Administration` → `Security` → `Users`) and assign them the role created above.

     ![Create Nexus User](../images/registry/nexus/nexus-user.png)

     As a result, DKP images will be available at a URL like: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

### Harbor configuration notes

Use the [Harbor Proxy Cache](https://github.com/goharbor/harbor) feature.

* Configure the registry:
  * Go to `Administration` → `Registries` → `New Endpoint`.
  * `Provider`: Docker Registry.
  * `Name`: arbitrary value of your choice.
  * `Endpoint URL`: `https://registry.deckhouse.ru`.
  * Set `Access ID` and `Access Secret` (your Deckhouse Kubernetes Platform license key).

    ![Registry Configuration](../images/registry/harbor/harbor1.png)

* Create a new project:
  * Navigate to `Projects → New Project`.
  * `Project Name` will be part of the URL. Choose any name, e.g., `d8s`.
  * `Access Level`: `Public`.
  * Enable `Proxy Cache` and select the registry created in the previous step.

    ![Create New Project](../images/registry/harbor/harbor2.png)

    As a result, DKP images will be available at a URL like: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Manual loading of Deckhouse Kubernetes Platform images, vulnerability scanner DB, and DKP modules into a private registry

{% alert level="warning" %}
The `d8 mirror` utility is not available for use with the Community Edition (CE) and Basic Edition (BE).
{% endalert %}

{% alert level="info" %}
You can check the current status of versions in the release channels at [releases.deckhouse.ru](https://releases.deckhouse.ru).
{% endalert %}

1. [Download and install the Deckhouse CLI utility](../cli/d8/).

1. Download DKP images to a dedicated directory using the `d8 mirror pull` command.

   By default, `d8 mirror pull` downloads only the current versions of DKP, vulnerability scanner databases (if included in the DKP edition), and officially delivered modules.  

   For example, for Deckhouse Kubernetes Platform 1.59, only version 1.59.12 will be downloaded, as it is sufficient for upgrading the platform from 1.58 to 1.59.

   Run the following command (specify the edition code and license key) to download the current version images:

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.ru/deckhouse/<EDITION>' \
     --license='<LICENSE_KEY>' /home/user/d8-bundle
   ```

   where:

   - `--source` — address of the Deckhouse Kubernetes Platform container registry.
   - `<EDITION>` — Deckhouse Kubernetes Platform edition code (e.g., `ee`, `se`, `se-plus`). By default, the `--source` parameter refers to the Enterprise Edition (`ee`) and can be omitted.
   - `--license` — parameter for specifying the Deckhouse Kubernetes Platform license key for authentication in the official container registry.
   - `<LICENSE_KEY>` — Deckhouse Kubernetes Platform license key.
   - `/home/user/d8-bundle` — directory where the image packages will be placed. It will be created if it does not exist.

   > If the image download is interrupted, rerunning the command will resume the download, provided no more than one day has passed since the interruption.

   {% offtopic title="Other command parameters available for use:" %}

   - `--no-pull-resume` — force the download to start from the beginning;
   - `--no-platform` — skip downloading the Deckhouse Kubernetes Platform image package (`platform.tar`);
   - `--no-modules` — skip downloading module packages (`module-*.tar`);
   - `--no-security-db` — skip downloading the vulnerability scanner database package (`security.tar`);
   - `--include-module` / `-i` = `name[@Major.Minor]` — download only a specific set of modules using a whitelist (and, if needed, their minimum versions). Use multiple times to add more modules to the whitelist. These flags are ignored if used with `--no-modules`.

     The following syntax options are supported for specifying module versions:
     - `module-name@1.3.0` — pulls versions with semver ^ constraint (^1.3.0), including v1.3.0, v1.3.3, v1.4.1;
     - `module-name@~1.3.0` — pulls versions with semver ~ constraint (>=1.3.0 <1.4.0), including only v1.3.0, v1.3.3;
     - `module-name@=v1.3.0` — pulls exact tag match v1.3.0, publishing to all release channels;
     - `module-name@=bobV1` — pulls exact tag match "bobV1", publishing to all release channels;
   - `--exclude-module` / `-e` = `name` — skip downloading a specific set of modules using a blacklist. Use multiple times to add more modules to the blacklist. Ignored if `--no-modules` or `--include-module` is used;
   - `--modules-path-suffix` — change the suffix of the path to the module repository in the main DKP registry. The default suffix is `/modules` (e.g., full path to the module repo will be `registry.deckhouse.ru/deckhouse/EDITION/modules`);
   - `--since-version=X.Y` — download all DKP versions starting from the specified minor version. This option is ignored if the specified version is higher than the version on the Rock Solid update channel. Cannot be used with `--deckhouse-tag`;
   - `--deckhouse-tag` — download only the specific DKP version (regardless of update channels). Cannot be used with `--since-version`;
   - `--gost-digest` — calculate the checksum of the final DKP image bundle using the GOST R 34.11-2012 (Streebog) algorithm. The checksum will be displayed and written to a `.tar.gostsum` file in the folder containing the image tarball;
   - use the `--source-login` and `--source-password` parameters to authenticate with an external image registry;
   - `--images-bundle-chunk-size=N` — set the maximum file size (in GB) to split the image archive. As a result, instead of one image archive, a set of `.chunk` files will be created (e.g., `d8.tar.NNNN.chunk`). To upload images from such a set, use the file name without the `.NNNN.chunk` suffix (e.g., `d8.tar` for files `d8.tar.NNNN.chunk`);
   - `--tmp-dir` — path to a directory for temporary files used during image download and upload. All processing is done in this directory. It must have enough free disk space to hold the entire image bundle. Defaults to the `.tmp` subdirectory in the image bundle directory.
  
   {% endofftopic %}

   Additional configuration parameters for the `d8 mirror` command family are available as environment variables.

   {% offtopic title="More details:" %}

   - `HTTP_PROXY` / `HTTPS_PROXY` — proxy server URL for HTTP(S) requests not listed in the `$NO_PROXY` variable.
   - `NO_PROXY` — comma-separated list of hosts to exclude from proxying. Each entry can be an IP (`1.2.3.4`), CIDR (`1.2.3.4/8`), domain, or wildcard (`*`). IPs and domains may include a port (`1.2.3.4:80`). A domain matches itself and all subdomains. A domain starting with a `.` matches only subdomains. For example, `foo.com` matches `foo.com` and `bar.foo.com`; `.y.com` matches `x.y.com` but not `y.com`. The `*` disables proxying.
   - `SSL_CERT_FILE` — path to an SSL certificate. If set, system certificates are not used.
   - `SSL_CERT_DIR` — colon-separated list of directories to search for SSL certificate files. If set, system certificates are not used. [More info...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html)
   - `MIRROR_BYPASS_ACCESS_CHECKS` — set this variable to `1` to disable credential validation for the registry.

   {% endofftopic %}

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
   --source='registry.deckhouse.ru/deckhouse/se' \
   /home/user/d8-bundle
   ```

   Example command to download DKP images from an external image registry:

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

   Example command to download `stronghold` module with semver `^` constraint from version 1.2.0:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db \
   --include-module stronghold@1.2.0 \
   /home/user/d8-bundle
   ```

   Example command to download `secrets-store-integration` module with semver `~` constraint from version 1.1.0:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db \
   --include-module secrets-store-integration@~1.1.0 \
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

1. On the host with access to the registry where DKP images should be uploaded, copy the downloaded DKP image bundle and install the [Deckhouse CLI](../cli/d8/).

1. Copy the downloaded DKP image bundle and install the [Deckhouse CLI](../cli/d8/) on the host that has access to the target image registry.

1. Upload the DKP images to the registry using the `d8 mirror push` command.

   The `d8 mirror push` command uploads images from all packages located in the specified directory.
   If you only want to push specific packages, you can either run the command separately for each `.tar` image bundle by specifying the direct path to it,
   or temporarily remove the `.tar` extension from unwanted files or move them out of the directory.

   Example command to upload image packages from the `/mnt/MEDIA/d8-images` directory (provide authentication data if required):

   ```shell
   d8 mirror push /mnt/MEDIA/d8-images 'corp.company.com:5000/sys/deckhouse' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   Before uploading the images, make sure that the target path in the image registry exists (in the example — `/sys/deckhouse`) and that the account used has write permissions.

   If you're using Harbor, you won't be able to upload images to the root of a project. Use a dedicated repository within the project to store DKP images.

1. After uploading the images to the registry, you can proceed with installing DKP. Use the [Quick Start Guide](/products/kubernetes-platform/gs/bm-private/step2.html).

   When running the installer, use the address of your own image registry (where the images were uploaded earlier) instead of the official public DKP registry. For the example above, the installer image address will be `corp.company.com:5000/sys/deckhouse/install:stable` instead of `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   In the [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) resource during installation, also use your registry address and authorization data (parameters [imagesRepo](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-registrydockercfg), or [Step 3]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm-private/step3.html) of the Quick Start Guide).

### Creating a cluster and running DKP without using update channels

{% alert level="warning" %}
This method should only be used if your isolated private registry does not contain images with update channel metadata.
{% endalert %}

If you need to install DKP with automatic updates disabled:

1. Use the installer image tag corresponding to the desired version. For example, to install release `v1.44.3`, use the image `your.private.registry.com/deckhouse/install:v1.44.3`.
1. Specify the appropriate version number in the [deckhouse.devBranch](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-devbranch) parameter of the [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) resource.  
   > **Do not specify** the [deckhouse.releaseChannel](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleupdatepolicy-v1alpha2-spec-releasechannel) parameter in the [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) resource.

If you want to disable automatic updates in an already running Deckhouse installation (including patch updates), remove the [releaseChannel](/modules/deckhouse/configuration.html#parameters-releasechannel) parameter from the `deckhouse` module configuration.

### Using a proxy server

{% alert level="warning" %}
Available in the following editions: BE, SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

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

To configure DKP to use a proxy, use the [proxy](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-proxy) parameter in the ClusterConfiguration resource.

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

Starting from DKP v1.67, the `/etc/profile.d/d8-system-proxy.sh` file is no longer configured to set proxy variables for users.  
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
