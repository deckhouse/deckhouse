---
title: CandI subsystem (Cluster and Infrastructure)
permalink: /candi/
---

CandI subsystem consists of the following components:
* [**bashible**](/candi/bashible/) — framework for dynamic configuration and updates.
* kubeadm – TODO
* cloud-providers (layouts for terraform + extra bashible) – TODO
* **Deckhouse** modules:
  * [**control-plane-manager**](https://deckhouse.io/en/documentation/v1/modules/040-control-plane-manager/) — `control-plane` maintaining.
  * [**node-manager**](https://deckhouse.io/en/documentation/v1/modules/040-node-manager/) — swiss knife to create and update cloud and bare metal nodes.
  * **cloud-provider-** — modules to integrate different cloud with Deckhouse.
* Installer or **dhctl** — tool for creating the first master node, deploy `Deckhouse` and converging the cluster state.

## Installer

### Configuration

Configuration is a single YAML file, which contains several YAML documents separated by the `---`.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: 1.21
defaultCRI: "Containerd"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  imagesRepo: registry.example.com/deckhouse
  registryDockerCfg: edsfkslfklsdfkl==
  releaseChannel: Alpha
```

For validation and values defaulting, each configuration object has its OpenAPI specification.

| Kind                           | Description        |  OpenAPI path       |
| ------------------------------ | ------------------ | ------------------ |
| ClusterConfiguration           | Basic Kubernetes cluster configuration | [candi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/openapi/cluster_configuration.yaml) |
| InitConfiguration              | Required only for Deckhouse installation | [candi/openapi/init_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/openapi/init_configuration.yaml)|
| StaticClusterConfiguration     | Bare metal specific configuration | [candi/openapi/static_cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/openapi/static_cluster_configuration.yaml)|
| OpenStackClusterConfiguration  | OpenStack specific configuration | [candi/cloud-providers/openstack/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/cloud-providers/openstack/openapi/cluster_configuration.yaml) |
| AWSClusterConfiguration        | AWS specific configuration | [candi/cloud-providers/aws/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/cloud-providers/aws/openapi/cluster_configuration.yaml) |
| GCPClusterConfiguration        | GCP specific configuration | [candi/cloud-providers/gcp/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/cloud-providers/gcp/openapi/cluster_configuration.yaml) |
| vSphereClusterConfiguration    | vSphere specific configuration | [candi/cloud-providers/vsphere/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/cloud-providers/vsphere/openapi/cluster_configuration.yaml) |
| YandexClusterConfiguration     | Yandex.Cloud specific configuration | [candi/cloud-providers/yandex/openapi/openapi/cluster_configuration.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/cloud-providers/yandex/openapi/cluster_configuration.yaml) |
| BashibleTemplateData           | Bashible Bundle compiling settings (only for dhctl render bashible-bunble) | [candi/bashible/openapi.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/bashible/openapi.yaml) |
| KubeadmConfigTemplateData      | Kubeadm config compiling settings (only for dhctl render kubeadm-config) | [candi/control-plane-kubeadm/openapi.yaml](https://github.com/deckhouse/deckhouse/blob/main/candi/control-plane-kubeadm/openapi.yaml)|

### Bootstrap

Bootstrap process with [dhctl](../../dhctl/docs/README.md) consists of several stages:

#### Terraform

There are three variants of terraforming:
* `base-infrastructure` — creates basic cluster components: networks, routers, SSH key pairs, etc.
  * dhctl discovers through terraform [output](https://www.terraform.io/docs/configuration/outputs.html):
    * `cloud_discovery_data` — the information for the cloud provider module to work correctly, will be saved in the secret `d8-provider-cluster-configuration` in namespace `kube-system`.

* `master-node` — creates a master node.
  * dhctl discovers through terraform [output](https://www.terraform.io/docs/configuration/outputs.html):
    * `master_ip_address_for_ssh` — external master ip address to connect to the node.
    * `node_internal_ip_address` — internal address to bind control plane components.
    * `kubernetes_data_device_path` — device name for storing Kubernetes data (etcd and manifests).

* `static-node` — creates a static node.

> Terraform state will be saved in the secret in d8-system namespace after each terraform pipeline execution.

**Attention!!** dhctl do not use terraform for bare metal clusters, it is required to pass `--ssh-host` to connect instead.

#### Static-cluster

There is a special option for bare metal clusters, which is located in the separate configuration — StaticClusterConfiguration.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.0.0/24
```

`internalNetworkCIDRs` — addresses from these networks will be considered as "internal"

#### Preparations

* **SSH connection check**: dhctl will quite the bootstrap process if does not manage connect to the host.
* **Detect bashible bundle**: execute `/candi/bashible/detect_bundle.sh` to get a bashible bundle name from the host.
* **Execute bootstrap.sh and bootstrap-network.sh**: scripts to install basdic software (jq, curl) and st up the network.

**Attention!!** dhctl will check the ssh connection first.

#### Bashible Bundle

Bundle is a tar archive with required steps, bashbooster framework, and bashible entrypoint.

Bundle includes:
1. Rendered steps (you can read more about steps from the [bashible documentation](/candi/bashible/)).
2. Rendered kubeadm configuration file (you can read more about kubeadm from the [control-plane-kubeadm documentation](/candi//control-plane-kubeadm/)).
3. Archive all files.

After this, archive will be uploaded to the remote host by scp, unarchived and executed with the following command `/var/lib/bashible/bashible.sh --local`.

#### Install Deckhouse

dhctl does two things to connect to Kubernetes API:
* Executes `kubectl proxy --port=0` on the remote host.
* Opens SSH tunnel to the kubectl proxy process.

After successfully connection to the Kubernetes API, `dhctl` creates or updates:
* Cluster Role `cluster-admin`
* Service Account for `deckhouse`
* Cluster Role Binding for `cluster-admin` the `deckhouse` service account
* Secret with a docker registry credentials `deckhouse-registry`
* ConfigMap for `deckhouse`
* Deployment for `deckhouse`
* Secrets with configuration data
  * `d8-cluster-configuration`
  * `d8-provider-cluster-configuration`
* Secrets with terraform state
  * `d8-cluster-terraform-state`
  * `d8-node-terraform-state-.*`
  
After installation ends, `dhctl` will wait for the `deckhouse` pod to become `Ready`.
Readiness probe is working the way that deckhouse become ready only if there is no task to install or update a module.

The `Ready` state is a signal for `dhctl` to create the `NodeGroup` object for master nodes.

#### Create additional master or static nodes

On additional cluster nodes boostrap, dhctl make calls to the Kubernetes API.
* Creates desired NodeGroup objects
* Waits for Secrets with the cloud config for the particular node group
* Execute corresponding terraform pipeline (for master node or static node)
* Save state back to the Kubernetes cluster if successful

> dhctl waits for nodes in each NodeGroup to become Ready. Otherwise, creation process may ends with an error.

#### Create additional resources

User can provide a YAML file with additional resources by specifying a `--resource` flag for bootstrap process.
`dhctl` will sort them by their `apiGroup/kind`, wait for their registration in Kubernetes API, and deploy them.
> This process is described in detail in the [dhctl](../../dhctl/docs/README.md) documentation.
