---
title: "Cloud provider — Yandex Cloud: примеры"
---

Ниже представлен пример конфигурации cloud-провайдера Yandex Cloud.

## Пример конфигурации модуля

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: b1g0ldjdpa8rtm2ru3s4
        folderID: b1gq1vn5jqk0tmh4u7bp
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: ssh-rsa AAAAB3Nza...
        zones:
          - ru-central1-a
          - ru-central1-b
        labels:
          billing: prod
        externalSubnetIDs:
          worker:
            - e2lu8r1tbbtryhdpa9ro
        externalIPAddresses:
          worker:
            - Auto
    storage:
      parameters:
        excludedStorageClasses:
          - network-hdd
        provisionedStorageClasses:
          - name: network-ssd-64k
            type: network-ssd
            blockSize: 64Ki
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4snovl2ko4p15em
        additionalInternalNetworkIDs:
          - enpmhcdmtvs6ptdq3mkt
```

Ключ сервисного аккаунта в этом ресурсе не хранится, он задаётся [отдельным секретом](environment.html#секрет-с-учётными-данными).

## Использование зоны ru-central1-f

Чтобы размещать узлы в зоне `ru-central1-f`, явно укажите её в параметре [`zones`](configuration.html#parameters-nodes-parameters-zones) вместо одной из зон по умолчанию. В `zones` можно указать не более четырёх зон.

1. Выберите одну из зон по умолчанию, которую заменит `ru-central1-f`.
1. Укажите `ru-central1-f` и остальные выбранные зоны в параметре [`zones`](configuration.html#parameters-nodes-parameters-zones) ресурса [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля `cloud-provider-yandex`. Пример:

   ```yaml
   spec:
     settings:
       nodes:
         parameters:
           layout: Standard
           nodeNetworkCIDR: 10.241.32.0/20
           sshPublicKey: ssh-rsa AAAAB3Nza...
           zones:
             - ru-central1-a
             - ru-central1-b
             - ru-central1-d
             - ru-central1-f
   ```

   В примере исключена зона `ru-central1-e`.

   Deckhouse Platform создаёт подсеть в `ru-central1-f`, используя часть [`nodeNetworkCIDR`](configuration.html#parameters-nodes-parameters-nodenetworkcidr), закреплённую за исключённой зоной. В работающем кластере с зонами по умолчанию замена `ru-central1-e` на `ru-central1-f` удаляет подсеть в `ru-central1-e` и создаёт новую в `ru-central1-f`. Подсети остальных выбранных зон сохраняются.

### Использование существующих подсетей

Если задан параметр [`existingZoneToSubnetIDMap`](configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap), Deckhouse Platform использует указанные в параметре подсети, не создавая и не удаляя их. Ограничение в четыре зоны на `existingZoneToSubnetIDMap` не распространяется. Если [`zones`](configuration.html#parameters-nodes-parameters-zones) не задан, узлы размещаются во всех зонах `existingZoneToSubnetIDMap`, включая зоны вне набора по умолчанию.

### Изменение зон в работающем кластере

Чтобы добавить `ru-central1-f`, обновите параметр [`zones`](configuration.html#parameters-nodes-parameters-zones).

{% alert level="info" %}
При добавлении `ru-central1-f` Deckhouse Platform удаляет подсети исключённых зон по умолчанию. При замене `[ru-central1-a, ru-central1-b]` на `[ru-central1-a, ru-central1-b, ru-central1-f]` удаляются подсети в `ru-central1-d` и `ru-central1-e`. В `ru-central1-f` создаётся подсеть с диапазоном адресов одной из удалённых подсетей.
{% endalert %}

При изменении списка зон возможен конфликт диапазонов. Он возникает, если подсеть добавляемой зоны по умолчанию должна использовать диапазон подсети `ru-central1-f`. Deckhouse Platform отклоняет такое изменение с ошибкой. Чтобы устранить конфликт, примените изменения в два этапа:

1. Удалите `ru-central1-f` из [`zones`](configuration.html#parameters-nodes-parameters-zones) и примените изменения. Deckhouse Platform удалит подсеть в `ru-central1-f`.
1. Укажите итоговый список зон с `ru-central1-f` и повторно примените изменения.

Например, такой конфликт возможен при замене `ru-central1-d` на `ru-central1-e` с сохранением `ru-central1-f` в списке.

Если конфликта нет, список зон можно изменить напрямую. Подсеть `ru-central1-f` сохраняет свой диапазон адресов.

Если в схеме [`WithNATInstance`](layouts.html#withnatinstance) подсети NAT-инстанса не заданы явно, его размещение определяется по списку [`zones`](configuration.html#parameters-nodes-parameters-zones):

| Список зон | Размещение NAT-инстанса |
| --- | --- |
| Содержит только зоны по умолчанию | NAT-инстанс остаётся в `ru-central1-a` |
| Содержит зону вне набора по умолчанию, а `ru-central1-a` исключена | NAT-инстанс пересоздаётся в первой по алфавиту зоне из списка `zones` |

{% alert level="warning" %}
Исходящий трафик кластера прерывается на время пересоздания NAT-инстанса.
{% endalert %}

## Пример кастомного ресурса YandexInstanceClass

YandexInstanceClass описывает виртуальные машины, которые DP заказывает для группы узлов. Обязательны только два параметра, [`cores`](cr.html#yandexinstanceclass-v1-spec-cores) и [`memory`](cr.html#yandexinstanceclass-v1-spec-memory). Остальные параметры можно не задавать.

В примере ниже дополнительно заданы образ ОС, тип и размер диска, доля вычислительной мощности ядра и режим работы сетевого интерфейса:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  cores: 4
  memory: 8192
  coreFraction: 50
  imageID: fd8bhu2b2bsbnqkhhd2k
  diskType: network-ssd
  diskSizeGB: 50
  networkType: SoftwareAccelerated
```

## LoadBalancer

### Аннотации объекта Service

В кластере заданы значения по умолчанию для размещения ресурсов балансировщиков (сеть для целевой группы и подсеть для Listener). Эти значения выставляются автоматически во время развёртывания кластера и могут быть переопределены аннотациями на уровне конкретного Service.

> Поведение по умолчанию (внешний или внутренний LB) зависит от конфигурации кластера. Для явного выбора типа используйте аннотацию `yandex.cpi.flant.com/loadbalancer-external`.

В Yandex Cloud Controller Manager поддерживаются следующие аннотации:

1. `yandex.cpi.flant.com/target-group-network-id` — указывает NetworkID, в котором будет создана целевая группа для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-subnet-id` — задаёт SubnetID для слушателей (listeners) создаваемого балансировщика для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-address-ipv4` — задаёт предопределённый IPv4-адрес для слушателей (поддерживаются и внутренние, и внешние LB).
1. `yandex.cpi.flant.com/loadbalancer-external` — включает создание внешнего (external) LB для данного Service (используйте, если нужно явно создать внешний балансировщик). Переопределяет поведение по умолчанию.
1. `yandex.cpi.flant.com/target-group-name-prefix` — задаёт префикс имени целевой группы, которую использует LoadBalancer. Аннотация на Service определяет целевую группу, но не выбирает узлы. Чтобы включить узлы в целевую группу, задайте аннотацию с тем же значением в параметре [`spec.nodeTemplate.annotations`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-annotations) ресурса NodeGroup. Yandex Cloud Controller Manager включает в целевую группу все подходящие узлы с таким значением аннотации независимо от их NodeGroup. Имя целевой группы формируется как `<ANNOTATION_VALUE><YANDEX_CLOUD_CLUSTER_NAME><NETWORK_ID>`. Подробнее — в разделе [«Использование отдельной целевой группы для NodeGroup»](faq.html#использование-отдельной-целевой-группы-для-nodegroup).

Если для control plane или master-узлов создаются отдельные целевые группы, добавьте на master-узлы лейбл `node.kubernetes.io/exclude-from-external-load-balancers: ""`. Это предотвратит попытки контроллера автоматически добавлять master-узлы в новые целевые группы для балансировщиков.

Если вы создаёте собственный балансировщик для master-узлов и хотите, чтобы YCC также мог размещать свои балансировщики на master-узлах, заранее создайте целевую группу с именем по маске `${CLUSTER-NAME}${VPC.ID}`.

### Проверки состояния целевой группы

Параметры проверок состояния для создаваемых целевых групп балансировщика:

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — как часто запускать проверку, в секундах (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — сколько ждать ответа от эндпоинта, в секундах. Если за это время ответ не получен, проверка считается неуспешной (по умолчанию 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — сколько подряд неуспешных проверок нужно, чтобы пометить эндпоинт как неработоспособный (unhealthy) и исключить его из балансировки (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — сколько подряд успешных проверок нужно, чтобы вернуть эндпоинт в статус работоспособный (healthy) и снова включить его в балансировку (по умолчанию 2).
