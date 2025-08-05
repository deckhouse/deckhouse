---
title: Layouts and configuration
permalink: en/admin/integrations/virtualization/vcd/configuration-and-layout-scheme.html
lang: en
---

## Layouts

Deckhouse Kubernetes Platform supports one layout for deploying resources in VCD.

### Standard

![Standard layout in VCD](../../../../images/cloud-provider-vcd/vcd-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11247&t=IvETjbByf1MSQzcm-0 --->

Example layout configuration:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetwork: internal
    mainNetworkIPAddresses:
    - 192.168.199.2
```

## StandardWithNetwork

![StandardWithNetwork layout](../../../../images/cloud-provider-vcd/vcd-standardwithnetwork.png)

When using this layout, you must confirm the type of network virtualization platform with your administrator and specify it in the `edgeGateway.type` parameter. Two options are supported: `NSX-T` and `NSX-V`.

If the Edge Gateway is based on `NSX-T`, a DHCP server will be automatically enabled for the created node network. It will assign IP addresses starting from the 30th address in the subnet up to the penultimate one (before the broadcast address). The starting address of the DHCP pool can be changed using the `internalNetworkDHCPPoolStartAddress` parameter.

If `NSX-V` is used, DHCP must be configured manually. Otherwise, nodes waiting to obtain an IP address via DHCP will not be able to receive one.

{% alert level="warning" %}
It is not recommended to use dynamic addressing for the first master node together with `NSX-V`.
{% endalert %}

This layout assumes the automatic creation of the following NAT rules:

- **SNAT** — translation of internal node network addresses to the external address specified in `edgeGateway.externalIP`.
- **DNAT** — translation of the external address and port specified in `edgeGateway.externalIP` and `edgeGateway.externalPort` to the internal IP address of the first master node on port 22 (TCP) to provide administrative SSH access.

{% alert level="warning" %}
If the Edge Gateway is powered by `NSX-V`, you must specify the name and type of the network to which the rule will be bound in the `edgeGateway.NSX-V.externalNetworkName` and `edgeGateway.NSX-V.externalNetworkType` properties. Usually, this is the network connected to the Edge Gateway in the `Gateway Interfaces` section and having an external IP address.
{% endalert %}

It is also possible to create firewall rules using the `createDefaultFirewallRules` property.

{% alert level="warning" %}
If the Edge Gateway is powered by `NSX-T`, existing Edge Gateway rules will be overwritten. This option is intended for scenarios where only one cluster is deployed on the Edge Gateway.
{% endalert %}

The following rules will be created:

- Allow all outgoing traffic;
- Allow incoming TCP traffic on port 22 for SSH connections to cluster nodes;
- Allow all incoming ICMP traffic;
- Allow incoming TCP and UDP traffic on ports 30000–32767 for `NodePort` services.

Example configuration for the StandardWithNetwork layout using `NSX-T`:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
mainNetwork: internal
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-T"
  externalIP: 10.0.0.1
  externalPort: 10022
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

Example configuration for the StandardWithNetwork layout using `NSX-V`:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
mainNetwork: internal
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-V"
  externalIP: 10.0.0.1
  externalPort: 10022
  NSX-V:
    externalNetworkName: external
    externalNetworkType: ext
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

## Configuration

Integration is carried out using the VCDClusterConfiguration resource, which describes the configuration of the cloud cluster in VCD and is used by the virtualization system if the cluster’s control plane is hosted there. The DKP module responsible for integration is automatically configured based on the selected deployment layout.

To modify the configuration in a running cluster, execute the following command:

```shell
d8 -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

{% alert level="info" %}
After changing the node parameters, you must run the `dhctl converge` command for the changes to take effect.
{% endalert %}

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: VCDClusterConfiguration
sshPublicKey: "<SSH_PUBLIC_KEY>"
organization: My_Org
virtualDataCenter: My_Org
virtualApplicationName: Cloud
mainNetwork: internal
layout: Standard
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    sizingPolicy: 4cpu8ram
    rootDiskSizeGb: 20
    etcdDiskSizeGb: 20
    storageProfile: nvme
nodeGroups:
  - name: worker
    replicas: 1
    instanceClass:
      template: Org/Templates/ubuntu-focal-20.04
      sizingPolicy: 16cpu32ram
      storageProfile: ssd
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
  insecure: true
```

The number of nodes to be provisioned and their parameters are defined in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resource,
where you also specify the name of the instance class used for that group (the `cloudInstances.classReference` parameter).
For the VCD cloud provider, the instance class is a custom resource called [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass),
which contains the specific configuration of the VMs.

The following is the example configuration of VCDInstanceClass for ephemeral nodes of the VMware Cloud Director cloud provider.

### Example configuration of VCDInstanceClass

```yaml
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: test
spec:
  rootDiskSizeGb: 90
  sizingPolicy: payg-4-8
  storageProfile: SSD-dc1-pub1-cl1
  template: MyOrg/Linux/ubuntu2204-cloud-ova
```

### Storage

A StorageClass is automatically created for each Datastore and DatastoreCluster in the zone (or zones).

You can set the name of StorageClass that will be used in the cluster by default (the `default` parameter),
and filter out the unnecessary StorageClasses (the `exclude` parameter).

#### CSI

By default, the storage subsystem uses CNS volumes capable of on-the-fly resizing.
FCD volumes are also supported, but only in the legacy mode.
You can set this via the `compatibilityFlag` parameter.

#### Important information concerning the PVC size increase

Due to the [nature](https://github.com/kubernetes-csi/external-resizer/issues/44) of volume-resizer CSI and vSphere API,
you have to do the following after increasing the PVC size:

1. On the node where the Pod is located, run the `kubectl cordon <node_name>` command.
1. Delete the Pod.
1. Make sure that the resize was successful. The PVC object must *not have* the `Resizing` condition.
   > The `FileSystemResizePending` state is OK.
1. On the node where the Pod is located, run the `kubectl uncordon <node_name>` command.

### Environment requirements

* vSphere version required: `v7.0U2` ([required](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/v2.3.0/docs/book/features/volume_expansion.md#vsphere-csi-driver---volume-expansion) for the `Online volume expansion` to work).
* vCenter to which master nodes can connect to from within the cluster.
* Datacenter with the following components:
  1. VirtualMachine template.
     * VM image should use `Virtual machines with hardware version 15 or later` (required for online resize to work).
     * The following packages must be installed in the VM image: `open-vm-tools`, `cloud-init` and [`cloud-init-vmware-guestinfo`](https://github.com/vmware-archive/cloud-init-vmware-guestinfo#installation) (if the `cloud-init` version lower than 21.3 is used).
  2. The network must be available on all ESXi where VirtualMachines will be created.
  3. One or more Datastores connected to all ESXi where VirtualMachines will be created.
     * A tag from the tag category in `zoneTagCategory` (`k8s-zone` by default) **must be added** to Datastores. This tag will indicate the **zone**.  All Clusters of a specific zone must have access to all Datastores within the same zone.
  4. The cluster with the required ESXis.
     * A tag from the tag category in `zoneTagCategory` (`k8s-zone` by default) **must be added** to the Cluster. This tag will indicate the **zone**.
  5. Folder for VirtualMachines to be created.
     * An optional parameter. By default, the root vm folder is used.
  6. Create a role with the appropriate [set](#list-of-required-privileges) of privileges.
  7. Create a user and assign the above role to it.
* A tag from the tag category in `regionTagCategory` (`k8s-region` by default) **must be added** to the Datacenter. This tag will indicate the **region**.
