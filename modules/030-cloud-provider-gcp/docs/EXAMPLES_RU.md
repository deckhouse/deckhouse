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

{% alert %}
Вложенная виртуализация поддерживается только на определённых типах машин. Список совместимых типов приведён [в документации GCP](https://cloud.google.com/compute/docs/instances/nested-virtualization/overview#supported_machine_types).
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: vm-nodes
spec:
  machineType: n2-standard-8
  enableNestedVirtualization: true
```

## Добавление дополнительных дисков

Чтобы подключить к инстансам дополнительные диски (например, для узлов хранилища LINSTOR, Ceph, NFS и аналогичных решений), задайте их в параметре `additionalDisks`:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: storage-nodes
spec:
  machineType: n1-standard-8
  additionalDisks:
  - size: 200
    type: pd-ssd
  - size: 500
    type: pd-standard
    autoDelete: true
```
