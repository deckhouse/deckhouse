---
title: Layouts and configuration
permalink: en/admin/integrations/virtualization/vcd/configuration-and-layout-scheme.html
---

## Layouts

Deckhouse Kubernetes Platform supports two layouts for deploying resources in VCD.

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

### WithNAT

![WithNAT layout](../../../../images/cloud-provider-vcd/vcd-withnat.png)

When using this layout, you must confirm the type of network virtualization platform with your administrator and specify it in the [`edgeGateway.type`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-type) parameter. Two options are supported: `NSX-T` and `NSX-V`.

To ensure administrative access to the cluster nodes, a bastion is deployed. The parameters for its configuration are described in [the `bastion` section](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-bastion).

If the Edge Gateway is based on `NSX-T`, a DHCP server will be automatically enabled for the created node network. It will assign IP addresses starting from the 30th address in the subnet up to the penultimate one (before the broadcast address). The starting address of the DHCP pool can be changed using the [`internalNetworkDHCPPoolStartAddress`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-internalnetworkdhcppoolstartaddress) parameter.

If `NSX-V` is used, DHCP must be configured manually. Otherwise, nodes waiting to obtain an IP address via DHCP will not be able to receive one.

{% alert level="warning" %}
It is not recommended to use dynamic addressing for the first master node together with `NSX-V`.
{% endalert %}

This layout assumes the automatic creation of the following NAT rules:

- **SNAT** — translation of internal node network addresses to the external address specified in [`edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip).
- **DNAT** — translating the external address and port, specified in the [`edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip) and [`edgeGateway.externalPort`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalport) properties, respectively, to the internal address of the bastion instance on port 22 using the TCP protocol for administrative access to the nodes via SSH.

{% alert level="warning" %}
If the Edge Gateway is powered by `NSX-V`, you must specify the name and type of the network to which the rule will be bound in the [`edgeGateway.NSX-V.externalNetworkName`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworkname) and [`edgeGateway.NSX-V.externalNetworkType`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworktype) properties. Usually, this is the network connected to the Edge Gateway in the `Gateway Interfaces` section and having an external IP address.
{% endalert %}

It is also possible to create firewall rules using the [`createDefaultFirewallRules`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-createdefaultfirewallrules) property.

{% alert level="warning" %}
If the Edge Gateway is powered by `NSX-T`, existing Edge Gateway rules will be overwritten. This option is intended for scenarios where only one cluster is deployed on the Edge Gateway.
{% endalert %}

The following rules will be created:

- Allow all outgoing traffic;
- Allow incoming TCP traffic on port 22 for SSH connections to cluster nodes;
- Allow all incoming ICMP traffic;
- Allow incoming TCP and UDP traffic on ports 30000–32767 for `NodePort` services.

Example configuration for the WithNAT layout using `NSX-T`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: WithNAT
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
internalNetworkDNSServers:
  - 77.88.8.8
  - 1.1.1.1
mainNetwork: internal
bastion:
  instanceClass:
    rootDiskSizeGb: 30
    sizingPolicy: 2cpu1mem
    template: "catalog/Ubuntu 22.04 Server"
    storageProfile: Fast vHDD
    mainNetworkIPAddress: 10.1.4.10
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-T"
  externalIP: 10.0.0.1
  externalPort: 10022
createDefaultFirewallRules: false
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

Example configuration for the WithNAT layout using `NSX-V`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: WithNAT
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
internalNetworkDNSServers:
  - 77.88.8.8
  - 1.1.1.1
mainNetwork: internal
bastion:
  instanceClass:
    rootDiskSizeGb: 30
    sizingPolicy: 2cpu1mem
    template: "catalog/Ubuntu 22.04 Server"
    storageProfile: Fast vHDD
    mainNetworkIPAddress: 10.1.4.10
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-V"
  externalIP: 10.0.0.1
  externalPort: 10022
  NSX-V:
    externalNetworkName: external
    externalNetworkType: ext
createDefaultFirewallRules: true
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

Integration is carried out using the [VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration) resource, which describes the configuration of the cloud cluster in VCD and is used by the virtualization system if the cluster’s control plane is hosted there. The DKP module responsible for integration is automatically configured based on the selected deployment layout.

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
