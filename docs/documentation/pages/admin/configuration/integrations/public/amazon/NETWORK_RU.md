---
title: Настройка сети в Amazon Web Services
permalink: ru/admin/integrations/public/amazon/network.html
lang: ru
---

## Внутренняя адресация и подсети

[Параметр `nodeNetworkCIDR`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-nodenetworkcidr)
определяет диапазон адресов, который будет распределен по [зонам доступности](../../../../glossary.html#availability-zone).
Этот диапазон должен соответствовать или быть вложенным в [`vpcNetworkCIDR`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-vpcnetworkcidr). Подсети автоматически создаются на основании количества зон региона.

Пример:

```yaml
nodeNetworkCIDR: 10.241.1.0/20
vpcNetworkCIDR: 10.241.0.0/16
```

## Группы безопасности

Группы безопасности (security groups) в AWS используются для управления входящим и исходящим сетевым трафиком на виртуальные машины. В Deckhouse Platform они позволяют:

- разрешить подключение к узлам кластера с других подсетей;
- открыть доступ к приложениям, размещённым на [статических узлах](../../../../glossary.html#static-node);
- ограничить или разрешить доступ к внешним ресурсам в соответствии с требованиями безопасности.

Если задан параметр [`disableDefaultSecurityGroup: false`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-disabledefaultsecuritygroup),
при создании кластера Deckhouse Platform создаёт группы безопасности по умолчанию:

- `<CLUSTER_PREFIX>-node` — назначается узлам кластера:
  - разрешение любого исходящего трафика в `0.0.0.0/0`;
  - разрешение любого входящего трафика от группы `<CLUSTER_PREFIX>-loadbalancer`;
  - разрешение любого входящего трафика от узлов той же группы `<CLUSTER_PREFIX>-node`;
  - разрешение входящего трафика по протоколу ICMP из CIDR, указанных в [`publicNetworkAllowList`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-publicnetworkallowlist) (по умолчанию `0.0.0.0/0`).
- `<CLUSTER_PREFIX>-loadbalancer` — используется балансировщиками нагрузки:
  - разрешение любого входящего трафика из CIDR, указанных в [`publicNetworkAllowList`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-publicnetworkallowlist);
  - разрешение любого исходящего трафика к группе `<CLUSTER_PREFIX>-node`.
- `<CLUSTER_PREFIX>-ssh-accessible` — создаётся, если задан [`sshAllowList`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-sshallowlist), и разрешает входящий трафик по протоколу
  TCP и порту `22` из указанных CIDR (по умолчанию `0.0.0.0/0`). Назначается [master-узлам](../../../../glossary.html#master-node) или [bastion-хосту](../../../../glossary.html#bastion-host) в схеме WithNAT.

Помимо групп по умолчанию, к узлам можно подключить собственные группы безопасности, созданные в облаке заранее.

Дополнительные группы безопасности можно назначить в следующих случаях:

| Тип узлов              | Где указывать                                                                 |
|------------------------|-------------------------------------------------------------------------------|
| Master-узлы            | В [поле `masterNodeGroup.instanceClass.additionalSecurityGroups`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup-instanceclass-additionalsecuritygroups) ресурса AWSClusterConfiguration |
| Статические [worker-узлы](../../../../glossary.html#worker-node)| В [поле `nodeGroups[].instanceClass.additionalSecurityGroups`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-nodegroups-instanceclass-additionalsecuritygroups) того же ресурса |
| Эфемерные узлы         | В объекте AWSInstanceClass, в [поле `spec.additionalSecurityGroups`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-additionalsecuritygroups)         |

Во всех случаях параметр `additionalSecurityGroups` принимает массив строк — имен (ID) групп безопасности в AWS.

Если установлено значение `disableDefaultSecurityGroup: true`, то группы по умолчанию создаваться не будут.

При использовании `disableDefaultSecurityGroup: true` необходимо самостоятельно создать все необходимые группы безопасности для доступа к узлам кластера. Кроме того, необходимо явно указать их в следующих параметрах:

- [`additionalSecurityGroups`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup-instanceclass-additionalsecuritygroups) в секции `masterNodeGroup` ресурса AWSClusterConfiguration;
- [`additionalSecurityGroups`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-additionalsecuritygroups) в ресурсе AWSInstanceClass;
- [`additionalSecurityGroups`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-nodegroups-instanceclass-additionalsecuritygroups) в секции `nodeGroups.instanceClass`.

Для настройки групп, используемых балансировщиками нагрузки, укажите их через аннотацию `service.beta.kubernetes.io/aws-load-balancer-security-groups`.

## Настройка пирингового соединения между VPC

Для примера рассмотрим настройку пирингового соединения между двумя условными VPC — `vpc-a` и `vpc-b`.

{% alert level="warning" %}
IPv4 CIDR у обоих VPC должен различаться.
{% endalert %}

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

Для подключения к узлам в приватных подсетях используйте [параметр `withNAT.bastionInstance`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-withnat-bastioninstance)
в AWSClusterConfiguration. Bastion-хост заказывается вместе с инфраструктурой по заданным параметрам `instanceClass`.

Поддерживаются сценарии:

- bastion-хост уже создан во внешней VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Настройте пиринговое соединение между внешней и свежесозданной VPC.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.

- bastion-хост требуется поставить в свежесозданной VPC:
  1. Создайте базовую инфраструктуру кластера — `dhctl bootstrap-phase base-infra`.
  1. Запустите вручную bastion-хост в подсети `<CLUSTER_PREFIX>-public-0`.
  1. Продолжите установку с указанием bastion-хоста — `dhctl bootstrap --ssh-bastion...`.

### Создание кластера в новом VPC с доступом через имеющийся bastion-хост

Выполните следующие шаги, чтобы создать кластер с доступом через существующий bastion-хост.

1. Выполните bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

1. Настройте пиринговое соединение [по инструкции](#настройка-пирингового-соединения-между-vpc) выше.

1. Продолжите установку кластера. На вопрос про кеш Terraform ответьте `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

### Создание кластера в новом VPC и развертывание bastion-хоста для доступа к узлам

Выполните следующие шаги, чтобы создать кластер и bastion-хост для доступа к его узлам.

1. Выполните bootstrap базовой инфраструктуры кластера:

   ```shell
   dhctl bootstrap-phase base-infra --config config
   ```

1. Запустите вручную [bastion-хост](../../../../glossary.html#bastion-host) в подсети `<CLUSTER_PREFIX>-public-0`.

1. Продолжите установку кластера. На вопрос про кеш Terraform ответьте `y`:

   ```shell
   dhctl bootstrap --config config --ssh-...
   ```

## Использование существующего VPC (existingVPCID)

[Параметр `existingVPCID`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-existingvpcid) в ресурсе AWSClusterConfiguration
позволяет использовать уже существующий VPC для развертывания кластера Deckhouse Platform, вместо создания нового VPC автоматически.

Этот параметр может быть полезен в случаях, когда:

- инфраструктура AWS уже частично развернута;
- необходимо интегрироваться с другими сервисами или ресурсами, размещёнными в этом VPC;
- политика безопасности или архитектурные требования запрещают автоматическое создание VPC.

{% alert level="warning" %}
Если в существующем VPC уже есть Internet Gateway, попытка развертывания базовой инфраструктуры завершится ошибкой. В текущей версии Deckhouse Platform не поддерживается повторное использование уже существующего Internet Gateway.
{% endalert %}

Совместимость с другими параметрами:

- Если указан `existingVPCID`, не указывайте [`vpcNetworkCIDR`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-vpcnetworkcidr) — это взаимоисключающие параметры.
- [Параметр `nodeNetworkCIDR`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-nodenetworkcidr) можно (и желательно) указывать — он будет вложен в существующий VPC.
