---
title: Управление узлами в Yandex Cloud
permalink: ru/admin/integrations/public/yandex/node-management.html
lang: ru
---

{% alert level="warning" %}
Управление CloudEphemeral-узлами переводится с Machine Controller Manager (MCM) на Cluster API (CAPI). Существующие [NodeGroup](/modules/node-manager/cr.html#nodegroup) продолжают использовать MCM, а новые по умолчанию создаются с использованием CAPI. Порядок миграции существующих групп описан в разделе [«Как мигрировать группы узлов на Cluster API (CAPI)»](../../../../faq.html#как-мигрировать-группы-узлов-на-cluster-api-capi).
{% endalert %}

## Особенности применения изменений

Deckhouse Platform не пересоздаёт уже существующие объекты Machine при изменении параметров. Пересоздание узлов происходит только при изменении:

- параметров в [секции NodeGroup](/modules/node-manager/cr.html#nodegroup);
- [параметров YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass).

Это поведение помогает избежать лишних операций и простоя существующих узлов, однако требует ручного вмешательства при необходимости пересоздать машины.

Если вы изменили [настройки провайдера](/modules/cloud-provider-yandex/configuration.html) в [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) `cloud-provider-yandex` (например, параметры провайдера, [схемы размещения](../../../../glossary.html#layout) или подсетей), чтобы изменения вступили в силу, выполните команду:

```shell
dhctl converge
```

Команда инициирует пересчёт конфигурации и приводит текущее состояние кластера в соответствие с описанным в ресурсах.

## Изменение networkType не пересоздаёт CloudEphemeral-узлы

В кластерах, использующих Machine Controller Manager (MCM), изменение параметра [`networkType`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-networktype) в ресурсе YandexInstanceClass не приводит к автоматическому пересозданию существующих CloudEphemeral-узлов. Хотя MachineClass обновляется, тип сетевого ускорения у уже созданных виртуальных машин в Yandex Cloud не изменяется.

Deckhouse Platform намеренно не учитывает параметр `networkType` при определении необходимости пересоздания CloudEphemeral-узлов в MCM. Если бы он учитывался, обновление Deckhouse Platform привело бы к пересозданию CloudEphemeral-узлов во всех кластерах, где `networkType` уже указан, даже если его значение не изменялось.

В Cluster API параметр `networkType` учитывается при определении необходимости пересоздания узлов, поэтому его изменение запускает их обновление автоматически. Новые [CloudEphemeral](../../../../glossary.html#cloud-ephemeral-node) NodeGroup в Yandex по умолчанию используют CAPI.

Если вы изменили `networkType` в кластере на MCM и хотите, чтобы новое значение применилось к уже существующим CloudEphemeral-узлам, пересоздайте их вручную — инструкция в [документации](/modules/node-manager/faq.html#как-пересоздать-эфемерные-машины-в-облаке-с-новой-конфигурацией).

## Интеграция вручную созданных ВМ

Deckhouse Platform позволяет подключать существующие виртуальные машины в Yandex Cloud к Kubernetes-кластеру в качестве узлов. Такие узлы называются [CloudStatic](../../../../glossary.html#cloud-static-node), поскольку они не управляются напрямую [модулем `node-manager`](/modules/node-manager/), но могут использоваться в составе кластера.

Чтобы вручную подключить виртуальную машину в качестве CloudStatic-узла, необходимо:

1. Узнать актуальное значение `nodeNetworkCIDR` из кластера:

   ```shell
   d8 k get mc cloud-provider-yandex -o jsonpath='{.spec.settings.nodes.parameters.nodeNetworkCIDR}{"\n"}'
   ```

   Результатом будет строка вида:

   ```console
   192.168.12.0/24
   ```

   Это значение необходимо скопировать и указать как `value` в метаданных виртуальной машины.

1. Задать параметр `node-network-cidr` в метаданных ВМ:

   ```yaml
   key: node-network-cidr
   value: <NODE_NETWORK_CIDR>
   ```

   Параметр `node-network-cidr` должен совпадать со значением параметра [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) в ModuleConfig.

## Замена образа ОС

Параметр [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) ресурса YandexInstanceClass задаёт образ узлов типа `CloudEphemeral`. При [настройке провайдера через ModuleConfig](../../../../faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec) этот параметр также задаёт образ узлов типа `CloudPermanent` и master-узлов.

При настройке через YandexClusterConfiguration образ узлов типа `CloudPermanent` и master-узлов задаётся параметром `imageID` в секциях `nodeGroups[].instanceClass` и `masterNodeGroup.instanceClass` соответственно.

Порядок замены описан в разделе [«Как заменить образ ОС в облачном кластере?»](../../../../faq.html#как-заменить-образ-ос-в-облачном-кластере).
