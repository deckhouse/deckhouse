---
title: "Cloud provider — Yandex Cloud: настройки"
---

> **Внимание!** При изменении настроек модуля **пересоздания существующих объектов `Machines` в кластере НЕ происходит** (новые объекты `Machine` будут создаваться с новыми параметрами). Пересоздание происходит только при изменении параметров `NodeGroup` и `YandexInstanceClass`. Подробнее в документации модуля [node-manager](../../modules/node-manager/faq.html#как-пересоздать-эфемерные-машины-в-облаке-с-новой-конфигурацией).

{% include module-alerts.liquid %}

{% include module-conversion.liquid %}

## Storage

Модуль автоматически создает StorageClass'ы, покрывающие все варианты дисков в Yandex Cloud:

| Тип                     | Имя StorageClass           | Комментарии               |
|-------------------------|----------------------------|---------------------------|
| network-hdd              | network-hdd               |                           |
| network-ssd              | network-ssd               |                           |
| network-ssd-nonreplicated | network-ssd-nonreplicated|                           |
| network-ssd-io-m3         | network-ssd-io-m3        |Размер дисков должен быть кратен 93 ГБ |

Вы можете отфильтровать ненужные StorageClass'ы с помощью параметра [exclude](#parameters-storageclass-exclude).

## LoadBalancer

Модуль подписывается на объекты Service с типом `LoadBalancer` и создает соответствующие `NetworkLoadBalancer` и `TargetGroup` в Yandex Cloud.

Больше информации [в документации Kubernetes Cloud Controller Manager for Yandex Cloud](https://github.com/flant/yandex-cloud-controller-manager).

{% include module-settings.liquid %}
