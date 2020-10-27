---
title: "Сloud provider — OpenStack: настройки"
---

Модуль автоматически включается для всех облачных кластеров развёрнутых в OpenStack.

Количество и параметры процесса заказа машин в облаке настраиваются в custom resource [`NodeGroup`](/modules/040-node-manager/cr.html#nodegroup) модуля node-manager, в котором также указывается название используемого для этой группы узлов instance-класса (параметр `cloudInstances.classReference` NodeGroup).  Instance-класс для cloud-провайдера OpenStack — это custom resource [`OpenStackInstanceClass`](cr.html#openstackinstanceclass), в котором указываются конкретные параметры самих машин.

## Параметры

Настройки модуля устанавливаются автоматически на основании выбранной схемы размещения. В большинстве случаев нет необходимости в ручной конфигурации модуля.

Если вам необходимо настроить модуль, потому что, например, у вас bare metal кластер, для которого нужно включить
возможность добавлять дополнительные инстансы из OpenStack, то смотрите раздел как [настроить Hybrid кластер в OpenStack](usage.html#создание-гибридного-кластера).

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
