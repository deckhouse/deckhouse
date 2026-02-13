---
title: "Cloud provider — Azure: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в Azure при работе облачного провайдера Deckhouse."
---

> **Внимание!** Поддерживаются только те [регионы](https://docs.microsoft.com/ru-ru/azure/availability-zones/az-region), в которых доступны `Availability Zones`.

## Standard

* Для кластера создается отдельная [ресурсная группа](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal) (resource group).
* По умолчанию каждому инстансу динамически выделяется один внешний IP-адрес, который используется только для доступа в интернет. На каждый IP-адрес для SNAT доступно 64000 портов.
* Поддерживается [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([тарификация](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)). Она позволяет использовать статические публичные IP-адреса для SNAT.
* Публичные IP-адреса можно назначить как на master-узлы, так и на узлы, созданные Terraform.
* Если master-узел не имеет публичного IP-адреса, для установки и доступа в кластер необходим дополнительный инстанс с публичным IP-адресом (например, bastion-хост). В этом случае также потребуется настроить пиринговое соединение между VNet кластера и VNet bastion-хоста.
* Между VNet кластера и другими VNet можно настроить пиринговое соединение.

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>" # Обязательный параметр.
vNetCIDR: 10.50.0.0/16 # Обязательный параметр.
subnetCIDR: 10.50.0.0/24 # Обязательный параметр.
standard:
  natGatewayPublicIpCount: 1 # Необязательный параметр, по умолчанию 0.
masterNodeGroup:
  replicas: 1
  zones: ["1"] # Необязательный параметр, по умолчанию ["1"].
  instanceClass:
    machineSize: Standard_F4 # Обязательный параметр.
    diskSizeGb: 32
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140  # Обязательный параметр.
    enableExternalIP: false # Необязательный параметр, по умолчанию true.
provider:
  subscriptionId: "<SUBSCRIPTION_ID>" # Обязательный параметр.
  clientId: "<CLIENT_ID>" # Обязательный параметр.
  clientSecret: "<CLIENT_SECRET>" # Обязательный параметр.
  tenantId: "<TENANT_ID>" # Обязательный параметр.
  location: "westeurope" # Обязательный параметр.
# Необязательный параметр, список Azure VNets, с которыми Kubernetes VNet будет
# соединяться через пиринговое соединение.
peeredVNets:
  - resourceGroupName: kube-bastion # Обязательный параметр.
    vnetName: kube-bastion-vnet # Обязательный параметр.
```
