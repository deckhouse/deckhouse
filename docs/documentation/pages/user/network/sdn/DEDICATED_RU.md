---
title: "Дополнительные сети для использования в прикладных подах"
permalink: ru/user/network/sdn/dedicated.html
description: |
  Создание и подключение дополнительных программно-определяемых сетей для подов: кластерные сети и сети проекта.
search: дополнительные сети, сеть проекта, кластерная сеть, Network, NetworkClass
lang: ru
---

В DKP реализована возможность использования дополнительных программно-определяемых сетей (далее — дополнительные сети) для прикладных нагрузок (поды, виртуальные машины). Вы можете использовать сети следующих типов:

- Кластерная (общедоступная) — сеть, общедоступная в каждом проекте, настраивается и управляется администратором. Пример — публичная WAN-сеть или shared-сеть обмена трафиком между проектами. Для создания такой сети и ее использования для прикладных подов обратитесь к администратору кластера.
- Сеть проекта (пользовательская сеть) — сеть, доступная в рамках неймспейса, создается и управляется пользователем c использованием предоставленного администратором манифеста NetworkClass.

Подробнее о дополнительных программно-определяемых сетях — в разделе [«Настройка и подключение дополнительных виртуальных сетей для использования в прикладных подах»](../../../admin/configuration/network/sdn/configure.html#настройка-и-подключение-дополнительных-виртуальных-сетей-для-использования-в-прикладных-подах).

## Создание сети проекта (пользовательской сети)

Для создания сети для проекта используйте кастомные ресурсы [Network](/modules/sdn/cr.html#network) и [NetworkClass](/modules/sdn/cr.html#networkclass) (предоставляется администратором):

1. Создайте и примените манифест объекта Network, указав в поле `spec.networkClass` имя NetworkClass, полученное у администратора:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
     name: my-network
     namespace: my-namespace
   spec:
     networkClass: my-network-class # Имя NetworkClass, полученное от администратора.
   ```

   > Поддерживается статическое определение номера VLAN ID из пула, выданного администратором кластера. Если значение поля `spec.vlan.id` не указано, VLAN ID будет назначен динамически.

1. После создания объекта Network проверьте его статус:

   ```shell
   d8 k -n my-namespace get network my-network -o yaml
   ```

   Пример статуса объекта Network:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
   ...
   status:
     bridgeName: d8-br-600
     conditions:
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: All node interface attachments are ready
       reason: AllNodeInterfaceAttachmentsAreReady
       status: "True"
       type: AllNodeAttachementsAreReady
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: Network is operational
       reason: NetworkReady
       status: "True"
       type: Ready
     nodeAttachementsCount: 1
     observedGeneration: 1
     readyNodeAttachementsCount: 1
     vlanID: 600
   ```

После создания сети ее можно [подключать к подам](#подключение-дополнительных-сетей-к-подам).

## Подключение дополнительных сетей к подам

Вы можете подключать к подам кластерные сети и сети проекта. Для этого используйте аннотацию пода, в которой укажите параметры подключаемых дополнительных сетей.

Пример манифеста пода с добавлением двух дополнительных сетей (кластерной `my-cluster-network` и сети проекта `my-network`):

> В поле `ifName` (опционально) задается имя TAP-интерфейса внутри пода. В поле `mac` (опционально) задается MAC-адрес, который следует назначить TAP-интерфейсу.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-additional-networks
  namespace: my-namespace
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "Network",
          "name": "my-network",
          "ifName": "veth_mynet",
          "mac": "aa:bb:cc:dd:ee:ff"
        },
        {
          "type": "ClusterNetwork",
          "name": "my-cluster-network",
          "ifName": "veth_public"
        }
      ]
spec:
  containers:
    - name: app
    # остальные параметры...
```

## IPAM для дополнительных сетей

Механизм IPAM (IP Address Management) позволяет выделять IP-адреса (поддерживаются IPv4-адреса) из пулов и назначать их на дополнительные сетевые интерфейсы подов, подключаемых к кластерным сетям и сетям проекта.

Выделением IP-адресов для подключения к кластерным сетям занимается администратор кластера. Он включает, настраивает IPAM для сетей и определяет пул IP-адресов для них. Назначать адреса и настраивать IPAM в сетях проекта могут пользователи.

### Особенности использования IPAM в кластере DKP

IPAM в кластере DKP имеет следующие особенности использования:

- IPAM включается **на уровне сети** через параметр `spec.ipam.ipAddressPoolRef` объекта Network или ClusterNetwork (для ClusterNetwork IPAM включает администратор кластера).
- Назначение IP-адреса на интерфейс пода описывается в добавляемой к поду аннотации `network.deckhouse.io/networks-spec` через поля:
  - `ipAddressNames` — список объектов [IPAddress](/modules/sdn/cr.html#ipaddress), которые нужно назначить на данный интерфейс (если параметр не указан, IPAddress может создаваться автоматически).
  - `skipIPAssignment` — управление резервированием/отслеживанием IPAddress. Если `skipIPAssignment: true`, включается резервирование/отслеживание IPAddress, но IP-адрес **не назначается** на интерфейс внутри пода (вариант для продвинутого использования).
- Поддерживается назначение **только IPv4-адресов** на дополнительные сетевые интерфейсы подов.

{% alert level="warning" %}
Если в одном поде подключено несколько дополнительных сетей с включенным IPAM, рекомендуется [явно задавать
`ipAddressNames`](#ручное-явное-создание-ipaddress-с-типом-auto) для каждого интерфейса (создавая отдельные IPAddress). Автоматически создаваемый `IPAddress` привязан к поду и может не подходить для нескольких IPAM-сетей одновременно.
{% endalert %}

### Выделение пула IP-адресов для сети проекта и включение IPAM

> Чтобы выделить пул адресов для [сети проекта](#создание-сети-проекта-пользовательской-сети), создайте ресурс [IPAddressPool](/modules/sdn/cr.html#ipaddresspool) **в том же неймспейсе**, что и сеть проекта (поды, подключаемые к сети).

Для выделения пула адресов и их назначения на сетевые интерфейсы подов, подключаемых к сети проекта, выполните следующие действия:

1. Создайте пул адресов. Для этого используйте ресурс [IPAddressPool](/modules/sdn/cr.html#ipaddresspool).

   Пример:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: IPAddressPool
   metadata:
     name: my-net-pool
     namespace: my-namespace
   spec:
     leaseTTL: 1h
     pools:
       - network: 192.168.10.0/24
         ranges:
           - 192.168.10.50-192.168.10.200
         routes:
           - destination: 10.10.0.0/16
             via: 192.168.10.1
   ```

   > Параметр [`spec.pools[].ranges`](/modules/sdn/cr.html#ipaddresspool-v1alpha1-spec-pools-ranges) опционален. Если он не указан, доступным считается весь CIDR из [`spec.pools[].network`](/modules/sdn/cr.html#ipaddresspool-v1alpha1-spec-pools-network) (за исключением network/broadcast адресов, см. поведение `/31` и `/32`).

1. Включите IPAM в дополнительной сети. Для этого в параметре [`spec.ipam.ipAddressPoolRef`](/modules/sdn/cr.html#network-v1alpha1-spec-ipam-ipaddresspoolref) ресурса Network укажите параметры созданного на предыдущем шаге IPAddressPool.

   Пример:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
     name: my-network
     namespace: my-namespace
   spec:
     networkClass: my-network-class
     ipam:
       ipAddressPoolRef:
         kind: IPAddressPool
         name: my-net-pool
   ```

После выделения пула IP-адресов для сети проекта их можно назначать на сетевые интерфейсы подов, подключаемых к этой сети.

### Назначение IP-адресов на сетевые интерфейсы подов, подключаемых к дополнительной сети

В DKP реализована [автоматическая выдача IP-адресов](#автоматическая-выдача-ip-адресов) для дополнительных интерфейсов подов, а также возможность [ручного назначения конкретных статических IP-адресов](#ручное-назначение-статического-ip-адреса-на-дополнительный-интерфейс-пода) на дополнительные интерфейсы подов.

#### Автоматическая выдача IP-адресов

Чтобы IP-адрес для дополнительного сетевого интерфейса пода был выбран автоматически из пула, добавьте к поду аннотацию `network.deckhouse.io/networks-spec`. В этой аннотации укажите параметры сети с включенным IPAM.

Пример (IP-адрес будет выбран автоматически из пула, созданного для сети `my-network` и назначен на интерфейс `net1`):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-ipam
  namespace: my-namespace
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "Network",
          "name": "my-network",
          "ifName": "net1"
        }
      ]
spec:
  containers:
    - name: app
      image: nginx
```

В таком случае будет автоматически создан объект [IPAddress](/modules/sdn/cr.html#ipaddress) (тип `Auto`) и из прикрепленного к дополнительной сети (в примере — `my-network`) пула будет автоматически выбран IP-адрес и назначен на сетевой интерфейс пода.

##### Ручное (явное) создание IPAddress с типом `Auto`

Также можно **вручную** создать объект [IPAddress](/modules/sdn/cr.html#ipaddress) с `spec.type: Auto` (без указания параметра `static.ip`). В этом случае контроллер выделит свободный адрес из пула прикрепленного к дополнительной сети (в примере — `my-network`), а вы сможете привязать его к конкретному интерфейсу пода через параметр `ipAddressNames` в аннотации `network.deckhouse.io/networks-spec`.

Пример:

1. Создайте объект IPAddress:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: IPAddress
   metadata:
     name: app-net1-auto
     namespace: my-namespace
   spec:
     networkRef:
       kind: Network
       name: my-network
     type: Auto
   ```

1. Назначьте IP-адрес из пула на интерфейс пода:

   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: app-with-manual-auto-ip
     namespace: my-namespace
     annotations:
       network.deckhouse.io/networks-spec: |
         [
           {
             "type": "Network",
             "name": "my-network",
             "ifName": "net1",
             "ipAddressNames": ["app-net1-auto"]
           }
         ]
   spec:
     containers:
       - name: app
         image: nginx
   ```

#### Ручное назначение статического IP-адреса на дополнительный интерфейс пода

Чтобы назначить конкретный статический IP-адрес на дополнительный интерфейс пода, выполните следующие шаги:

1. Создайте IPAddress в неймспейсе пода и укажите, для какой сети он предназначен, и какой IP-адрес требуется:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: IPAddress
   metadata:
     name: app-net1-static
     namespace: my-namespace
   spec:
     networkRef:
       kind: Network
       name: my-network
     type: Static
     static:
       ip: 192.168.10.50
   ```

1. Подключите сеть к поду и укажите созданный IPAddress в параметре `ipAddressNames`:

   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: app-with-static-ip
     namespace: my-namespace
     annotations:
       network.deckhouse.io/networks-spec: |
         [
           {
             "type": "Network",
             "name": "my-network",
             "ifName": "net1",
             "ipAddressNames": ["app-net1-static"]
           }
         ]
   spec:
     containers:
       - name: app
         image: nginx
   ```

### Проверка назначения IP-адреса интерфейсу

Чтобы проверить, что IP-адрес назначен на интерфейс, выполните следующие шаги:

1. Проверьте выделенный адрес и фазу у `IPAddress` (фаза должна быть `Allocated`):

   ```shell
   d8 k -n my-namespace get ipaddress app-net1-static -o yaml
   ```

   Пример вывода:

   ```console
   NAME               TYPE   KIND      NAME    ADDRESS        NETWORK           PHASE       AGE
   ipaddress-auto-1   Auto   Network   mynet   192.168.12.1   192.168.12.0/24   Allocated   4d1h
   ipaddress-auto-2   Auto   Network   mynet   192.168.12.2   192.168.12.0/24   Allocated   4d1h
   ```

1. Проверьте аннотацию пода `network.deckhouse.io/networks-status` (включая `ipAddressConfigs` и маршруты):

   ```shell
   d8 k -n my-namespace get pod app-with-static-ip -o jsonpath='{.metadata.annotations.network\.deckhouse\.io/networks-status}   ' | jq
   ```

   Пример вывода:

   ```json
   [
     {
       "type": "Network",
       "name": "mynet",
       "ifName": "aabbcc",
       "mac": "ae:1c:68:7a:00:8f",
       "vlanID": 0,
       "ipAddressConfigs": [
         {
           "name": "ipaddress-auto-1",
           "address": "192.168.12.1",
           "network": "192.168.12.0/24"
         }
       ],
       "conditions": [
         {
           "type": "Configured",
           "status": "True",
           "lastTransitionTime": "2026-02-26T10:06:49Z",
           "reason": "InterfaceConfiguredSuccessfully",
           "message": ""
         },
         {
           "type": "Negotiated",
           "status": "True",
           "lastTransitionTime": "2026-02-26T10:06:49Z",
           "reason": "Up",
           "message": ""
         }
       ]
     }
   ]
   ```
