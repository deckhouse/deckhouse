---
title: "Base platform installation"
permalink: en/virtualization-platform/documentation/admin/install/steps/base-cluster.html
---

## Preparing the Configuration

To install the platform, you need to create an installation YAML configuration file.

### Installation Configuration File

The YAML installation configuration file includes parameters for several resources (manifests):

- [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — general cluster settings such as control plane version, networking parameters, CRI settings, and more.

  > The ClusterConfiguration resource should only be included in the configuration if the platform installation involves deploying a new Kubernetes cluster. It is not required when installing the platform into an existing Kubernetes cluster.

- [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration) — settings for a Kubernetes cluster deployed on bare-metal servers.

  > Similar to ClusterConfiguration, the StaticClusterConfiguration resource is not required if the platform is being installed in an existing Kubernetes cluster.

- [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) — a set of resources containing configuration parameters for the platform's modules.

For example, when planning the cluster parameters, the following values were chosen:

- Pod and service subnets: `10.88.0.0/16` and `10.99.0.0/16`;
- Nodes are connected via the `192.168.1.0/24` subnet;
- Public wildcard domain for the cluster: `my-dvp-cluster.example.com`;
- Release channel: `early-access`.

{% offtopic title="Example config.yaml for installing the basic platform..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.88.0.0/16
serviceSubnetCIDR: 10.99.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
  - 192.168.1.0/24
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: "%s.my-dvp-cluster.example.com"
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    bundle: Default
    releaseChannel: EarlyAccess
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: true
  version: 2
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    # Enabling access to the Kubernetes API through Ingress.
    # https://deckhouse.io/modules/user-authn/configuration.html#parameters-publishapi
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
  name: cni-cilium
spec:
  enabled: true
  version: 1
  settings:
    tunnelMode: VXLAN
```

{% endofftopic %}

## Authentication in the container registry

Depending on the chosen edition, authentication in the container registry `registry.deckhouse.io` may be required:

- Authentication is not required for the Community Edition installation.

- For the Enterprise Edition and higher, authentication must be performed on the **installation machine** using the license key:

  ```shell
  docker login -u license-token registry.deckhouse.io
  ```

### Choosing the installer image

The installer runs as a container. The container image is selected based on the edition and release channel:

```shell
registry.deckhouse.io/deckhouse/<REVISION>/install:<RELEASE_CHANNEL>
```

Where:

- `<REVISION>` — the [edition](../../../about/editions.html) of the platform (e.g., `ee` for Enterprise Edition, `ce` for Community Edition, etc.)

- `<RELEASE_CHANNEL>` — the [release channel](../../../about/release-channels.html) of the platform in kebab-case:
  - `alpha` — for the *Alpha* release channel;
  - `beta` — for the *Beta* release channel;
  - `early-access` — for the *EarlyAccess* release channel;
  - `stable` — for the *Stable* release channel;
  - `rock-solid` — for the *RockSolid* release channel.

### Installation with cluster creation

1. Run the container, in which the configuration file and SSH keys for node access will be mounted.

   For example, to install the `CE` edition from the `Stable` release channel, use the image `registry.deckhouse.io/deckhouse/ce/install:stable`. In this case, the container can be started with the following command:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/config.yaml:/config.yaml" \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
   ```

1. Run the platform installer inside the container using the `dhctl bootstrap` command.

   For example, if the `dvpinstall` user was created during node preparation and the master node has the address `54.43.32.21`, the platform installation can be started with the following command:

   ```shell
   dhctl bootstrap \
     --ssh-host=54.43.32.21 \
     --ssh-user=dvpinstall --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --config=/config.yaml --ask-become-pass
   ```

If a password is required to run `sudo` on the server, provide it in response to the `[sudo] Password:` prompt.
The `--ask-become-pass` parameter can be omitted if `sudo` was configured to run without a password during node preparation.

Mounting the `$HOME/.ssh` directory gives the installer access to the SSH configuration, so in the `--ssh-host` parameter, you can specify the Host sections from the SSH configuration file.

### Installation in an existing cluster

1. Run the container, where the configuration file, keys for node access, and the file for connecting to the Kubernetes API will be mounted.

   For example, to install the `CE` edition from the `Stable` release channel, the image `registry.deckhouse.io/deckhouse/ce/install:stable` will be used, and the connection to the Kubernetes API will use the configuration file in `$HOME/.kube/config`.

   In this case, the container can be started with the following command:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/config.yaml:/config.yaml" \
     -v "$HOME/.kube/config:/kubeconfig" registry.deckhouse.io/deckhouse/ce/install:stable bash
   ```

1. Run the platform installer inside the container using the command `dhctl bootstrap-phase install-deckhouse`.

   If access to the existing cluster is configured on the **installation machine**, you can start the platform installation with the following command:

   ```shell
   dhctl bootstrap-phase install-deckhouse \
     --config=/config.yaml \
     --kubeconfig=/kubeconfig
   ```

### Installation completion

The installation time may range from 5 to 30 minutes, depending on the connection quality between the master node and the container registry.

{% offtopic title="Example output upon successful completion of the installation..." %}

```console
...

┌ Create deckhouse release for version v1.65.6
│ 🎉 Succeeded!
└ Create deckhouse release for version v1.65.6 (0.23 seconds)

┌ ⛵ ~ Bootstrap: Clear cache
│ ❗ ~ Next run of "dhctl bootstrap" will create a new Kubernetes cluster.
└ ⛵ ~ Bootstrap: Clear cache (0.00 seconds)

🎉 Deckhouse cluster was created successfully!
```

{% endofftopic %}

After the installation is successfully completed, you can exit the running container and proceed to [access configuration](access.html).

### Pre-Installation Checks

List of checks performed by the installer before starting platform installation:

1. General checks:
   - The values of the parameters [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) and [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) do not match.
   - The authentication data for the container image registry specified in the installation configuration is correct.
   - The host name meets the following requirements:
     - The length does not exceed 63 characters;
     - It consists only of lowercase letters;
     - It does not contain special characters (hyphens `-` and periods `.` are allowed, but they cannot be at the beginning or end of the name).
   - The server (VM) does not have a CRI (containerd) installed.
   - The host name must be unique within the cluster.

1. Checks for static and hybrid cluster installation:
   - Only one `--ssh-host` parameter is specified. For static cluster configuration, only one IP address can be provided for configuring the first master node.
   - SSH connection is possible using the specified authentication data.
   - SSH tunneling to the master node server (or VM) is possible.
   - The server (VM) meets the minimum requirements for setting up the master node.
   - Python is installed on the master node server (VM).
   - The container image registry is accessible through a proxy (if proxy settings are specified in the installation configuration).
   - Required installation ports are free on the master node server (VM) and the installer host.
   - DNS must resolve `localhost` to IP address 127.0.0.1.
   - The user has `sudo` privileges on the server (VM).

1. Checks for cloud cluster installation:
   - The configuration of the virtual machine for the master node meets the minimum requirements.

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

Example of using the preflight skip flag:

  ```shell
      dhctl bootstrap \
      --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
      --config=/config.yml \
      --preflight-skip-all-checks 
  ```

{% endofftopic %}
