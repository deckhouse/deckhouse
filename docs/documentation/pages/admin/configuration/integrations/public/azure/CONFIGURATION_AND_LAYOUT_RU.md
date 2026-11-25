---
title: Схемы размещения Microsoft Azure
permalink: ru/admin/integrations/public/azure/layout.html
lang: ru
---

Данный раздел описывает [схему размещения](../../../../glossary.html#layout) кластера в инфраструктуре Azure.

## Standard

Standard — поддерживаемая схема размещения:

- Для кластера создаётся отдельная ресурсная группа (resource group).
- Каждому инстансу по умолчанию выделяется внешний IP-адрес (используется только для выхода в интернет).
- Для SNAT используется до 64 000 портов на один IP-адрес.
- Поддерживается NAT Gateway с возможностью тарификации — позволяет использовать статические публичные IP-адреса для SNAT.
- Публичные IP могут быть назначены как на [master-узлы](../../../../glossary.html#master-node), так и на узлы, созданные через Terraform.
- При отсутствии внешнего IP у master-узла требуется
  [bastion-хост](../../../../glossary.html#bastion-host) и VNet-пиринг между кластерами.
- Поддерживается пиринг между VNet кластера и другими VNet.

Пример конфигурации размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"       # Обязательный параметр.
vNetCIDR: 10.50.0.0/16                  # Обязательный параметр.
subnetCIDR: 10.50.0.0/24                # Обязательный параметр.
standard:
  natGatewayPublicIpCount: 1           # Необязательный параметр (по умолчанию 0).
masterNodeGroup:
  replicas: 1
  zones: ["1"]                          # Необязательный параметр (по умолчанию ["1"]).
  instanceClass:
    machineSize: Standard_F4           # Обязательный параметр.
    diskSizeGb: 32
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140  # Обязательный параметр.
    enableExternalIP: false            # Необязательный параметр (по умолчанию true).
provider:
  subscriptionId: "<SUBSCRIPTION_ID>"  # Обязательный параметр.
  clientId: "<CLIENT_ID>"              # Обязательный параметр.
  clientSecret: "<CLIENT_SECRET>"      # Обязательный параметр.
  tenantId: "<TENANT_ID>"              # Обязательный параметр.
  location: "westeurope"               # Обязательный параметр.
peeredVNets:                            # Необязательный параметр.
  - resourceGroupName: kube-bastion    # Обязательный параметр.
    vnetName: kube-bastion-vnet        # Обязательный параметр.
```

[Настройка сети](network.html).

## Замена образа ОС

Образ ОС узлов задаётся параметром [`urn`](/modules/cloud-provider-azure/cr.html#azureinstanceclass-v1-spec-urn) ресурса AzureInstanceClass для узлов типа `CloudEphemeral` и в секции `instanceClass` ресурса AzureClusterConfiguration для узлов типа `CloudPermanent` и master-узлов. Чтобы перевести узлы кластера на новый образ, выполните шаги из раздела [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).
