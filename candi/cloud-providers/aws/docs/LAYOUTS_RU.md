---
title: "Cloud provider — AWS: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в AWS при работе облачного провайдера Deckhouse."
---

Поддерживаются две схемы размещения. Ниже подробнее о каждой их них.

## WithoutNAT

**Рекомендованная схема размещения.**

Каждому узлу присваивается публичный IP (ElasticIP). NAT не используется.


<!--- Исходник: https://docs.google.com/drawings/d/1JDmeSY12EoZ3zBfanEDY-QvSgLekzw6Tzjj2pgY8giM/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithoutNAT
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: <SSH_PUBLIC_KEY>
provider:
  providerAccessKeyId: '<AWS_ACCESS_KEY>'
  providerSecretAccessKey: '<AWS_SECRET_ACCESS_KEY>'
  region: eu-central-1
masterNodeGroup:
  # Количество master-узлов.
  # Если указано больше одного master-узла, то etcd-кластер соберется автоматически.
  replicas: 1
  instanceClass:
    # Тип используемого инстанса.
    instanceType: m5.xlarge
    # ID образа виртуальной машины в Amazon.
    # Каталог AMI в консоли AWS: EC2 -> AMI Catalog.
    ami: ami-0caef02b518350c8b
    # Размер диска для виртуальной машины master-узла.
    diskSizeGb: 30
    # Используемый тип диска для виртуальной машины master-узла.
    diskType: gp3
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-0caef02b518350c8b
    additionalTags:
      backup: srv1
tags:
  team: torpedo
```

## WithNAT

> **Важно!** В данной схеме размещения необходим bastion-хост для доступа к узлам (его можно создать вместе с кластером, указав параметры в секции `withNAT.bastionInstance`).
>
> **Важно!** В этой схеме размещения NAT Gateway всегда создается в зоне `a`. Если узлы кластера будут заказаны в других зонах, при проблемах в зоне `a` они также будут недоступны. Другими словами, при выборе схемы размещения `WithNat` доступность всего кластера будет зависеть от работоспособности зоны `a`.

Виртуальные машины выходят в интернет через NAT Gateway с общим и единственным IP-адресом.


<!--- Исходник: https://docs.google.com/drawings/d/1UPzygO3w8wsRNHEna2uoYB-69qvW6zDYB5s1OumUOes/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
layout: WithNAT
provider:
  providerAccessKeyId: '<AWS_ACCESS_KEY>'
  providerSecretAccessKey: '<AWS_SECRET_ACCESS_KEY>'
  region: eu-central-1
withNAT:
  bastionInstance:
    zone: eu-central-1a
    instanceClass:
      instanceType: m5.large
      ami: ami-0caef02b518350c8b
      diskType: gp3
masterNodeGroup:
  # Количество master-узлов.
  # Если указано больше одного master-узла, etcd-кластер соберется автоматически.
  replicas: 1
  instanceClass:
    # Тип используемого инстанса.
    instanceType: m5.xlarge
    # ID образа виртуальной машины в Amazon.
    # Каталог AMI в консоли AWS: EC2 -> AMI Catalog.
    ami: ami-0caef02b518350c8b
    # Размер диска для виртуальной машины master-узла.
    diskSizeGb: 30
    # Используемый тип диска для виртуальной машины master-узла.
    diskType: gp3
nodeGroups:
  - name: mydb
    nodeTemplate:
      labels:
        node-role.kubernetes.io/mydb: ""
    replicas: 2
    instanceClass:
      instanceType: t2.medium
      ami: ami-0caef02b518350c8b
    additionalTags:
      backup: me
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: "<SSH_PUBLIC_KEY>"
tags:
  team: torpedo
```
