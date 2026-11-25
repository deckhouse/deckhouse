---
title: Схемы размещения Amazon Web Services
permalink: ru/admin/integrations/public/amazon/layout.html
lang: ru
---

Данный раздел описывает [схемы размещения](../../../../glossary.html#layout)
кластера в инфраструктуре AWS. Выбор
схемы (layout) влияет на способ назначения публичных
IP-адресов, работу NAT и возможность подключения к узлам.

## WithoutNAT

Рекомендуемая схема размещения.

Каждому узлу назначается публичный IP-адрес (Elastic
IP), NAT-шлюз не используется. Такая схема обеспечивает
прямой доступ к узлам по публичным IP-адресам и позволяет упростить маршрутизацию исходящего трафика.

![resources](../../../../images/cloud-provider-aws/aws-withoutnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10681&t=Qb5yyWumzPiTBtfL-0 --->

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
    ami: ami-0aad10862ade98f27
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
      ami: ami-0aad10862ade98f27
    additionalTags:
      backup: srv1
tags:
  team: torpedo
```

## WithNAT

{% alert level="warning" %} В данной схеме NAT Gateway всегда создается в зоне `a`. Если узлы кластера размещены в других зонах, то при сбое зоны a кластер может стать недоступным. [Bastion-хост](../../../../glossary.html#bastion-host) обязателен для подключения к узлам. {% endalert %}

В этой схеме размещения NAT Gateway используется для выхода в интернет, а публичные IP-адреса узлам
не присваиваются. Доступ к узлам возможен только
через bastion-хост, размещаемый в отдельной подсети.

![resources](../../../../images/cloud-provider-aws/aws-withnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-9864&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации [схемы размещения](../../../../glossary.html#layout):

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
      ami: ami-0aad10862ade98f27
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
    ami: ami-0aad10862ade98f27
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
      ami: ami-0aad10862ade98f27
    additionalTags:
      backup: me
vpcNetworkCIDR: "10.241.0.0/16"
nodeNetworkCIDR: "10.241.32.0/20"
sshPublicKey: "<SSH_PUBLIC_KEY>"
tags:
  team: torpedo
```

[Настройка интеграции](configuration.html).
[Настройка сети](network.html).
