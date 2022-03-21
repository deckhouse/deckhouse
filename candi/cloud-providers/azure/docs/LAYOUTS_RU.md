---
title: "Cloud provider — Azure: схемы размещения"
---

> **Внимание!** Поддерживаются только те [регионы](https://docs.microsoft.com/ru-ru/azure/availability-zones/az-region), в которых доступны `Availability Zones`.

## Standard

* Для кластера создаётся отдельная [resorce group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal);
* По умолчанию каждому инстансу динамически выделяется один внешний IP-адрес, который используется только для доступа в интернет. На каждый IP-адрес для SNAT доступно 64000 портов;
* Поддерживается [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([тарификация](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)). Она позволяет использовать статические публичные IP-адреса для SNAT;
* Публичные IP-адреса можно назначить как на master-узлы, так и на узлы, созданные Terraform;
* Если master-узел не имеет публичного IP-адреса, то для установки и доступа в кластер необходим дополнительный инстанс с публичным IP-дресом (например, bastion-хост). В этом случае также потребуется настроить пиринговое соединение между VNet кластера и VNet bastion-хоста;
* Между VNet кластера и другими VNet можно настроить пиринговое соединение.

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa <SSH_PUBLIC_KEY>" # Required.
vNetCIDR: 10.50.0.0/16 # Required.
subnetCIDR: 10.50.0.0/24 # Required.
standard:
  natGatewayPublicIpCount: 1 # Optional, by default 0.
masterNodeGroup:
  replicas: 1
  zones: ["1"] # Optional, by default ["1"].
  instanceClass:
    machineSize: Standard_F4 # Required.
    diskSizeGb: 32
    # Required.
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    enableExternalIP: false # Optional, by default true.
provider:
  subscriptionId: "" # Required.
  clientId: "" # Required.
  clientSecret: "" # Required.
  tenantId: "" # Required.
  location: "westeurope" # Required.
# Optional, list of Azure VNets with which Kubernetes VNet will be peered.
peeredVNets:
  - resourceGroupName: kube-bastion # Required.
    vnetName: kube-bastion-vnet # Required.
```
