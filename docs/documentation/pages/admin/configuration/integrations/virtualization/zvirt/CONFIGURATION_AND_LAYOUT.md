---
title: Layouts and configuration in zVirt
permalink: en/admin/integrations/virtualization/zvirt/layout.html
---

## Layouts

The [layout](../../../../glossary.html#layout) defines how cluster nodes are placed in the zVirt infrastructure. Only the Standard layout is supported. The layout is set with the [`layout`](/modules/cloud-provider-zvirt/configuration.html#parameters-nodes-parameters-layout) parameter in the ModuleConfig resource of the [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/) module.

### Standard

In the Standard layout, all cluster nodes are created in a single zVirt cluster. Key features of the layout:

- Virtual machines are created in the zVirt cluster set with the [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid) parameter.
- The zVirt cluster must have shared storage domains available to all its hosts and CPUs of the same type.
- Virtual machines are cloned from a template prepared from a cloud image.
- The virtual machine network interface is created from the vNIC profile set with the [`vnicProfileID`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-vnicprofileid) parameter of the ZvirtInstanceClass resource.
- Deckhouse Platform creates and deletes the node virtual machines via the zVirt API.

![Standard layout in zVirt](../../../../images/cloud-provider-zvirt/zvirt-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11447&t=Qb5yyWumzPiTBtfL-0 --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        server: <ZVIRT_API_URL>
        clusterID: <CLUSTER_ID>
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: <SSH_PUBLIC_KEY>
```

## Configuration

The zVirt cloud provider parameters are set in four resources:

- The [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) of the [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/) module sets the [layout](#layouts), the zVirt API connection parameters, and the zVirt cluster to create virtual machines in.
- The `d8-credentials` Secret in the `d8-cloud-provider-zvirt` namespace stores the zVirt user login and password in the format covered in the ["Credentials Secret"](authorization.html#credentials-secret) section.
- [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) describes the virtual machine parameters.
- [NodeGroup](/modules/node-manager/cr.html#nodegroup) describes the node groups and references the instance class. Master nodes are described by the `master` node group with the [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) type.

To change the provider settings in a running cluster, edit the ModuleConfig:

```shell
d8 k edit mc cloud-provider-zvirt
```

To apply the changes to the master nodes and the nodes of CloudPermanent groups, run the following command:

```shell
dhctl converge
```

{% alert level="warning" %}
If the cluster was installed with the [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) schema, it must be migrated to the ModuleConfig-based configuration.
Until the migration is complete, the [`D8CloudProviderZvirtMigrationPending`](../../../../reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending) alert may fire and Deckhouse Platform updates may be blocked.

The migration procedure is described in ["How to migrate a cloud provider to ModuleConfig-based configuration?"](../../../../faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat).
{% endalert %}

### Configuration example

Below is a minimal configuration of a cluster with the Standard layout, one master node, and one group of worker nodes:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        server: <ZVIRT_API_URL>
        clusterID: <CLUSTER_ID>
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: <SSH_PUBLIC_KEY>
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-zvirt
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: userPassword
  identity: <USERNAME>
  secret: <PASSWORD>
---
apiVersion: deckhouse.io/v1
kind: ZvirtInstanceClass
metadata:
  name: master
spec:
  numCPUs: 4
  memory: 8192
  rootDiskSizeGb: 50
  etcdDiskSizeGb: 10
  template: <TEMPLATE_NAME>
  vnicProfileID: <VNIC_PROFILE_ID>
  storageDomainID: <STORAGE_DOMAIN_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: ZvirtInstanceClass
      name: master
    minPerZone: 1
    maxPerZone: 1
---
apiVersion: deckhouse.io/v1
kind: ZvirtInstanceClass
metadata:
  name: worker
spec:
  numCPUs: 4
  memory: 8192
  template: <TEMPLATE_NAME>
  vnicProfileID: <VNIC_PROFILE_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: ZvirtInstanceClass
      name: worker
    minPerZone: 1
    maxPerZone: 3
```

Where:

- `ZVIRT_API_URL`: URL of the zVirt API endpoint (the [`server`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-server) parameter), for example, `https://zvirt.example.com/ovirt-engine/api`.
- `CLUSTER_ID`: ID of the zVirt cluster to create virtual machines in (the [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid) parameter).
- `SSH_PUBLIC_KEY`: Public SSH key for accessing the nodes (the [`sshPublicKey`](/modules/cloud-provider-zvirt/configuration.html#parameters-nodes-parameters-sshpublickey) parameter).
- `USERNAME` and `PASSWORD`: zVirt user login and password for the [credentials Secret](authorization.html#credentials-secret).
- `TEMPLATE_NAME`: Name of the virtual machine template prepared as described in the ["Preparing a virtual machine template"](authorization.html#preparing-a-virtual-machine-template) section.
- `VNIC_PROFILE_ID`: ID of the vNIC profile used to create the virtual machine network interface.
- `STORAGE_DOMAIN_ID`: ID of the zVirt storage domain to create the etcd disk in.

The number of master nodes is set with the [`minPerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-minperzone) parameter of the `master` node group. In the ZvirtInstanceClass resource of the `master` group, set the [`etcdDiskSizeGb`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-etcddisksizegb) and [`storageDomainID`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-storagedomainid) parameters, which set the size and the storage domain of a separate disk for etcd. The `etcdDiskSizeGb` parameter cannot be set in the instance classes of other CloudPermanent groups.

If the zVirt API uses a self-signed certificate, pass the CA certificate in the [`caBundle`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-cabundle) parameter or set the [`insecure`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-insecure) parameter to `true`.

The other parameters are described in the [module settings](/modules/cloud-provider-zvirt/configuration.html) and in the [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) resource description.

{% alert level="info" %}
The `clusterID`, `vnicProfileID`, and `storageDomainID` values can be obtained via the zVirt API or the Administration Portal. Getting the vNIC profile ID via the API is described in the ["How to get the vNIC profile ID?"](/modules/cloud-provider-zvirt/faq.html#how-to-get-the-vnic-profile-id) section.
{% endalert %}

### Static network configuration of nodes

Nodes whose group has no static network configuration get their network settings over DHCP. To assign static IP addresses to the nodes of CloudPermanent groups, set the [`customNetworkConfigs`](/modules/cloud-provider-zvirt/configuration.html#parameters-nodes-parameters-customnetworkconfigs) parameter in the `nodes.parameters` section of the ModuleConfig resource. The parameter key sets the node group name, and the value sets the network interface configuration of the nodes in this group.

Example of the static network configuration for the `master` group of three nodes:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  settings:
    nodes:
      parameters:
        customNetworkConfigs:
          master:
            networkInterfaceName: enp1s0
            networkInterfaceAddresses:
              - 192.168.1.10
              - 192.168.1.11
              - 192.168.1.12
            networkInterfaceNetmask: 255.255.255.0
            networkInterfaceGateway: 192.168.1.1
            dnsServers:
              - 192.168.1.2
```

Addresses from the `networkInterfaceAddresses` list are assigned to the group nodes in order, one address per node, so specify an address for each node of the group. Duplicate addresses in the list are not allowed.

{% alert level="warning" %}
If you change the network settings of existing nodes in the `customNetworkConfigs` parameter, `dhctl converge` recreates the virtual machines of these nodes.
{% endalert %}
