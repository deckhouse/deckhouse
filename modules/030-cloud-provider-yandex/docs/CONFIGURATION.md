---
title: "Сloud provider — Yandex.Cloud: настройки"
---

## Параметры

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `YandexInstanceClass`. См. подробнее в документации модуля [node-manager](/modules/040-node-manager/faq.html#как-перекатить-эфемерные-машины-в-облаке-с-новой-конфигурацией).

* `additionalExternalNetworkIDs` — список Network ID, которые будут считаться `ExternalIP` при перечислении адресов у Node;
  * Формат — массив строк.
  * Опциональный параметр.

## Storage

Модуль автоматически создаёт StorageClasses, покрывающие все варианты дисков в Yandex:

| Тип | Имя StorageClass |
|---|---|
| network-hdd | network-hdd |
| network-ssd | network-ssd |

А также позволяет отфильтровать ненужные StorageClass, указанием их в параметре `exclude`.

* `exclude` — полные имена (или regex выражения имён) StorageClass, которые не будут созданы в кластере.
  * Формат — массив строк.
  * Опциональный параметр.
* `default` — имя StorageClass, который будет использоваться в кластере по умолчанию.
  * Формат — строка.
  * Опциональный параметр.
  * Если параметр не задан, фактическим StorageClass по умолчанию будет либо: 
    * Присутствующий в кластере произвольный StorageClass с default аннотацией.
    * Первый StorageClass из создаваемых модулем (в порядке из таблицы выше).

```yaml
cloudProviderYandex: |
  storageClass:
    exclude: 
    - .*-hdd
    default: network-ssd
```

### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и Yandex.Cloud API, после увеличения размера PVC нужно:

1. Выполнить `kubectl cordon нода_где_находится_pod`;
2. Удалить Pod;
3. Убедиться, что ресайз произошёл успешно. В объекте PVC *не будет* condition `Resizing`. **Внимание!** `FileSystemResizePending` не является проблемой;
4. Выполнить `kubectl uncordon нода_где_находится_pod`.

## LoadBalancer

Модуль подписывается на Service объекты с типом LoadBalancer и создаёт соответствующие NetworkLoadBalancer и TargetGroup в Yandex.Cloud.

Больше информации в [документации](https://github.com/flant/yandex-cloud-controller-manager) CCM.
