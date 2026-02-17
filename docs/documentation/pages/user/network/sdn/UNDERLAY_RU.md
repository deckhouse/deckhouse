---
title: "Подключение физических сетевых интерфейсов к подам DPDK-приложений"
permalink: ru/user/network/sdn/underlay.html
description: |
  Подключение физических сетевых интерфейсов к подам через DRA для DPDK-приложений: режимы Shared и Dedicated.
search: DPDK приложения, UnderlayNetwork, физические интерфейсы, SR-IOV, VF PF
lang: ru
---

Если в вашем неймспейсе (проекте) размещаются высокопроизводительные рабочие нагрузки, требующие прямого доступа к оборудованию (например, приложения DPDK), можно использовать прямое подключение физических сетевых интерфейсов (Physical Functions и Virtual Functions) к подам через Kubernetes Dynamic Resource Allocation (DRA).

Физические сетевые интерфейсы могут подключаться в поды в одном из двух режимов:

- `Shared` — создается Virtual Functions (VF) из Physical Functions (PF) с использованием SR-IOV, несколько подов могут совместно использовать одно и то же оборудование.
- `Dedicated` — каждый под получает эксклюзивный доступ к полному PF.

Подробнее о возможностях и особенностях работы с Underlay-сетями в DKP — в разделе [«Настройка и подключение underlay-сетей для проброса аппаратных устройств»](../../../admin/configuration/network/sdn/cluster-preparing-and-sdn-enabling.html#настройка-и-подключение-underlay-сетей-для-проброса-аппаратных-устройств).

## Подключение физических сетевых интерфейсов к подам

Для подключения физических сетевых интерфейсов (PF/VF) напрямую в поды для DPDK-приложений необходимо:

1. Убедиться, что администратор добавил на ваш неймспейс [лейбл для использования Underlay-сетей](../../../admin/configuration/network/sdn/cluster-preparing-and-sdn-enabling.html#подготовка-неймспейса-для-использования-underlaynetwork).
1. Создать под c аннотацией, запрашивающей устройство из Underlay-сети.

### Создание пода c устройством из Underlay-сети

Создайте под, который запрашивает устройство из Underlay-сети. В аннотации пода `network.deckhouse.io/networks-spec` укажите параметры:

* `type: "UnderlayNetwork"` — указывает, что это запрос физического устройства;
* `name: "underlay-network-name"` — имя ресурса UnderlayNetwork, созданного администратором;
* `bindingMode` — режим привязки устройства (VFIO-PCI, DPDK или NetDev).

Пример конфигурации пода для режима DPDK (универсальный режим, который автоматически выбирает подходящий драйвер для вендора сетевого адаптера):

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dpdk-app
  namespace: mydpdk
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "dpdk-shared-network",
          "bindingMode": "DPDK"
        }
      ]
spec:
  containers:
  - name: dpdk-container
    image: dpdk-app:latest
    securityContext:
      privileged: false
      capabilities:
        add:
        - NET_ADMIN
        - NET_RAW
        - IPC_LOCK
    volumeMounts:
    - mountPath: /hugepages
      name: hugepage
    resources:
      limits:
        hugepages-2Mi: 4Gi
        memory: 4Gi
        cpu: 4
      requests:
        cpu: 4
        memory: 4Gi
    command: ["/bin/sh", "-c", "sleep infinity"]
  volumes:
  - name: hugepage
    emptyDir:
      medium: HugePages
```

{% alert level="info" %}
Для DPDK-приложений важно:

* Настроить `capabilities` (NET_ADMIN, NET_RAW, IPC_LOCK) для запуска в непривилегированном режиме вместо использования `privileged: true`;
* Подключить volumes с hugepages, так как DPDK требует hugepages для эффективного управления памятью.
{% endalert %}

{% alert level="info" %}
Для VF устройств в режиме Shared можно дополнительно указать `vlanID` в аннотации для настройки VLAN-тегирования на VF:

```yaml
network.deckhouse.io/networks-spec: |
  [
    {
      "type": "UnderlayNetwork",
      "name": "dpdk-shared-network",
      "bindingMode": "VFIO-PCI",
      "vlanID": 100
    }
  ]
```

{% endalert %}

После создания пода убедитесь, что устройство было выделено, проверив аннотацию `network.deckhouse.io/networks-status`:

```shell
d8 k -n mydpdk get pod dpdk-app -o jsonpath='{.metadata.annotations.network\.deckhouse\.io/networks-status}' | jq
```

Вы также можете проверить ResourceClaim, который был автоматически создан:

```shell
d8 k -n mydpdk get resourceclaim
```

Пример статуса пода с выделенным устройством UnderlayNetwork:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dpdk-app
  namespace: mydpdk
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "dpdk-shared-network",
          "bindingMode": "DPDK"
        }
      ]
    network.deckhouse.io/networks-status: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "dpdk-shared-network",
          "bindingMode": "DPDK",
          "netDevInterfaces": [
            {
              "name": "ens1f0",
              "mac": "00:1b:21:bb:aa:cc"
            }
          ],
          "conditions": [
            {
              "type": "Configured",
              "status": "True",
              "reason": "InterfaceConfiguredSuccessfully",
              "message": "",
              "lastTransitionTime": "2025-01-15T10:35:00Z"
            },
            {
              "type": "Negotiated",
              "status": "True",
              "reason": "Up",
              "message": "",
              "lastTransitionTime": "2025-01-15T10:35:00Z"
            }
          ]
        }
      ]
status:
  phase: Running
  ...
```
