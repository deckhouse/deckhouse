---
title: "Cloud provider — VMware vSphere: examples"
---

This page collects common configuration scenarios for the VMware vSphere cloud provider.
Examples are ordered from simple to complex.
For a full description of the parameters, refer to [Configuration](configuration.html) and [Custom Resources](cr.html).

## Creating a node group

An instance class describes the virtual machine parameters, while a node group defines their number and placement zones.
The example creates a group of two nodes, each allocated two vCPUs and 4096 MiB of memory.

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: worker
spec:
  numCPUs: 2
  memory: 4096
  rootDiskSize: 40
  template: <TEMPLATE_PATH>
  mainNetwork: <NETWORK_PATH>
  datastore: <DATASTORE_PATH>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: VsphereInstanceClass
      name: worker
    minPerZone: 2
    maxPerZone: 2
    zones:
      - <ZONE_TAG_NAME>
```

Placeholder values:

- `<TEMPLATE_PATH>`: Path to the virtual machine template relative to the Datacenter, for example `dev/golden_image`.
- `<NETWORK_PATH>`: Path to the network relative to the Datacenter that the primary node interface connects to.
- `<DATASTORE_PATH>`: Path to the Datastore relative to the Datacenter, for example `dev/lun_1`.
- `<ZONE_TAG_NAME>`: Name of the zone tag assigned to the Cluster object.

The root disk size is set in GiB, and the memory size is set in MiB.

After the manifest is applied, Deckhouse Kubernetes Platform (DKP) creates the virtual machines in vSphere.
Wait for the nodes to reach the `Ready` state:

```shell
d8 k get nodes -l node.deckhouse.io/group=worker
```

## Connecting to vCenter in an existing cluster

In a cluster created without the installer, vCenter connection parameters are set in ModuleConfig.
The example verifies the vCenter TLS certificate against the certificate chain of an enterprise certificate authority.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    host: <VCENTER_FQDN>
    username: <USERNAME>@<DOMAIN>
    password: <PASSWORD>
    caBundle: |
      -----BEGIN CERTIFICATE-----
      <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
      -----END CERTIFICATE-----
    vmFolderPath: <VM_FOLDER_PATH>
    regionTagCategory: k8s-region
    zoneTagCategory: k8s-zone
    region: <REGION_TAG_NAME>
    zones:
      - <ZONE_TAG_NAME>
    internalNetworkNames:
      - <PORT_GROUP_NAME>
    sshKeys:
      - <SSH_PUBLIC_KEY>
```

Placeholder values:

- `<VCENTER_FQDN>`: vCenter address.
- `<USERNAME>@<DOMAIN>`: Username together with the domain, for example `deckhouse@vsphere.local`.
- `<VM_FOLDER_PATH>`: Directory where the virtual machines are created.
- `<REGION_TAG_NAME>`: Name of the region tag assigned to the Datacenter object.
- `<PORT_GROUP_NAME>`: Network name without the full path, used to determine the internal node address.

Check that the module has reached the `Ready` state:

```shell
d8 k get module cloud-provider-vsphere -o wide
```

## Limiting the set of StorageClasses

DKP creates a StorageClass for every tagged Datastore, and with SPBM storage policies configured, also for every combination of a Datastore and a policy.
The example excludes the StorageClasses of slow storage from the cluster so that developers do not order volumes on them.

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
        - slow-lun103
        - ".*-lun101-.*"
```

The `exclude` parameter takes StorageClass names or regular expressions for them.
The expression is matched against the Datastore name, so an exclusion removes both the base StorageClass and all StorageClasses with storage policies for that Datastore.

Make sure the unnecessary StorageClasses are gone from the cluster:

```shell
d8 k get storageclass
```

To set the default StorageClass, use the [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass) global parameter.

## Nodes with multiple networks and resource limits

Virtual machines can connect to additional networks and have their resource consumption limited on the vSphere side.
In the example, the node connects to a storage network, and its CPU consumption is limited to 4000 MHz.

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: storage-worker
spec:
  numCPUs: 8
  memory: 16384
  rootDiskSize: 60
  template: <TEMPLATE_PATH>
  mainNetwork: <NETWORK_PATH>
  additionalNetworks:
    - <STORAGE_NETWORK_PATH>
  datastore: <DATASTORE_PATH>
  runtimeOptions:
    cpuLimit: 4000
    memoryReservation: 100
```

Placeholder values:

- `<STORAGE_NETWORK_PATH>`: Path to the additional network relative to the Datacenter.

The `cpuLimit` parameter sets the CPU consumption limit in MHz.
The `memoryReservation` parameter reserves memory as a percentage of the `memory` value, and defaults to 80.

{% alert level="warning" %}
Setting `cpuLimit` too low slows the node down.
{% endalert %}

## Load balancing through NSX-T

If NSX-T is deployed in the infrastructure, DKP can order load balancers in it for services of the LoadBalancer type.
The example sets the default address pool and verifies the NSX-T TLS certificate.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    nsxt:
      host: <NSXT_HOST>
      user: <NSXT_USERNAME>
      password: <NSXT_PASSWORD>
      defaultIpPoolName: <IP_POOL_NAME>
      tier1GatewayPath: <TIER1_GATEWAY_PATH>
      caBundle: |
        -----BEGIN CERTIFICATE-----
        <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
        -----END CERTIFICATE-----
```

Placeholder values:

- `<IP_POOL_NAME>`: Name of the address pool for services without the `loadbalancer.vmware.io/class` annotation.
- `<TIER1_GATEWAY_PATH>`: Path to the Tier-1 gateway where load balancers are created.

To direct a service to a different address pool, describe a load balancer class in the `nsxt.loadBalancerClass` parameter and set it in the `loadbalancer.vmware.io/class` annotation of the service.
