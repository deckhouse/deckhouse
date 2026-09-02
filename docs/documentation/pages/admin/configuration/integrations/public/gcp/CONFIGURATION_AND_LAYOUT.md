---
title: Layouts and configuration in Google Cloud Platform
permalink: en/admin/integrations/public/gcp/configuration-and-layout-scheme.html
---

## Layouts

This section describes the possible node placement layouts in Google Cloud Platform (GCP) infrastructure
and the related configuration options.
The selected layout affects networking behavior, availability of public IP addresses, outgoing traffic routing,
and how nodes are accessed.

Deckhouse Kubernetes Platform (DKP) supports two layouts for deploying resources in GCP.

### Standard

- A dedicated VPC is created for the cluster along with [Cloud NAT](https://cloud.google.com/nat/docs/overview).
- Cluster nodes do not have public IP addresses by default.
- Public IP addresses can be assigned to static and master nodes:
  - In this case, One-to-One NAT is used to map the public IP address to the node's internal IP address
    (Cloud NAT will not be used in this scenario).
- If the master node does not have a public IP address, an additional instance with a public IP address
  (for example, a bastion host) is required for cluster installation and access.
- Peering can be configured between the cluster's VPC and other VPCs.

![Standard layout in GCP](../../../../images/cloud-provider-gcp/gcp-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10164&t=Qb5yyWumzPiTBtfL-0 --->

When a cluster is created, DKP creates the following firewall rules in the cluster VPC:

- `<prefix>-ssh-and-ping`: Allows incoming traffic over the `ICMP` and `TCP` (port 22) protocols to nodes with the `<prefix>` network tag from the CIDRs listed in [`sshAllowList`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-sshallowlist) (default `0.0.0.0/0`).
- `<prefix>-intercommunication`: Allows any traffic between nodes with the `<prefix>` network tag, as well as from the pod subnet ([`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr)).

Custom firewall rules are applied to nodes through additional network tags ([`additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags)):

- For master nodes, in the [`masterNodeGroup.instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags) parameter of the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource.
- For static nodes, in the [`nodeGroups[].instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-nodegroups-instanceclass-additionalnetworktags) parameter of the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource.
- For ephemeral nodes, in the [`spec.additionalNetworkTags`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-additionalnetworktags) parameter of the [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) resource.

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: Standard
standard:
  # Optional. Listed addresses will be used for
  # addressing Cloud NAT.
  cloudNATAddresses:
  - example-address-1
  - example-address-2
subnetworkCIDR: 10.0.0.0/24         # Required.
# Optional. A list of GCP VPC Networks used by Kubernetes VPC
# Network to connect over peering.
peeredVPCs:
- default
sshKey: "<SSH_PUBLIC_KEY>"  # Required.
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    disableExternalIP: false        # Optional. The master node has externalIP by default.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    disableExternalIP: true         # Optional. The nodes don't have externalIP by default.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: static
provider:
  region: europe-west4              # Required.
  serviceAccountJSON: |             # Required.
    {
      "type": "service_account",
      "project_id": "sandbox",
      "private_key_id": "98sdcj5e8c7asd98j4j3n9csakn",
      "private_key": "-----BEGIN PRIVATE KEY-----",
      "client_id": "342975658279248",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/k8s-test%40sandbox.iam.gserviceaccount.com"
    }
```

### WithoutNAT

- A dedicated VPC is created for the cluster.
  All cluster nodes are assigned public IP addresses.
- Peering can be configured between the cluster's VPC and other VPCs.

![WithoutNAT layout in GCP](../../../../images/cloud-provider-gcp/gcp-withoutnat.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10296&t=Qb5yyWumzPiTBtfL-0 --->

When a cluster is created, DKP creates the following firewall rules in the cluster VPC:

- `<prefix>-ssh-and-ping`: Allows incoming traffic over the `ICMP` and `TCP` (port 22) protocols to nodes with the `<prefix>` network tag from the CIDRs listed in [`sshAllowList`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-sshallowlist) (default `0.0.0.0/0`).
- `<prefix>-intercommunication`: Allows any traffic between nodes with the `<prefix>` network tag, as well as from the pod subnet ([`podSubnetCIDR`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetcidr)).

Custom firewall rules are applied to nodes through additional network tags ([`additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags)):

- For master nodes, in the [`masterNodeGroup.instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags) parameter of the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource.
- For static nodes, in the [`nodeGroups[].instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-nodegroups-instanceclass-additionalnetworktags) parameter of the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource.
- For ephemeral nodes, in the [`spec.additionalNetworkTags`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-additionalnetworktags) parameter of the [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) resource.

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
subnetworkCIDR: 10.0.0.0/24         # Required.
# Optional. A list of GCP VPC Networks used by Kubernetes VPC
# Network to connect over peering.
peeredVPCs:
- default
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: static
provider:
  region: europe-west4              # Required.
  serviceAccountJSON: |             # Required.
    {
      "type": "service_account",
      "project_id": "sandbox",
      "private_key_id": "98sdcj5e8c7asd98j4j3n9csakn",
      "private_key": "-----BEGIN PRIVATE KEY-----",
      "client_id": "342975658279248",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/k8s-test%40sandbox.iam.gserviceaccount.com"
    }
```

## Configuration

Integration with GCP is handled via the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource,
which describes the cloud cluster configuration in GCP
and is used by the cloud provider when the control plane is hosted in the cloud.
The responsible DKP module configures itself automatically based on the selected layout.

To update the configuration of a running cluster, run the following command:

```shell
d8 system edit provider-cluster-configuration
```

{% alert level="info" %}
After modifying node-related parameters, you must run the `dhctl converge` command for the changes to take effect.
{% endalert %}

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
sshKey: "<SSH_PUBLIC_KEY>"
subnetworkCIDR: 10.36.0.0/24
masterNodeGroup:
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
nodeGroups:
- name: static
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
    additionalNetworkTags:
    - tag1
    additionalLabels:
      kube-node: static
provider:
  region: europe-west3
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"
```

Machine provisioning and parameters are configured in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resource,
where the instance class for the node group is specified (the [`cloudInstances.classReference`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) parameter).
For GCP, the instance class is a [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) custom resource that defines the machine parameters.

DKP also automatically creates StorageClasses that cover all available disk types in GCP:

| Disk type | Replication | StorageClass name |
|---|---|---|
| standard | none | `pd-standard-not-replicated` |
| standard | regional | `pd-standard-replicated` |
| balanced | none | `pd-balanced-not-replicated` |
| balanced | regional | `pd-balanced-replicated` |
| ssd | none | `pd-ssd-not-replicated` |
| ssd | regional | `pd-ssd-replicated` |

You can exclude unnecessary StorageClasses by specifying them in the [`exclude`](/modules/cloud-provider-gcp/configuration.html#parameters-storageclass-exclude) parameter.

### Enabling nested virtualization

To run virtual machine workloads (e.g., KVM-based VMs) inside GCP instances, enable nested virtualization.

{% alert %}
Only specific machine types support nested virtualization. See [the GCP documentation](https://cloud.google.com/compute/docs/instances/nested-virtualization/overview#supported_machine_types) for the list of compatible types.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: vm-nodes
spec:
  machineType: n2-standard-8
  nestedVirtualization: true
```

### Adding additional disks

To attach additional disks to instances (for example, for LINSTOR, Ceph, NFS storage nodes, and similar solutions), specify them in the `additionalDisks` parameter:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: storage-nodes
spec:
  machineType: n1-standard-8
  additionalDisks:
  - sizeGb: 200
    type: pd-ssd
  - sizeGb: 500
    type: pd-standard
    autoDelete: true
```

### Adding CloudStatic nodes to the cluster

To add manually created virtual machines to the cluster as nodes, add a `Network Tag` matching the cluster prefix.

You can find the cluster prefix by running the following command:

```shell
d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
```

If the value is empty, get it from the deprecated [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of ClusterConfiguration (the prefix has moved to the ModuleConfig `global`):

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```
