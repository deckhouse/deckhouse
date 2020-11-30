---
title: "Сloud provider — OpenStack: настройки"
---

Модуль автоматически включается для всех облачных кластеров развёрнутых в OpenStack.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера OpenStack — это custom resource [`OpenStackInstanceClass`](cr.html#openstackinstanceclass), в котором указываются конкретные параметры самих машин.

## Параметры

Настройки модуля устанавливаются автоматически на основании выбранной схемы размещения. В большинстве случаев нет необходимости в ручной конфигурации модуля.

Если вам необходимо настроить модуль, потому что, например, у вас bare metal кластер, для которого нужно включить
возможность добавлять дополнительные инстансы из OpenStack, то смотрите раздел как [настроить Hybrid кластер в OpenStack](faq.html#как-поднять-гибридный-кластер).

Если у вас в кластере есть инстансы, для которых будут использоваться External Networks, кроме указанных в схеме размещения,
то их следует передавать в параметре

* `additionalExternalNetworkNames` — имена дополнительных сетей, которые могут быть подключены к виртуальной машине, и используемые `cloud-controller-manager` для проставления `ExternalIP` в `.status.addresses` в Node API объект.
  * Формат — массив строк.

### Пример

```yaml
cloudProviderOpenstack: |
  additionalExternalNetworkNames:
  - some-bgp-network
```

## Storage

Модуль автоматически создаёт StorageClasses, которые есть в OpenStack. А также позволяет отфильтровать ненужные, указанием их в параметре `exclude`.

* `exclude` — полные имена (или regex выражения имён) StorageClass, которые не будут созданы в кластере.
  * Формат — массив строк.
  * Опциональный параметр.
* `default` — имя StorageClass, который будет использоваться в кластере по умолчанию.
  * Формат — строка.
  * Опциональный параметр.
  * Если параметр не задан, фактическим StorageClass по умолчанию будет либо: 
    * Присутствующий в кластере произвольный StorageClass с default аннотацией.
    * Первый StorageClass из создаваемых модулем (в порядке из OpenStack).

```yaml
cloudProviderOpenstack: |
  storageClass:
    exclude:
    - .*-hdd
    - iscsi-fast
    default: ceph-ssd
```
