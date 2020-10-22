---
title: "Сloud provider — Yandex.Cloud: настройки"
---

## Параметры

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `YandexInstanceClass`. См. подробнее в документации модуля [node-manager](/modules/040-node-manager/faq.html#как-перекатить-эфемерные-машины-в-облаке-с-новой-конфигурацией).

* `additionalExternalNetworkIDs` — список Network ID, которые будут считаться `ExternalIP` при перечислении адресов у Node;
  * Формат — массив строк.
  * Опциональный параметр.

## Storage

Storage настраивать не нужно, модуль автоматически создаст 2 StorageClass'а, покрывающие все варианты дисков в Yandex: hdd или ssd.

1. `network-hdd`
2. `network-ssd`

### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и Yandex.Cloud API, после увеличения размера PVC нужно:

1. Выполнить `kubectl cordon нода_где_находится_pod`;
2. Удалить Pod;
3. Убедиться, что ресайз произошёл успешно. В объекте PVC *не будет* condition `Resizing`. **Внимание!** `FileSystemResizePending` не является проблемой;
4. Выполнить `kubectl uncordon нода_где_находится_pod`.

## LoadBalancer

Модуль подписывается на Service объекты с типом LoadBalancer и создаёт соответствующие NetworkLoadBalancer и TargetGroup в Yandex.Cloud.

Больше информации в [документации](https://github.com/flant/yandex-cloud-controller-manager) CCM.
