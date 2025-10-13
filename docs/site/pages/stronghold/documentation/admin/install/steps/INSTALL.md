---
title: "Platform installation"
permalink: en/stronghold/documentation/admin/install/steps/install.html
---

## Configuration Preparation

To install the platform, you need to prepare a YAML configuration file for installation and, if necessary, a YAML file for resources that need to be created after a successful installation of the platform.

### Installation Configuration File

The YAML installation configuration file includes parameters for several resources (manifests):

- [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) — initial parameters for the platform configuration.
  The platform will launch after installation with this configuration. This resource specifies parameters necessary for the platform to start and operate correctly, such as [component placement parameters](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys), the used [StorageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass), [container registry](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-registrydockercfg) access settings, the [template for DNS names](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate), and others.

- [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — general parameters of the cluster, such as control plane version, network parameters, CRI settings, etc.

  > You need to use the `ClusterConfiguration` resource only if you need to pre-deploy a Kubernetes cluster during the platform installation. `ClusterConfiguration` is not needed if the platform is installed in an existing Kubernetes cluster.

- [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration) — parameters for a Kubernetes cluster deployed on bare metal servers or virtual machines in unsupported clouds.
  > Similar to `ClusterConfiguration`, `StaticClusterConfiguration` is not needed if the platform is installed in an existing Kubernetes cluster.

- ModuleConfig — a set of resources containing configuration parameters for built-in platform modules.

  If the cluster is initially created with nodes designated for specific types of workloads (system nodes, monitoring nodes, etc.), it is recommended to explicitly specify the corresponding nodeSelector in the module configuration for modules using persistent storage volumes (e.g., for the `prometheus` module, this would be the [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector) parameter).

<!-- TODO: fix the manifests -->

{% offtopic title="Example Installation Configuration File..." %}

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
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  # Specify if using dedicated monitoring nodes.
  # settings:
  #   nodeSelector:
  #     node.deckhouse.io/group: monitoring
```

{% endofftopic %}

### Installation Resources File

An optional YAML file of installation resources contains Kubernetes resource manifests that the installer will apply after the successful installation of the platform. This file can be useful for additional configuration of the cluster post-installation: deploying an Ingress controller, creating additional node groups, configuring resources, setting permissions, users, etc.

**Attention!** You cannot use ModuleConfig for **built-in** modules in the installation resources file. Use the [configuration file](#installation-configuration-file) for configuring built-in modules.

{% offtopic title="Example Installation Resources File..." %}

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1apiVersion: deckhouse.io/v1
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
  settings:
    modules:
      publicDomainTemplate: "%s.example.com"
      https:
        certManager:
          clusterIssuerName: selfsigned
        mode: CertManager
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
      dexCAMode: FromIngressSecret
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: stronghold
spec:
  enabled: true
  version: 1
  settings:
    management:
      mode: Automatic
      administrators:
      - type: Group
        name: admins
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: secrets-store-integration
spec:
  enabled: true
  version: 1
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  inlet: HostPort
  enableIstioSidecar: true
  ingressClass: nginx
  hostPort:
    httpPort: 80
    httpsPort: 443
  nodeSelector:
    node-role.kubernetes.io/master: ''
  tolerations:
    - effect: NoSchedule
      operator: Exists
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
kind: Group
metadata:
  name: admins
spec:
  name: admins
  members:
  - kind: User
    name: admin
```

{% endofftopic %}

<!-- TODO: The next section seems excessive. -->

### Post-Bootstrap Script

After a successful platform installation, the installer can run a script on one of the master nodes. This script can be used for additional setup, collecting setup information, etc.

You can specify a post-bootstrap script with the `--post-bootstrap-script-path` parameter during the installation run (see below).

{% offtopic title="Example of a Script Displaying the Load Balancer's IP Address..." %}
Example of a script that displays the load balancer's IP address after the cluster deployment in the cloud and the platform installation:

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
  if lb_ip="$(d8 k -n d8-ingress-nginx get svc "${INGRESS_NAME}-load-balancer" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"; then
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

## Platform Installation

> When installing a platform edition other than the [Community Edition](../../../about/editions.html) from the official container registry `registry.deckhouse.io`, you need to authenticate with a license key beforehand:
>
> ```shell
> docker login -u license-token registry.deckhouse.io
> ```

In general, the command to run the installer container from the platform’s public container registry is as follows:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

where:

- `<DECKHOUSE_REVISION>` — [edition](../../../about/editions.html) of the platform (e.g., `ee` for Enterprise Edition, `ce` for Community Edition, etc.)
- `<MOUNT_OPTIONS>` — options for mounting files into the installer container, such as:
  - SSH access keys;
  - configuration file;
  - resources file, etc.
- `<RELEASE_CHANNEL>` — [release channel](../../../about/release-channels.html) of the platform in kebab-case. It should match the one set in `config.yaml`:
  - `alpha` — for the *Alpha* release channel;
  - `beta` — for the *Beta* release channel;
  - `early-access` — for the *Early Access* release channel;
  - `stable` — for the *Stable* release channel;
  - `rock-solid` — for the *Rock Solid* release channel.

Example of running the installer container for the CE edition:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/resources.yaml:/resources.yaml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

The platform installation is initiated in the installer container using the `dhctl` command:

- Use the `dhctl bootstrap` command to start the platform installation with cluster deployment (this applies to all cases except installation in an existing cluster).
- Use the `dhctl bootstrap-phase install-deckhouse` command to install the platform in an existing cluster.

> For help on the parameters, run `dhctl bootstrap -h`.

Example of starting a platform installation with cluster deployment in the cloud:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yaml --config=/resources.yaml
```

where:

- `/config.yaml` — installation configuration file;
- `/resources.yaml` — resource manifests file;
- `<SSH_USER>` — user on the server for SSH connection;
- `--ssh-agent-private-keys` — file containing the private SSH key for the connection.

Next, connect to the master node via SSH (the master node's IP address is provided by the installer at the end of the installation):

```bash
ssh <USER_NAME>@<MASTER_IP>
```

Starting the Ingress controller after the platform installation may take some time. Ensure the Ingress controller is running before proceeding:

```bash
d8 k -n d8-ingress-nginx get po
```

Wait until the Pods reach the status `Ready`.

Also, wait for the load balancer's readiness:

```bash
d8 k -n d8-ingress-nginx get svc nginx-load-balancer
```

The `EXTERNAL-IP` should be populated with a public IP address or DNS name.

## DNS Configuration

To access the web interfaces of platform components, you need to:

1. Set up DNS.
2. Specify a DNS name template in the platform parameters.

The DNS name template is used for configuring Ingress resources of system applications. For example, the Grafana interface is assigned the name `grafana`. Thus, for the template `%s.kube.company.my`, Grafana will be accessible at `grafana.kube.company.my`, etc.

To simplify the setup, the `sslip.io` service will be used.

On the master node, execute the following command to obtain the load balancer’s IP address and configure the DNS name template for platform services to use `sslip.io`:

```bash
BALANCER_IP=$(d8 k -n d8-ingress-nginx get svc nginx-load-balancer -o json | jq -r '.status.loadBalancer.ingress[0].ip') && \
echo "Balancer IP is '${BALANCER_IP}'." && d8 k patch mc global --type merge \
  -p "{\"spec\": {\"settings\":{\"modules\":{\"publicDomainTemplate\":\"%s.${BALANCER_IP}.sslip.io\"}}}}" && echo && \
echo "Domain template is '$(d8 k get mc global -o=jsonpath='{.spec.settings.modules.publicDomainTemplate}')'."
```

The command will also display the installed DNS name template. Example output:

```bash
Balancer IP is '1.2.3.4'.
moduleconfig.deckhouse.io/global patched
Domain template is '%s.1.2.3.4.sslip.io'.
```
