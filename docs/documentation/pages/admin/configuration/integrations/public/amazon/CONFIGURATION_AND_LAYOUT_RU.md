---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/public/amazon/amazon-layout.html
lang: ru
---

Данный раздел описывает схемы размещения кластера в инфраструктуре AWS и связанные с ними параметры. Выбор схемы (layout) влияет на способ назначения публичных IP-адресов, работу NAT и возможность подключения к узлам.

## WithoutNAT

Рекомендуемая схема размещения.

Каждому узлу назначается публичный IP-адрес (Elastic IP), NAT-шлюз не используется. Такая схема обеспечивает прямой доступ к узлам по публичным IP-адресам и позволяет упростить маршрутизацию исходящего трафика.

![resources](../../../../images/cloud-provider-aws/aws-withoutnat.png)
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

{% alert level="warning" %} В данной схеме NAT Gateway всегда создается в зоне `a`. Если узлы кластера размещены в других зонах, то при сбое зоны a кластер может стать недоступным. Bastion-хост обязателен для подключения к узлам. {% endalert %}

В этой схеме размещения NAT Gateway используется для выхода в интернет, а публичные IP-адреса узлам не присваиваются. Доступ к узлам возможен только через bastion-хост, размещаемый в отдельной подсети.

![resources](../../../../images/cloud-provider-aws/aws-withnat.png)
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

## Назначение AWSClusterConfiguration

Ресурс AWSClusterConfiguration описывает параметры кластера и используется платформой Deckhouse для:

- задания схемы размещения и сетевых CIDR;
- конфигурации master- и рабочих узлов;
- указания параметров подключения к AWS API (ключи доступа, регион);
- назначения общих и специфичных тегов;
- описания настроек bastion-хоста (в схеме WithNAT).

Обязательные поля:

- `apiVersion` — должен быть `deckhouse.io/v1`;
- `kind` — всегда AWSClusterConfiguration.

Пример заголовка ресурса:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
```

Чтобы отредактировать этот ресурс в работающем кластере, выполните команду:

```console
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

После внесения изменений их необходимо применить с помощью команды:

```console
dhctl converge
```

## Внутренняя адресация и подсети

Параметр `nodeNetworkCIDR` определяет диапазон адресов, который будет распределен по зонам доступности. Этот диапазон должен соответствовать или быть вложенным в `vpcNetworkCIDR`. Подсети автоматически создаются на основании количества зон региона.

Пример:

```yaml
nodeNetworkCIDR: 10.241.1.0/20
vpcNetworkCIDR: 10.241.0.0/16
```

## Группы безопасности

Группы безопасности (security groups) в AWS используются для управления входящим и исходящим сетевым трафиком на виртуальные машины. В Deckhouse Kubernetes Platform они позволяют:

- разрешить подключение к узлам кластера с других подсетей;
- открыть доступ к приложениям, размещённым на статических узлах;
- ограничить или разрешить доступ к внешним ресурсам в соответствии с требованиями безопасности.

> Deckhouse не создаёт группы безопасности автоматически. В конфигурации кластера следует указывать уже существующие security groups, созданные вручную через AWS Console или иным способом.

Дополнительные группы безопасности можно назначить в следующих случаях:

| Тип узлов              | Где указывать                                                                 |
|------------------------|-------------------------------------------------------------------------------|
| Master-узлы            | В поле `masterNodeGroup.instanceClass.additionalSecurityGroups` ресурса `AWSClusterConfiguration` |
| Статические worker-узлы| В поле `nodeGroups[].instanceClass.additionalSecurityGroups` того же ресурса |
| Эфемерные узлы         | В объекте `AWSInstanceClass`, в поле `spec.additionalSecurityGroups`         |

Во всех случаях параметр `additionalSecurityGroups` принимает массив строк — имен (ID) групп безопасности в AWS.

## Настройка пирингового соединения между VPC

Для примера рассмотрим настройку пирингового соединения между двумя условными VPC — `vpc-a` и `vpc-b`.

> **Важно.** IPv4 CIDR у обоих VPC должен различаться.

Для настройки выполните следующие шаги:

1. Перейдите в регион, где работает `vpc-a`.
1. Нажмите «VPC» → «VPC Peering Connections» → «Create Peering Connection» и настройте пиринговое соединение:
   - Name: `vpc-a-vpc-b`.
   - Заполните «Local» и «Another VPC».
1. Перейдите в регион, где работает `vpc-b`.
1. Нажмите «VPC» → «VPC Peering Connections».
1. Выделите созданное соединение и выберите «Action Accept Request».
1. Для `vpc-a` добавьте во все таблицы маршрутизации маршруты до CIDR `vpc-b` через пиринговое соединение.
1. Для `vpc-b` добавьте во все таблицы маршрутизации маршруты до CIDR `vpc-a` через пиринговое соединение.

## Настройка доступа через bastion-хост

Для подключения к узлам в приватных подсетях используйте параметр `withNAT.bastionInstance` в AWSClusterConfiguration. Bastion-хост заказывается вместе с инфраструктурой по заданным параметрам `instanceClass`.

Поддерживаются сценарии:

- bastion-хост уже создан во внешней VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Настройте пиринговое соединение между внешней и свежесозданной VPC.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.

- bastion-хост требуется поставить в свежесозданной VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Запустите вручную bastion-хост в подсети `<prefix>-public-0`.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.

### Создание кластера в новом VPC с доступом через имеющийся bastion-хост

1. Выполните bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

1. Настройте пиринговое соединение по [инструкции выше](#настройка-пирингового-соединения-между-vpc).

1. Продолжите установку кластера. На вопрос про кэш Terraform ответьте `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

### Создание кластера в новом VPC и развертывание bastion-хоста для доступа к узлам

1. Выполните bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

1. Запустите вручную bastion-хост в подсети `<prefix>-public-0`.

1. Продолжите установку кластера. На вопрос про кэш Terraform ответьте `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## Использование существующего VPC (existingVPCID)

Параметр `existingVPCID` в ресурсе AWSClusterConfiguration позволяет использовать уже существующий VPC для развертывания кластера Deckhouse, вместо создания нового VPC автоматически.

Этот параметр может быть полезен в случаях, когда:

- инфраструктура AWS уже частично развернута;
- необходимо интегрироваться с другими сервисами или ресурсами, размещёнными в этом VPC;
- политика безопасности или архитектурные требования запрещают автоматическое создание VPC.

> **Важно.** Если в существующем VPC уже есть Internet Gateway, попытка развертывания базовой инфраструктуры завершится ошибкой. В текущей версии Deckhouse не поддерживается повторное использование уже существующего Internet Gateway.

Совместимость с другими параметрами:

- Если указан `existingVPCID`, не указывайте `vpcNetworkCIDR` — это взаимоисключающие параметры.
- Параметр `nodeNetworkCIDR` можно (и желательно) указывать — он будет вложен в существующий VPC.