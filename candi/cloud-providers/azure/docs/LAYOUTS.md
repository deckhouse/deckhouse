---
title: "Cloud provider — Azure: Layouts"
---

## Layouts
### Standard
* A separate [resorce group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal) is created for the cluster.
* By default, one external IP address is dynamically allocated to each instance (it is used for Internet access only). Each IP has 64000 ports available for SNAT.
* The [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([pricing](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)) is supported. With it, you can use static public IP addresses for SNAT.
* Public IP addresses can be assigned to master nodes and nodes created by Terraform.
* If the master does not have a public IP, then an additional instance with a public IP (aka bastion host) is required for installation tasks and access to the cluster. In this case, you will also need to configure peering between the cluster's VNet and bastion's VNet.
* Peering can also be configured between the cluster VNet and other VNets.

```yaml
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>" # required
vNetCIDR: 10.50.0.0/16 # required
subnetCIDR: 10.50.0.0/24 # required
standard:
  natGatewayPublicIpCount: 1 # optional, by default 0
masterNodeGroup:
  replicas: 1
  zones: [1] # optional, by default [1]
  instanceClass:
    machineSize: Standard_F4 # required
    diskSizeGb: 32
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140 # required
    enableExternalIP: false # optional, by default true
provider:
  subscriptionId: "" # required
  clientId: "" # required
  clientSecret: "" # required
  tenantId: "" # required
  location: "westeurope" # required
peeredVNets: # optional, list of Azure VNets with which Kubernetes VNet will be peered
  - resourceGroupName: kube-bastion # required
    vnetName: kube-bastion-vnet # required
```

## AzureClusterConfiguration
**Caution!** Only [regions](https://docs.microsoft.com/en-us/azure/availability-zones/az-region) where `Availability Zones` are available are supported.

A particular placement strategy is defined via the `AzureClusterConfiguration` struct.
It has the following fields:
* `apiVersion` — deckhouse.io/v1
* `kind` — AzureClusterConfiguration
* `layout` — the way resources are located in the cloud;
    * Possible values: `Standard` (the description is provided below).
* `sshPublicKey` — public key to access nodes as `azureuser`;
    * A mandatory parameter;
* `vNetCIDR` — an address space of the virtual network in the [CIDR](https://en.wikipedia.org/wiki/Classless_Inter-Domain_Routing) format.
    * A mandatory parameter;
* `subnetCIDR` — a subnet from the `vNetCIDR` address space for cluster nodes;
    * A mandatory parameter;
* `standard` — settings for the `Standard` layout;
    * `natGatewayPublicIpCount` — the number of IP addresses for the [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([pricing](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)).
    * The default value is `0` (`NAT Gateway` is not used).
    * An optional parameter;
* `tags` — a list of tags in the `key: value` format to attach to all cluster resources. You have to re-create all the machines to add new tags if tags were modified in the running cluster;
* `masterNodeGroup` — the definition of the master's NodeGroup;
    * `replicas` — the number of master nodes to create;
    * `zones` — a list of zones where master nodes can be created;
        * You can browse a list of zones available for the selected instance type using the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli):
            * `az vm list-skus -l westeurope -o table`
        * The default value is `[1,2,3]`;
    * `instanceClass` — partial contents of the [AzureInstanceClass](cr.html#azureinstanceclass) fields.  The parameters in **bold** are unique for `AzureClusterConfiguration`. Possible values:
        * `machineSize`
        * `diskSizeGb`
        * `urn`
        * **`enableExternalIP`** — this parameter is only available for the `Standard` layout;
            * It is set to `false` by default. The nodes do not have public addresses and access the Internet over NAT;
            * `true` — static public addresses are created for nodes;
        * `additionalTags` — a list of additional tags in the `key: value` format to attach to instances;
* `nodeGroups` — an array of additional NodeGroups for creating static nodes (e.g., for dedicated front nodes or gateways). NodeGroup parameters:
    * `name` — the name of the NodeGroup to use for generating node names;
    * `replicas` — the number of nodes;
    * `zones` — a list of zones where static nodes can be created;
        * You can browse a list of zones available for the selected instance type using the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli):
            * `az vm list-skus -l westeurope -o table`
        * The default value is `[1,2,3]`;
    * `instanceClass` — partial contents of the [AzureInstanceClass](cr.html#azureinstanceclass) fields.  The parameters in **bold** are unique for  `AzureClusterConfiguration`. Possible values:
        * `machineSize`
        * `diskSizeGb`
        * `urn`
        * **`enableExternalIP`** — this parameter is only available for the `Standard` layout;
            * It is set to `false` by default. The nodes do not have public addresses and access the Internet over NAT.
            * `true` — static public addresses are created for nodes;
        * `additionalTags` — a list of additional tags in the `key: value` format to attach to instances;
    * `nodeTemplate` — parameters of Node objects in Kubernetes to add after registering the node;
      * `labels` — the same as the `metadata.labels` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
        * An example:
          ```yaml
          labels:
            environment: production
            app: warp-drive-ai
          ```
      * `annotations` — the same as the `metadata.annotations` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
        * An example:
          ```yaml
          annotations:
            ai.fleet.com/discombobulate: "true"
          ```
      * `taints` — the same as the `.spec.taints` field of the [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core) object. **Caution!** Only `effect`, `key`, `values` fields are available;
        * An example:

          ```yaml
          taints:
          - effect: NoExecute
            key: ship-class
            value: frigate
          ```
* `provider` — parameters for connecting to the Azure API;
    * `subscriptionId` — the ID of the subscription;
    * `clientId` — the client ID;
    * `clientSecret` — the client's secret;
    * `tenantId` — the ID of the tenant;
    * `location` — the name of the region to create all the resources;
* `peeredVNets` — an array of `VNets` to merge with the cluster network. The service account must have access to all the `VNets` listed above. You have to configure the peering connection [manually ](https://docs.microsoft.com/en-us/azure/virtual-network/virtual-network-peering-overview) if no access is available;
    * `resourceGroupName` — the name of the resource group with the VNet;
    * `vnetName` — the name of the VNet;
* `zones` — a limited set of zones in which nodes can be created;
  * An optional parameter;
  * Format — an array of strings;
