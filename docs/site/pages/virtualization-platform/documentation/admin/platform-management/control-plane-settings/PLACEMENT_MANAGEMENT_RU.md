---
title: "Размещение компонентов"
permalink: ru/virtualization-platform/documentation/admin/platform-management/control-plane-settings/placement-management.html
lang: ru
---

## Стратегии размещения

Для компонентов управления виртуализации предусмотрено 3 стратегии размещения:

### master

Компоненты размещаются на мастер-узлах. Это компоненты, реализующие ApiService, либо компоненты, в составе которых запускается ValidatingWebhook или MutatingWebhook.

### system

Компоненты с этой стратегией по умолчанию размещаются на мастер-узлах.

Однако, создав NodeGroup system или virtualization, можно снять нагрузку с мастер-узлов и перенести управляющие компоненты виртуализации на выделенные узлы.

### any-node

Это набор tolerations, благодаря которому компонент может быть запущен на любом узле в кластере.

## Узлы для стратегии system

Чтобы выделить узлы для стратегии system, нужно создать NodeGroup `system` и добавить в неё узлы.

```shell
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: Static
EOF
```

Чтобы узел добавился в NodeGroup `system`, его StaticInstance должен иметь лейбл `node-role.deckhouse.io/system` (подробнее в разделе про добавление узла [с помощью CAPS и label selector](../node-management/adding-node.html#caps-with-label-selector)).

Например:

```shell
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
 name: system-1
 labels:
   node-role.deckhouse.io/system: ""
spec:
 address: "<SERVER-SYSTEM-IP1>"
 credentialsRef:
   kind: SSHCredentials
   name: system-1-credentials
EOF
```

Стратегию system используют другие компоненты платформы, например, prometheus. Поэтому, создавая system-узлы, нужно учитывать, что на них переедут некоторые компоненты платформы. 
Для выделения узлов под компоненты виртуализации нужно создать NodeGroup `virtualization`, на узлах этой группы компоненты платформы не будет размещаться.

```shell
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: virtualization
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/virtualization: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: virtualization
  nodeType: Static
EOF
```

## Ограничение размещения виртуальных машин

TODO заготовка про ограничения для virt-handler.
