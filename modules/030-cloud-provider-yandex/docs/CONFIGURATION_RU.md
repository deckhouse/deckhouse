---
title: "Cloud provider — Yandex Cloud: настройки"
---

> **Внимание.** При изменении настроек модуля **пересоздания существующих объектов `Machines` в кластере НЕ происходит** (новые объекты `Machine` будут создаваться с новыми параметрами). Пересоздание происходит только при изменении параметров `NodeGroup` и `YandexInstanceClass`. Подробнее в документации модуля [node-manager](/modules/node-manager/faq.html#как-пересоздать-эфемерные-машины-в-облаке-с-новой-конфигурацией).

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

## Storage

Модуль автоматически создает StorageClass'ы, покрывающие все варианты дисков в Yandex Cloud:

| Тип                     | Имя StorageClass           | Комментарии               |
|-------------------------|----------------------------|---------------------------|
| network-hdd              | network-hdd               |                           |
| network-ssd              | network-ssd               |                           |
| network-ssd-nonreplicated | network-ssd-nonreplicated|                           |
| network-ssd-io-m3         | network-ssd-io-m3        |Размер дисков должен быть кратен 93 ГБ |

Вы можете отфильтровать ненужные StorageClass с помощью параметра [`exclude`](#parameters-storageclass-exclude).

Параметр [`provision`](#parameters-storageclass-provision) позволяет создавать дополнительные StorageClass или переопределять параметры StorageClass, создаваемых модулем по умолчанию.

С помощью параметра [`blockSize`](#parameters-storageclass-provision-blocksize) можно задать [размер блока](https://cloud.yandex.ru/docs/compute/operations/disk-create/empty-disk-blocksize) для создаваемых дисков. От размера блока зависит максимальный размер диска: для значения `4Ki` максимальный размер составляет `8Ti`, а при каждом последующем увеличении размера блока удваивается — вплоть до `256Ti` при `128Ki`.

Пример StorageClass с размером блока `64Ki`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 1
  settings:
    storageClass:
      provision:
      - name: network-ssd-64k
        type: network-ssd
        blockSize: 64Ki
```

{% alert level="warning" %}
После создания диска изменить размер его блока нельзя. Изменение параметра `blockSize` приводит к пересозданию StorageClass, но не изменяет размер блока у ранее созданных томов.
{% endalert %}

## LoadBalancer

Модуль подписывается на объекты Service с типом `LoadBalancer` и создает соответствующие `NetworkLoadBalancer` и `TargetGroup` в Yandex Cloud.

Больше информации [в документации Kubernetes Cloud Controller Manager for Yandex Cloud](https://github.com/flant/yandex-cloud-controller-manager).

{% include module-settings.liquid %}
