---
title: Networking, storage, and load balancing in VMware vSphere
permalink: en/admin/integrations/virtualization/vsphere/storage.html
---

## Networking

Cluster nodes connect to the network whose path is set in the [`mainNetwork`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetwork) parameter. Through this network, nodes reach vCenter, the container image registry, and each other.

The network must meet the following requirements:

- It is available on every ESXi host where virtual machines are created.
- It provides DHCP unless node addresses are set statically.
- vCenter and the container image registry are reachable from it.

### Node addressing

By default, nodes get their addresses over DHCP. For nodes created by the installer, addresses are set statically in the [`mainNetworkIPAddresses`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetworkipaddresses) parameter. Static addressing is not supported for the nodes ordered through the [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) resource, such nodes get their addresses over DHCP.

Addresses are assigned to the nodes of a group in order, so provide as many addresses as there are nodes in the group. If there are fewer addresses, the list is reused and the same address goes to several nodes.

An example of static addressing for three master nodes:

```yaml
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: k8s-msk-178
    mainNetworkIPAddresses:
    - address: 10.1.14.20/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.21/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.22/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
```

### Multiple networks

Secondary interfaces of virtual machines connect to the networks listed in the [`additionalNetworks`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-additionalnetworks) parameter. For example, an additional interface is used to connect the network for BGP traffic.

When there are several networks, specify which of them the platform treats as internal and which as external. Based on the network names from the [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) and [`externalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) parameters, the `vsphere-cloud-controller-manager` component sets the InternalIP and ExternalIP addresses in the Node object. These parameters take the name of the network, not its path.

The [`internalNetworkCIDR`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) parameter sets the subnet that master nodes get their addresses from in the internal network. Addresses are allocated starting with the tenth one. The parameter is required if the configuration contains the [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) section.

An example of a configuration with two networks:

```yaml
externalNetworkNames:
- net3-k8s
internalNetworkNames:
- K8S_3
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: net3-k8s
    additionalNetworks:
    - K8S_3
```

## Storage

The following storage types are used in VMware vSphere for cluster data:

- **Datastores**: Used to store the root disks of virtual machines;
- **CNS disks (Container Native Storage)**: Used for automatic creation of PersistentVolumes via CSI.

Deckhouse Kubernetes Platform (DKP) automatically creates a StorageClass for each Datastore tagged with a zone tag.

To keep the platform from creating a StorageClass for particular Datastores, list them in the [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude) parameter. It takes a list of names or regular expressions, each of which must match the Datastore name in full. A partial match is not taken into account. For example, for the `vsanDatastore` Datastore, the `vsan` expression excludes no StorageClass, while `vsan.*` excludes all of them.

Example configuration using ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    storageClass:
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

To set the default StorageClass, use the [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) global parameter.

### Datastore configuration

The platform creates a StorageClass only for the Datastores that have the region and zone tags assigned. A Datastore without tags is invisible to the platform, and a PersistentVolume cannot be provisioned on it. A Datastore is given the same region tag as the Datacenter and the same zone tag as the Cluster whose nodes use that storage.

{% alert %}
You can also tag **Datastore** objects through the **VMware vSphere Client** — follow [Datastore configuration](authorization.html#configuring-datastore-in-vsphere-client) in the connection and authorization guide. The steps below use **`govc` only**.
{% endalert %}

{% alert level="warning" %}
For dynamic `PersistentVolume` provisioning, a `Datastore` must be available on **each** ESXi host (shared datastore).
{% endalert %}

Assign the tags. In the example below, two Datastores are tagged for different zones of the same region:

```shell
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>

govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
```

### SPBM storage policies

If storage policies (SPBM, Storage Policy Based Management) are configured in vSphere, DKP discovers them and additionally creates a StorageClass for each combination of a Datastore and a policy. When a PersistentVolume is provisioned through such a StorageClass, vSphere applies the corresponding policy to the volume.

The StorageClass name combines the Datastore name and the policy name. DKP converts it to lowercase, replaces spaces with hyphens, and removes the remaining characters except hyphens and dots. For example, the `lun_1` Datastore and the `Gold Policy` policy produce the `lun1-gold-policy` StorageClass.

For DKP to discover the policies, the vSphere account needs the `StorageProfile.View` privilege. It is part of the [list of required privileges](layout.html#list-of-required-privileges), and creating a role is covered in the [Creating and assigning a role in vSphere Client](authorization.html#creating-and-assigning-a-role-in-vsphere-client) section.

Policies apply to volumes in any scenario where the [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) module runs, including a hybrid cluster. You cannot pick a policy for an individual StorageClass, since the StorageClass set is generated automatically.

Limitations:

- For DatastoreCluster objects, neither base StorageClasses nor StorageClasses with policies are created. Such StorageClasses are created only in the legacy mode with FCD volumes, which is enabled by the [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag) parameter.
- The [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude) parameter is matched against the Datastore name converted by the rules described above. An exclusion removes both the base StorageClass and the StorageClasses with policies for that Datastore. The parameter does not exclude an individual StorageClass with a policy by its own name.

### Storage policy for node disks

Disks of the virtual machines created by the installer get their storage policy from the [`storagePolicyID`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) parameter of the VsphereClusterConfiguration resource. The parameter takes the ID of an SPBM policy.

The parameter applies to master nodes and to static nodes created by the installer. Nodes ordered through a VsphereInstanceClass do not get the policy from this parameter.

The policy ID can be viewed in vSphere Client. Open "Menu" → "Policies and Profiles" → "VM Storage Policies" and click the policy name. The browser address bar then contains the `lvSelectedItemId` parameter, where the ID is placed between `PbmRequirementStorageProfile:` and the first `%` character.

![List of storage policies](../../../../images/cloud-provider-vsphere/storage-policy-setup/vm-storage-policies.png)

The `govc` command also prints the policy ID:

```shell
govc storage.policy.ls "<POLICY_NAME>"
```

Replace `<POLICY_NAME>` with the policy name as shown in the "VM Storage Policies" list, for example `vSAN Default Storage Policy`.

### CSI operation mode

By default, the storage subsystem uses CNS disks that support resizing without detaching the volume from the node (online resize). The legacy mode with FCD disks is also supported, in which resizing without detaching the volume is unavailable. The mode is selected by the [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag) parameter.

### Expanding a PersistentVolumeClaim

The platform supports resizing a PersistentVolume without detaching it from the node (online resize), starting with vSphere 7.0U2.

To expand a volume, change the requested size in the PersistentVolumeClaim:

```shell
d8 k -n <NAMESPACE> patch pvc <PVC_NAME> -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
```

No additional actions are required. DKP expands the volume in vSphere, and then kubelet expands the file system on the node the volume is attached to. The workload is not restarted.

While the expansion is in progress, the PersistentVolumeClaim status contains the `Resizing` and `FileSystemResizePending` conditions. After kubelet expands the file system, both conditions are removed and the `status.capacity` field contains the new size. The command below prints the current volume size and the list of conditions:

```shell
d8 k -n <NAMESPACE> get pvc <PVC_NAME> \
  -o jsonpath='{.status.capacity.storage}{"\n"}{range .status.conditions[*]}{.type}={.status} {end}'
```

An example of the output for a volume whose expansion has finished. The first line is the size, and the second line is empty because no conditions are left in the status:

```console
2Gi

```

If the `Resizing` condition remains in the status, expanding without detaching the volume is unavailable in this configuration, for example in the legacy mode with FCD volumes. The limitation is described in an [external-resizer issue](https://github.com/kubernetes-csi/external-resizer/issues/44). To expand such a volume, detach it from the node:

1. Prevent new workloads from being scheduled on the node the volume is attached to:

   ```shell
   d8 k cordon <NODE_NAME>
   ```

   Replace `<NODE_NAME>` with the name of the node that runs the workload using the PersistentVolumeClaim.

1. Delete the workload that uses the PersistentVolumeClaim so that the volume is detached from the node.

1. Wait until the `Resizing` condition is removed from the PersistentVolumeClaim status.

1. Allow scheduling on the node again:

   ```shell
   d8 k uncordon <NODE_NAME>
   ```

### Viewing volumes in vSphere Client

Volumes provisioned through CSI are shown in vSphere Client. Open "Menu" → "Inventory" → "Hosts and Clusters", select a Cluster object, go to the "Monitor" tab, and choose "Container Volumes" under "Cloud Native Storage". For every volume, the list shows the name, labels, Datastore, storage policy compliance ("Compliance Status"), availability ("Health Status"), and size.

![List of CNS volumes](../../../../images/cloud-provider-vsphere/cns-volumes/container-volumes.png)

The volume name in vSphere matches the PersistentVolume name in the cluster.

The icon to the left of the volume name opens the details panel. The "Kubernetes objects" tab shows the namespace, the PersistentVolumeClaim name and labels, and the workload that uses the volume.

![CNS volume details](../../../../images/cloud-provider-vsphere/cns-volumes/container-volume-details.png)

## Load balancing

Inbound traffic is balanced in one of three ways.

1. **Through an external load balancer.** A load balancer that already exists in the infrastructure directs traffic to the cluster frontend nodes. All that is required from vSphere is a network in which the frontend nodes are reachable by the load balancer. In the cluster, the traffic is accepted by the Ingress controller, which is covered in the [ALB with Ingress NGINX Controller](../../../configuration/network/ingress/alb/nginx.html) section.

1. **Through MetalLB.** This way suits an environment without an external load balancer. MetalLB assigns addresses to services of the LoadBalancer type and works in the L2 and BGP modes. The network requirements, the editions the modes are available in, and the configuration are covered in the [Load balancing with MetalLB](../../../configuration/network/ingress/nlb/metallb.html) section. On the vSphere side, the BGP mode requires the following:

   - A dedicated network in which the frontend nodes exchange traffic with the BGP routers.
   - A second network interface on the frontend nodes, connected to that network by the [`additionalNetworks`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-additionalnetworks) parameter.
   - DHCP in that network. The platform sets an address on the additional interface only for master nodes, from the [`internalNetworkCIDR`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) subnet starting with the tenth address. For the nodes of a node group, the platform does not assign an address in that network.
   - IP addresses of the BGP routers, the autonomous system number (ASN) of the routers and the ASN of the cluster, and the range of addresses that the cluster announces.

1. **Through NSX-T.** If NSX-T is deployed in the infrastructure, `cloud-controller-manager` orders a load balancer in it for every service of the LoadBalancer type. The address pool name, the Tier-1 gateway path, and the credentials are set in the [`nsxt`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt) section.
