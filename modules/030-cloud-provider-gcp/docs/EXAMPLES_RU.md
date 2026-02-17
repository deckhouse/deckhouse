---
title: "Cloud provider — GCP: примеры"
---

## Пример кастомного ресурса `GCPInstanceClass`

Ниже представлен простой пример конфигурации кастомного ресурса `GCPInstanceClass`:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: test
spec:
  machineType: n1-standard-1
```

## Включение вложенной виртуализации

Для запуска виртуальных машин (например, KVM) внутри GCP-инстансов необходимо включить вложенную виртуализацию.

> **Внимание.** Поддерживается только на определённых типах машин. Список совместимых типов приведён в [документации GCP](https://cloud.google.com/compute/docs/instances/nested-virtualization/overview#supported_machine_types).

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: vm-nodes
spec:
  machineType: n2-standard-8
  nestedVirtualization: true
```

## Добавление дополнительных дисков

Для подключения дополнительных дисков к инстансам (например, для узлов хранилища LinStor, Ceph, NFS):

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: storage-nodes
spec:
  machineType: n1-standard-8
  additionalDisks:
  - sizeGb: 200
    type: pd-ssd
  - sizeGb: 500
    type: pd-standard
    autoDelete: true
```

## Настройка политик безопасности на узлах

На виртуальных машинах кластера в GCP может возникнуть необходимость ограничить или расширить входящий и исходящий трафик по различным причинам. Некоторые из них могут включать:

- Разрешение подключения к узлам кластера с виртуальных машин из другой подсети.
- Разрешение подключения к портам статического узла для работы приложения.
- Ограничение доступа к внешним ресурсам или другим виртуальным машинам в облаке по требованию службы безопасности.

Для всего этого необходимо применять дополнительные network tags.

## Установка дополнительных network tags на статических и master-узлах

Данный параметр можно задать либо при создании кластера или в уже существующем кластере. В обоих случаях дополнительные network tags указываются в `GCPClusterConfiguration`:

- для master-узлов — в секции `masterNodeGroup` в поле `additionalNetworkTags`;
- для статических узлов — в секции `nodeGroups` в конфигурации, описывающей соответствующую nodeGroup, в поле `additionalNetworkTags`.

Поле `additionalNetworkTags` содержит массив строк с именами network tags.

## Установка дополнительных network tags на эфемерных узлах

Необходимо указать параметр `additionalNetworkTags` для всех [`GCPInstanceClass`](cr.html#gcpinstanceclass) в кластере, которым нужны дополнительные network tags.
