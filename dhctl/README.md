---
title: Deckhouse CandI (Cluster and Infrastructure) 
permalink: /candi/dhctl.html
---
```text
========================================================================================
 _____             _     _                                ______                _ _____
(____ \           | |   | |                              / _____)              | (_____)
 _   \ \ ____ ____| |  _| | _   ___  _   _  ___  ____   | /      ____ ____   _ | |  _
| |   | / _  ) ___) | / ) || \ / _ \| | | |/___)/ _  )  | |     / _  |  _ \ / || | | |
| |__/ ( (/ ( (___| |< (| | | | |_| | |_| |___ ( (/ /   | \____( ( | | | | ( (_| |_| |_
|_____/ \____)____)_| \_)_| |_|\___/ \____(___/ \____)   \______)_||_|_| |_|\____(_____)
========================================================================================
```

An application for creating Kubernetes clusters and configuring their infrastructure.

Basic features:
* Terraform base infrastructure and initial master node in various clouds, e.g., AWS, YandexCloud, OpenStack.
* Install Kubernetes and other packages required for its work (ether in a cloud or bare metal).
* Install **Deckhouse** - Kubernetes cluster operator, which deploys cluster add-ons and manages their configurations.
* Create additional `master` nodes or `static` nodes, configure and delete them.
* Follow cloud infrastructure changes and converge a cluster to the desired state.

## Create Kubernetes cluster

### Preparations

The first step is setting up a host.
Only `Ubuntu 18.04`, `Ubuntu 20.04`, `Ubuntu 22.04`, `Centos 7`, `Centos 8`, `Centos 9`, `Debian 10`, `Debian 11`, `Debian 12` OS are supported.

* **Bare metal** - provide SSH access to the host and sudo access.
* **Cloud provider** - ensure that Deckhouse supports your cloud.
  If it is not, you can still create VMs by hand and follow bare metal installation instructions.

### Configuration

A configuration file is a YAML file with several sections.
* `ClusterConfiguration` - very basic Kubernetes cluster settings, e.g., networks, CRI, cloud provider, Kubernetes version.
* `InitConfiguration` - initial settings for Deckhouse installation that can be changed in the future.

Configuration example:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.27"
cri: "Containerd"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.deckhouse.io/fe
  registryDockerCfg: | # Base64-encoded section of docker.auths {"auths":{"registry.example.com":{"username":"oauth2","password":"token"}}}
    eyJhdXRocyI6eyJyZWdpc3RyeS5leGFtcGxlLmNvbSI6eyJ1c2VybmFtZSI6Im9hdXRoMiIsInBhc3N3b3JkIjoidG9rZW4ifX19Cg==
  releaseChannel: Stable
  configOverrides:
    global:
    nginxIngressEnabled: false
    flantIntegrationEnabled: false
```

#### Cloud cluster

To bootstrap a cluster in the cloud, you need to add one more section to the same config file:
* **${PROVIDER_NAME}ClusterConfiguration** - cloud provider-specific settings: API access, nodes network, settings and capacity.

Example for OpenStack based installation:

```yaml
...
---
apiVersion: deckhouse.io/v1
kind: OpenStackClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa publicsshkeyhere"
standard:
  internalNetworkCIDR: 192.168.199.0/24
  internalNetworkDNSServers:
  - 8.8.8.8
  - 4.2.2.2
  internalNetworkSecurity: true
  externalNetworkName: public
masterNodeGroup:
  replicas: 1
  instanceClass:
    flavorName: m1.large
    imageName: ubuntu-18-04-cloud-amd64
    rootDiskSize: 20
  volumeTypeMap:
    nova: "__DEFAULT__"
provider:
  authURL: https://cloud.example.com/v3/
  domainName: Default
  tenantName: xxx
  username: xxx
  password: xxx
  region: SomeRegion
```

### Bootstrap Kubernetes cluster

To bootstrap a cluster, it is required to pull a docker image with all the needed software.
For example, use a docker image from the Flant docker registry:

1. Pull a fresh Docker image for desired release channel (we picked the Alpha channel for an example)

   ```bash
   docker pull registry.deckhouse.io/deckhouse/fe/install:alpha
   ```

   > **Note!** It is required to have Deckhouse license key to download FE images.

2. Run docker container and connect the terminal session to it:

   > `config.yaml` - configuration file for the cluster bootstrap as described above.

     ```bash
     docker run -it \
       -v "$PWD/config.yaml:/config.yaml" \
       -v "$HOME/.ssh/:/tmp/.ssh/" \
       registry.deckhouse.io/fe/install:alpha \
       bash
     ```

     > macOS users do not need to mount the .ssh folder to the `/tmp`.
     > Because of Docker for MAc specific features it is more convenient to mount it to the `/root`.
3. Execute cluster bootstrap:

   ```bash
   dhctl bootstrap \
     --ssh-user=ubuntu \
     --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --config=/config.yaml 
   ```

### Create additional resources

During a bootstrap process, ready to work deckhouse controller will be installed in the cluster.
It is a cluster operator which extends the cluster's Kubernetes API with [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

It is possible to deploy Kubernetes resources after successful cluster creation by providing a path to the file
with manifests specifying the `--resources` flag for `bootstrap` command.

Example:

```yaml
---
apiVersion: deckhouse.io/v1
kind: OpenStackInstanceClass
metadata:
  name: worker
spec:
  flavorName: m1.large
  imageName: ubuntu-18-04-cloud-amd64
  mainNetwork: standard
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 2
    maxPerZone: 4
    classReference:
      kind: OpenStackInstanceClass
      name: worker
---
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  inlet: HostPort
  ingressClass: nginx
  hostPort:
    httpPort: 80
    httpsPort: 443
```

In this example, additional dynamically scaling node group and ingress controller will be added.

**The most significant feature** is that `kind: IngressNginxController` doesn't exist in the cluster after the installation process ends.
We have to wait for a cluster to become bootstrapped (create one non-master node).

In this case, Deckhouse-candi creates `OpenStackInstanceClass` and `NodeGroup` resources, then waits for the possibility to create `IngressNginxController`, and then deploy it.

> **Note!** You can run separate resources creating process by executing `bootstrap-phase create-resources`.

## Converge infrastructure

It is essential to be able to react to the changes in the cluster infrastructure.

With manual changes to the objects in cloud, it is possible to end with a cluster which is far from the desired configuration.
You cannot observe these changes, react to them. Every Kubernetes cluster become "unique".

1. To converge objects in a cloud to its desired state, dhctl has the `converge` command.
    During this command execution, dhctl will:
    * Connect to the Kubernetes cluster
    * Download terraform state for the base infrastructure
    * Sequentially call terraform apply for the downloaded state
    * Upload new states to the cluster
    * For master nodes and static nodes
        * If nodes quantity in the configuration is higher than we actually have, dhctl will create new nodes
        * If nodes quantity in the configuration is lower than we actually have, dhctl will delete excessive nodes
        * Sequentially call terraform apply for other nodes
    > if object is marked to be changed or deleted, dhctl will ask a user for confirmation.

    Example:

    ```bash
    dhctl converge \
      --ssh-host=8.8.8.8 \
      --ssh-user=ubuntu \
      --ssh-agent-private-keys=/tmp/.ssh/id_rsa
    ```

2. There are two commands to check the current state of objects in a cloud:
    * `dhctl terraform converge-exporter` - runs Prometheus exporter, which periodically checks the difference between
      objects and cloud and terraform state from secrets.
        > This command is used in the module `040-terraform-manager`
    * `dhctl terraform check` - executes the check once and returns report in ether YAML or JSON format.

## Destroy Kubernetes cluster

To destroy a Kubernetes cluster from a cloud, execute `destroy` command.
During execution, dhctl will:
* Connect to the cluster
* Delete Kubernetes resources bound to the cloud objects, e.g., services with type LoadBalancer, PV, PVC, and Machines (dhctl will wait until resources become deleted).
* Download terraform state for base infrastructure and nodes from the cluster
* Sequentially call terraform destroy for downloaded states

Execution example:

```bash
dhctl destroy \
  --ssh-host 8.8.8.8 \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=/tmp/.ssh/id_rsa
```
