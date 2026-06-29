---
title: Layouts and configuration in VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/layout.html
---

## Standard

The Standard layout is intended for deploying a cluster within the vSphere infrastructure
with full control over resources, networking, and storage.

Key features:

- Uses a vSphere Datacenter as a [`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region).
- Uses a vSphere Cluster as a [`zone`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones).
- Supports multiple zones and node placements across zones.
- Supports using different datastores for disks and volumes.
- Supports network connectivity including additional network isolation (for example, MetalLB + BGP).

![Standard layout in vSphere](../../../../images/cloud-provider-vsphere/vsphere-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11345&t=Qb5yyWumzPiTBtfL-0 --->

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  zones:
    - ru-central1-a
    - ru-central1-b
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
nodeGroups:
  - name: khm
    replicas: 1
    zones:
      - ru-central1-a
    instanceClass:
      numCPUs: 4
      memory: 8192
      template: dev/golden_image
      datastore: dev/lun_1
      mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
  - ru-central1-a
  - ru-central1-b
```

Required parameters for the [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) resource:

- `layout` — layout scheme (`Standard`);
- `provider` — vCenter connection parameters;
- `masterNodeGroup` — master node definition (`instanceClass` requires `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`);
- `region` — tag assigned to the Datacenter object;
- `zoneTagCategory` and `regionTagCategory` — tag categories used to identify regions and zones;
- `vmFolderPath` — path to the folder where cluster virtual machines will be placed;
- `sshPublicKey` — public SSH key used to access the nodes;
- `zones` — list of zones available for node placement.

{% alert level="info" %}
All nodes placed in different zones must have access to shared datastores with matching zone tags.
{% endalert %}

## Network parameters {#network-parameters}

Network-related settings for the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module are spread across [`VsphereClusterConfiguration`](/modules/cloud-provider-vsphere/cluster_configuration.html), [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), and [`ModuleConfig`](/modules/cloud-provider-vsphere/configuration.html). The table below summarizes parameter requirements and applicability by node type.

| Parameter | Where to set | CloudPermanent | CloudEphemeral | Purpose |
|-----------|--------------|----------------|----------------|---------|
| [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork) | `instanceClass` / `VsphereInstanceClass` | **Required** | **Required** (if omitted in `VsphereInstanceClass`, the master node value is used when available) | Port group for the VM primary NIC during provisioning |
| [`internalNetworkCIDR`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) | `VsphereClusterConfiguration` | **Required** if `additionalNetworks` is set on the master; otherwise not used | Not used | Static IP allocation for master nodes with additional NICs (Terraform) |
| [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) | `VsphereClusterConfiguration` / `ModuleConfig` | Optional | Optional | `vsphere-cloud-controller-manager`: `InternalIP` in `Node.status.addresses` |
| [`externalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) | `VsphereClusterConfiguration` / `ModuleConfig` | Optional | Optional | `vsphere-cloud-controller-manager`: `ExternalIP` in `Node.status.addresses` |
| [`resourcePool`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-resourcepool) | `instanceClass` / `VsphereInstanceClass` | Optional (DKP can create a nested pool) | Optional (must already exist if set) | VM placement in a Resource Pool |

`mainNetwork` selects the network used when creating a VM. `internalNetworkNames` and `externalNetworkNames` do not affect VM provisioning — they only affect how node IP addresses are published in the Kubernetes API after the VM is created.

### mainNetwork {#mainnetwork}

**Required** in `instanceClass`:

- for `masterNodeGroup` and `nodeGroups` in `VsphereClusterConfiguration` ([CloudPermanent](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) nodes, provisioned by Terraform);
- for [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) when provisioning [CloudEphemeral](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) nodes (provisioned by machine-controller-manager) — if omitted, the value from the master node configuration is used when available.

Specifies the port group for the VM's primary NIC (default route).

**Value format** — network path **relative to the Datacenter** (not the network display name alone and not the full inventory path from the vCenter root):

| Value | Description |
|-------|-------------|
| `net3-k8s` | Port group name at the root of the datacenter Networks section |
| `k8s-msk/test_187` | Port group `test_187` in folder `k8s-msk` |
| `"PROD NET"` | Port group name with a space — quote it in YAML |

```yaml
instanceClass:
  mainNetwork: "PROD NET"
  # or with a folder:
  # mainNetwork: "k8s-networks/PROD NET"
```

### CloudPermanent and CloudEphemeral nodes {#cloudpermanent-and-cloudephemeral-nodes}

[CloudPermanent](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) and [CloudEphemeral](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) nodes in vSphere are provisioned by different components — Terraform (`dhctl`) and [machine-controller-manager](https://github.com/gardener/machine-controller-manager), respectively.

Both accept a network path relative to the Datacenter, but the vSphere Inventory lookup mechanism differs. Because of this, network issues may manifest differently for different node types: for one type the network may be found correctly, while provisioning for the other may fail due to permissions, path, or lookup method.

For example, "network not found" errors with a valid master node configuration may appear only when creating ephemeral nodes — verify `mainNetwork` in `VsphereInstanceClass` and [`Network.Assign`](authorization.html#verifying-network-permissions-with-govc) permissions on the target network.

### internalNetworkCIDR

**Optional** `VsphereClusterConfiguration` parameter.

| Node type | Required? | Used by |
|-----------|-----------|---------|
| CloudPermanent (master with `additionalNetworks`) | **Yes** | Terraform — assigns IP addresses to master nodes from the specified subnet (starting at the tenth address: for `192.168.199.0/24`, from `192.168.199.10`) |
| CloudPermanent (master without `additionalNetworks`) | No | Not used |
| CloudPermanent (workers in `nodeGroups`) | No | Not used |
| CloudEphemeral | No | Not used |

### internalNetworkNames and externalNetworkNames {#internalnetworknames-and-externalnetworknames}

**Optional** parameters. Set in `VsphereClusterConfiguration` and/or in the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/configuration.html) module settings (`ModuleConfig`).

| Node type | Required? | Used by |
|-----------|-----------|---------|
| CloudPermanent | No | `vsphere-cloud-controller-manager` |
| CloudEphemeral | No | `vsphere-cloud-controller-manager` |

Used to populate `InternalIP` and `ExternalIP` in `Node.status.addresses`. Does not affect which network a VM is connected to during provisioning.

{% alert level="info" %}
Specify the **network name only** (port group) — without a path, as displayed in the VM network adapter properties in vSphere. This differs from the `mainNetwork` format.
{% endalert %}

Recommended when nodes have multiple network interfaces and you need to explicitly separate internal/external addresses for Kubernetes.

Example:

```yaml
internalNetworkNames:
  - K8S_INTERNAL
externalNetworkNames:
  - PUBLIC_NET
```

### Network permissions in vSphere {#network-permissions-in-vsphere}

The service account must have the [`Network.Assign`](authorization.html#list-of-required-privileges) privilege on each port group specified in `mainNetwork` (and `additionalNetworks` if used). With the [granular permission model](authorization.html#granular-permission-model), this privilege must be assigned on **each** target port group or inherited from a parent object.

Verify that the service account has the required permissions on the target network:

```shell
export GOVC_URL="https://<VCENTER_FQDN>/sdk"
export GOVC_USERNAME="<USERNAME@DOMAIN.LOCAL>"
export GOVC_PASSWORD="<PASSWORD>"
export GOVC_INSECURE=true

govc permissions.ls -r "/<Datacenter>/network/<NETWORK_NAME>"
```

Example for a network with a space in the name:

```shell
govc permissions.ls -r "/<Datacenter>/network/PROD NET"
```

The command output must show a role for the DKP account that includes the `Network.Assign` privilege. The `-r` (`--recursive`) flag displays permissions inherited from parent objects.

For path examples and more details, see [Verifying network permissions with govc](authorization.html#verifying-network-permissions-with-govc).

### resourcePool {#resourcepool}

**Optional** `instanceClass` parameter.

| Node type | Behavior |
|-----------|----------|
| **CloudPermanent** | With [`useNestedResourcePool`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-usenestedresourcepool): `true` (default), DKP automatically creates a nested resource pool in each zone. The `resourcePool` value in `instanceClass` overrides the default pool |
| **CloudEphemeral** | If `resourcePool` is explicitly set in `VsphereInstanceClass` or NodeGroup settings, the corresponding resource pool **must already exist** in vSphere — machine-controller-manager does not create Resource Pools automatically. VM provisioning fails if the pool at the specified path is not found |

{% alert level="warning" %}
If `resourcePool` is set in `VsphereInstanceClass` or NodeGroup settings, the Resource Pool must already exist in vSphere. Machine-controller-manager does not create Resource Pools automatically. If the specified Resource Pool is not found, MCM fails node provisioning with an error.

This is important when different NodeGroups need placement in different Resource Pools.
{% endalert %}

By default, ephemeral nodes in a cloud cluster use `resourcePoolPath` from `VsphereCloudDiscoveryData`, created during cluster deployment.

## Troubleshooting node provisioning {#troubleshooting-node-provisioning}

If node provisioning fails, check the following:

1. **machine-controller-manager logs** (for CloudEphemeral nodes):

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

1. **Service account permissions on the network** — verify with [`govc permissions.ls`](#network-permissions-in-vsphere) that the account has `Network.Assign` on the port group from `mainNetwork`.

1. **Network name/path in `mainNetwork`** — use the Datacenter-relative path format; quote names with spaces in YAML (for example, `mainNetwork: "PROD NET"`). See [mainNetwork](#mainnetwork).

1. **`resourcePool` existence** — if set in `VsphereInstanceClass`, verify that the Resource Pool already exists in vSphere.

1. **Permanent vs ephemeral provisioning** — CloudPermanent nodes are created by Terraform (`dhctl`), CloudEphemeral nodes by machine-controller-manager. A network issue may appear only for one node type even when `mainNetwork` looks correct for the other.

For additional diagnostics, see [Troubleshooting common issues](services.html#troubleshooting-common-issues).

## vSphere privileges

The full list of required privileges, role creation instructions, and granular permission model options are described in [Connection and authorization](authorization.html#list-of-required-privileges).
