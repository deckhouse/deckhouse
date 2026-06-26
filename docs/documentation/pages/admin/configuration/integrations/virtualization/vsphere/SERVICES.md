---
title: Integration with VMware vSphere services
permalink: en/admin/integrations/virtualization/vsphere/services.html
---

Deckhouse Kubernetes Platform (DKP) integrates with VMware vSphere infrastructure and uses [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resources
to describe the specifications of virtual machines created as part of the Kubernetes cluster.

Key features:

- Provisioning and removal of virtual machines via the vCenter API.
- Node placement across multiple clusters ([`zones`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones)) and datacenters ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region)).
- Use of VM templates with `cloud-init`.
- Support for networks with DHCP, static addressing, and additional interfaces.
- Storage management: provisioning root disks and PVCs based on Datastore or CNS disks.
- Support for incoming traffic load balancing:
  - Via external load balancers.
  - Via MetalLB (in BGP mode).

{% alert level="info" %}
To connect vSphere to a static cluster, see [Hybrid cluster with vSphere](../../hybrid/vsphere-hybrid.html).
{% endalert %}

## Node types

In a cloud cluster on vSphere, nodes are [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) and are managed via the [`masterNodeGroup`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup) and [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) sections of the `VsphereClusterConfiguration` resource.

## Virtual machine parameters

VM parameters are set in the `instanceClass` section of `VsphereClusterConfiguration`:

| Parameter | Description |
|-----------|-------------|
| [`numCPUs`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-numcpus) | Number of vCPUs |
| [`memory`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-memory) | RAM in MiB |
| [`template`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-template) | Path to the VM template relative to the Datacenter |
| [`datastore`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-datastore) | Path to the Datastore for the root disk |
| [`rootDiskSize`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-rootdisksize) | Root disk size in GiB (default 20) |
| [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork) | Primary network (port group) with the default route. Path relative to the Datacenter — see [Network parameters](layout.html#network-parameters) |
| [`additionalNetworks`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-additionalnetworks) | Additional network interfaces |
| [`resourcePool`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-resourcepool) | Resource Pool relative to the zone (vSphere Cluster). For CloudEphemeral, must exist beforehand — see [Network parameters](layout.html#resourcepool) |
| [`mainNetworkIPAddresses`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masterinstanceclass-mainnetworkipaddresses) | Static IP addresses instead of DHCP (only in `VsphereClusterConfiguration`) |
| [`runtimeOptions`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-runtimeoptions) | Additional VM parameters: CPU/memory shares, limits, nested virtualization |

Example `instanceClass` for a worker group:

```yaml
instanceClass:
  numCPUs: 4
  memory: 8192
  template: Templates/ubuntu-24.04
  datastore: lun10
  mainNetwork: net3-k8s
  rootDiskSize: 50
  additionalNetworks:
    - K8S_INTERNAL
  runtimeOptions:
    nestedHardwareVirtualization: false
```

{% alert %}
When using static IP addresses (`mainNetworkIPAddresses`), the OS image must have the `ens192` interface configured — see [Connection and authorization](authorization.html#virtual-machine-image-requirements).
{% endalert %}

### Node placement across zones

The `zones` list in a node group limits which vSphere Clusters VMs can be created in. Nodes are distributed across zones **in alphabetical order**: the first node goes to the zone with the smallest name, the second to the next, and so on. If there are more nodes than zones, distribution restarts from the beginning.

```yaml
nodeGroups:
- name: worker
  replicas: 4
  zones:
    - zone-a
    - zone-b
  instanceClass:
    # ...
```

In this example, nodes are placed: `zone-a`, `zone-b`, `zone-a`, `zone-b`.

## vSphere resource management

### General workflow

Node configuration changes in a cloud cluster on vSphere are performed in two steps:

1. Edit [`VsphereClusterConfiguration`](/modules/cloud-provider-vsphere/cluster_configuration.html):

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Apply changes via `dhctl converge` [in the DKP installer container](/products/kubernetes-platform/documentation/v1/installing/#installing) of the same edition and version as the cluster:

   ```shell
   dhctl converge \
     --ssh-host <MASTER-NODE-IP-ADDRESS> \
     --ssh-user <USERNAME> \
     --ssh-agent-private-keys /tmp/.ssh/<PRIVATE-SSH-KEY-NAME>
   ```

The `dhctl converge` command runs Terraform, which creates, modifies, or deletes virtual machines in vSphere, bootstraps new nodes, and registers them in the Kubernetes cluster.

Check Terraform state before applying:

```shell
dhctl terraform check \
  --ssh-host <MASTER-NODE-IP-ADDRESS> \
  --ssh-user <USERNAME> \
  --ssh-agent-private-keys /tmp/.ssh/<PRIVATE-SSH-KEY-NAME>
```

### Increasing the number of nodes

To add nodes to a `CloudPermanent` group:

1. Open the vSphere configuration:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Increase [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) in the required `nodeGroups` entry. For example, from 2 to 4:

   ```yaml
   nodeGroups:
   - name: worker
     replicas: 4
     zones:
     - zone-a
     - zone-b
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
   ```

1. Apply the configuration via `dhctl converge` (see [above](#general-workflow)).

1. Wait for completion and verify nodes:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=worker
   ```

   New virtual machines appear in vSphere Client in the folder specified in [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath).

### Adding a new node group

To create a new worker group (for example, `frontend`):

1. Add a group definition to `nodeGroups`:

   ```yaml
   nodeGroups:
   - name: frontend
     replicas: 2
     zones:
     - zone-a
     instanceClass:
       numCPUs: 2
       memory: 4096
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
     nodeTemplate:
       labels:
         node-role.deckhouse.io/frontend: ""
       taints:
       - effect: NoExecute
         key: dedicated.deckhouse.io
         value: frontend
   ```

   The [`nodeTemplate`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-nodetemplate) section sets Kubernetes labels and taints for nodes.

1. Apply the configuration via `dhctl converge`.

1. Verify that the NodeGroup object was created and nodes are `Ready`:

   ```shell
   d8 k get nodegroup frontend
   d8 k get nodes -l node.deckhouse.io/group=frontend
   ```

### Changing virtual machine parameters

You can change `instanceClass` parameters (CPU, RAM, template, datastore, networks) in the configuration and apply them via `dhctl converge`.

{% alert level="warning" %}
Changing hardware parameters (CPU, RAM, template) or datastore for **existing** nodes may require VM recreation. Recommended approach:

1. Increase `replicas` by 1 and run `dhctl converge` — a node with new parameters is created.
1. Move workloads off the old node (drain).
1. Decrease `replicas` and run `dhctl converge` — the extra node is removed.

For `rootDiskSize` changes only, Terraform enlarges the disk without recreating the VM if the new size is larger than the current one.
{% endalert %}

### Managing master nodes

Master nodes are configured in the [`masterNodeGroup`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup) section:

```yaml
masterNodeGroup:
  replicas: 3
  zones:
    - zone-a
    - zone-b
    - zone-c
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: Templates/ubuntu-24.04
    datastore: lun10
    mainNetwork: net3-k8s
```

{% alert level="warning" %}
The number of master nodes (`replicas`) must be **odd** to maintain etcd quorum. After changing `masterNodeGroup`, always run `dhctl converge`.
{% endalert %}

### Reducing the number of nodes

The [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) parameter defines the target number of virtual machines in a group.

{% alert level="warning" %}
Do not remove the group definition from the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section while `replicas` is greater than zero. Removing the group definition prematurely may desynchronize Terraform state with virtual machines in vSphere.
{% endalert %}

To reduce the number of nodes:

1. Open the configuration: `d8 system edit provider-cluster-configuration`.
1. Decrease `replicas` to the required value.
1. Run `dhctl converge`.
1. Verify the result:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<GROUP-NAME>
   ```

{% alert level="info" %}
Before reducing the node count, ensure that critical Pods are not running on nodes to be removed. If needed, perform a [drain](#drain-cordon-and-node-maintenance) manually before running `dhctl converge`.
{% endalert %}

### Completely removing a node group

Removing a `CloudPermanent` group is a two-stage process:

1. Set `replicas: 0` and run `dhctl converge`. Wait for all nodes and virtual machines to be removed.
1. Remove the group definition from `nodeGroups` and run `dhctl converge` again.

Step-by-step:

1. Open the configuration and set `replicas: 0` for the group to be removed. **Do not remove** the group definition at this stage.
1. Run `dhctl converge`.
1. Ensure no nodes remain:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<GROUP-NAME>
   ```

   Verify in vSphere Client that virtual machines have been removed.
1. Remove the group definition from `nodeGroups` and run `dhctl converge` again.
1. Verify that the NodeGroup was removed:

   ```shell
   d8 k get nodegroup <GROUP-NAME>
   ```

## Drain, cordon, and node maintenance

### Manual node decommissioning

For planned maintenance of a vSphere virtual machine (migration, hypervisor update, hardware replacement), cordon the node and evict Pods:

```shell
d8 k cordon <node_name>
d8 k drain <node_name> --ignore-daemonsets --delete-emptydir-data
```

After maintenance, return the node to service:

```shell
d8 k uncordon <node_name>
```

### Automatic drain during updates

For disruptive updates (containerd upgrade, kubelet upgrade, reboot), DKP can automatically drain the node. The mode is set in [`disruptions.approvalMode`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) of the `NodeGroup` resource:

| Mode | Behavior |
|------|----------|
| `Automatic` | Drain runs automatically before the update (default) |
| `Manual` | Manual approval required via `update.node.deckhouse.io/disruption-approved=` annotation |

To manually approve an update:

```shell
d8 k annotate node <node_name> update.node.deckhouse.io/disruption-approved=
```

Drain timeout is configured via [`nodeDrainTimeoutSecond`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodedraintimeoutsecond) in NodeGroup (default — 10 minutes).

During drain, the following annotations appear on the node:

| Annotation | Meaning |
|------------|---------|
| `update.node.deckhouse.io/draining` | Drain requested (value is the source, e.g. `bashible`) |
| `update.node.deckhouse.io/drained` | Drain completed |

For details, see [Node management basics](../../platform-scaling/node/node-management.html#disruptive-updates).

## Monitoring node and group status

### Checking Kubernetes nodes

```shell
# All cluster nodes
d8 k get nodes -o wide

# Nodes in a specific group
d8 k get nodes -l node.deckhouse.io/group=<GROUP-NAME>

# Detailed node info (addresses, taints, conditions)
d8 k describe node <node_name>
```

Cloud Controller Manager sets node addresses based on networks specified in [`externalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) and [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames).

### NodeGroup status

```shell
d8 k get nodegroup <GROUP-NAME> -o yaml
```

Main NodeGroup conditions:

| Condition | `True` means |
|-----------|--------------|
| `Ready` | The group has enough nodes in `Ready` state |
| `Updating` | At least one node is being updated |
| `WaitingForDisruptiveApproval` | Manual approval of a disruptive update is pending |
| `Scaling` | Scaling is in progress |
| `Error` | Node creation error (details in `status.error`) |

### Checking in vSphere

In vSphere Client, cluster virtual machines are placed in the folder specified by [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath). VM names follow the pattern `<prefix>-<group_name>-<index>`.

When troubleshooting nodes, check:

- VM state in vSphere Client (powered on, VMware Tools running);
- vCenter availability from the cluster;
- component logs:

  ```shell
  d8 k -n d8-cloud-provider-vsphere logs -l app=cloud-controller-manager --tail=50
  d8 k -n d8-cloud-instance-manager logs -l app=machine-controller-manager --tail=50
  ```

## Troubleshooting common issues

| Symptom | Possible cause | What to check |
|---------|----------------|---------------|
| `dhctl converge` fails | Insufficient vSphere privileges, resource shortage | [Privileges](authorization.html#list-of-required-privileges), Datastore free space, Resource Pool |
| Node in `NotReady` | Network or bootstrap issues | `cloud-init` logs on the VM, Kubernetes API availability |
| Incorrect node IP addresses | Misconfigured networks | `externalNetworkNames`, `internalNetworkNames`, port group mapping |
| VM created but not joining the cluster | SSH/bootstrap issues | SSH key in [`sshPublicKey`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-sshpublickey), network connectivity |
