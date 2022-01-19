---
title: "Cloud provider — Azure: схемы размещения"
---

> **Внимание!** Поддерживаются только [регионы](https://docs.microsoft.com/ru-ru/azure/availability-zones/az-region) в которых доступны `Availability Zones`.

## Схемы размещения
### Standard
* Для кластера создаётся отдельная [resorce group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal).
* По умолчанию каждому инстансу динамически выделается один внешний IP-адрес, который используется только для доступа в интернет. На каждый IP для SNAT доступно 64000 портов.
* Поддерживается [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([тарификация](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)). Позволяет использовать статические публичные IP для SNAT.
* Публичные IP адреса можно назначить на master-узлы и узлы, созданные Terraform.
* Если master не имеет публичного IP, то для установки и доступа в кластер, необходим дополнительный инстанс с публичным IP (aka bastion). В этом случае так же потребуется настроить peering между VNet кластера и VNet bastion.
* Между VNet кластера и другими VNet можно настроить peering.

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
