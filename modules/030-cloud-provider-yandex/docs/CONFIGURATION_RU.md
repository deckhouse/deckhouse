---
title: "Cloud provider — Yandex.Cloud: настройки"
---

## Параметры

> **Внимание!** При изменении конфигурационных параметров, приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse), **пересоздание существующих Machines не производится** (новые Machines будут создаваться с новыми параметрами). Пересоздание происходит только при изменении параметров `NodeGroup` и `YandexInstanceClass`. См. подробнее в документации модуля [node-manager](../../modules/040-node-manager/faq.html#как-пересоздать-эфемерные-машины-в-облаке-с-новой-конфигурацией).

* `additionalExternalNetworkIDs` — список Network ID, которые будут считаться `ExternalIP` при перечислении адресов у Node;
  * Формат — массив строк.
  * Опциональный параметр.

## Storage

Модуль автоматически создаёт StorageClass'ы, покрывающие все варианты дисков в Yandex:

| Тип | Имя StorageClass |
|---|---|
| network-hdd | network-hdd |
| network-ssd | network-ssd |
| network-ssd-nonreplicated | network-ssd-nonreplicated |

А также позволяет отфильтровать ненужные StorageClass'ы, указав их в параметре `exclude`:

* `exclude` — полные имена (или regex выражения имён) StorageClass, которые не будут созданы в кластере.
* `default` — имя StorageClass, который будет использоваться в кластере по умолчанию. Если параметр не задан, фактическим StorageClass по умолчанию будет либо:
  * Присутствующий в кластере произвольный StorageClass с default аннотацией;
  * Первый StorageClass из создаваемых модулем (в порядке из таблицы выше).

Пример:

```yaml
cloudProviderYandex: |
  storageClass:
    exclude:
    - .*-hdd
    default: network-ssd
```

### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и Yandex.Cloud API, после увеличения размера PVC необходимо:

1. Выполнить `kubectl cordon узел_где_находится_pod`;
2. Удалить Pod;
3. Убедиться, что увеличение размера произошло успешно. В объекте PVC *не будет* condition `Resizing`. 
  > **Внимание!** `FileSystemResizePending` не является проблемой;
4. Выполнить `kubectl uncordon узел_где_находится_pod`.

## LoadBalancer

Модуль подписывается на объекты Service с типом LoadBalancer и создаёт соответствующие NetworkLoadBalancer и TargetGroup в Yandex.Cloud.

Больше информации [в документации](https://github.com/flant/yandex-cloud-controller-manager) CCM.
