---
title: "Модуль cloud-provider-yandex"
---

## Содержимое модуля

1. cloud-controller-manager — контроллер для управления ресурсами облака из Kubernetes.
    * Синхронизирует метаданные Yandex Instances и Kubernetes Nodes.
    * Удаляет из Kubernetes ноды, которых более нет в Yandex.
    * Управляет таблицей маршрутизации, чтобы pod'ы имели связь друг с другом.
2. CSI storage — для заказа дисков в Yandex.
3. Регистрация в модуле [node-manager]({{ site.baseurl }}/modules/040-node-manager/), чтобы [YandexInstanceClass'ы](#yandexinstanceclass-custom-resource) можно было использовать в [CloudInstanceClass'ах]({{ site.baseurl }}/modules/040-node-manager/#nodegroup-custom-resource)
4. Включение необходимого CNI ([simple bridge]({{ site.baseurl }}/modules/035-cni-simple-bridge/)).

## Конфигурация

### Параметры

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `YandexInstanceClass`. См. подробнее в документации модуля [node-manager]({{ site.baseurl }}/guides/node-manager.html#как-перекатить-эфемерные-машины-в-облаке-с-новой-конфигурацией).

* `additionalExternalNetworkIDs` — список Network ID, которые будут считаться `ExternalIP` при перечислении адресов у Node;
  * Формат — массив строк.
  * Опциональный параметр.

#### Пример конфигурации

```yaml
cloudProviderYandexEnabled: "true"
cloudProviderYandex: |
  additionalExternalNetworkIDs:
  - enp6t4snovl2ko4p15em
```

### YandexInstanceClass custom resource

Ресурс описывает параметры группы Yandex Instance'ов, которые будет использовать machine-controller-manager из модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/). На этот ресурс ссылается ресурс `NodeGroup` из вышеупомянутого модуля.

Все опции идут в `.spec`.

* `cores` — количество ядер у создаваемых инстансов.
  * Формат — integer.
* `coreFraction` - базовый уровень производительности каждого ядра CPU у создаваемых инстансов. [Подробнее об уровнях производительности](https://cloud.yandex.ru/docs/compute/concepts/performance-levels).
  * Формат — integer.
  * По-умолчанию `100`.
  * Допустимые значения `0`, `5`, `20`, `50`, `100`.
* `memory` — количество оперативной памяти в мебибайтах у создаваемых инстансов.
  * Формат — integer.
* `gpus` — количество графических адаптеров у создаваемых инстансов.
  * Формат — integer.
  * По-умолчанию `0`.
  * Опциональный параметр.
* `platformID` — тип платформы instances. [Список существующих платформ](https://cloud.yandex.com/docs/compute/concepts/vm-platforms).
  * Формат — строка.
  * По-умолчанию `standard-v2`.
  * Опциональный параметр.
* `imageID` — идентификатор образа, который будет установлен в заказанные instance'ы.
  * Формат — строка.
  * По-умолчанию образ из masterInstanceClass из providerClusterConfiguration.
  * Опциональный параметр.
* `preemptible` — Заказывать ли preemptible instance.
  * Формат — bool.
  * По-умолчанию `false`.
  * Опциональный параметр.
* `diskType` — тип диска у инстансов. [Типы дисков](https://cloud.yandex.com/docs/compute/concepts/disk#disks_types).
  * Формат — строка.
  * По-умолчанию `network-ssd`.
  * Опциональный параметр.
* `diskSizeGB` — размер диска у инстансов.
  * Формат — integer. В ГиБ.
  * По-умолчанию `50` ГиБ.
  * Опциональный параметр.
* `assignPublicIPAddress` - Присваивать ли публичные ip адреса инстансам потерять.
  * Формат — bool.
  * По-умолчанию `false`.
  * Опциональный параметр.
* `mainSubnet` — позволяет переопределить имя основного subnet, к которому будет подключен инстанс, по-умолчанию
используется subnet для зоны из конфига deckhouse `zoneToSubnetIdMap`.
  * Формат — string.
  * Пример — `e9bnc7g9mu9mper9clk4`
* `additionalSubnets` — список subnet, которые будут подключены к инстансу.
  * Формат — массив строк.
  * Пример:

    ```yaml
    - enp6t4snovl2ko4p15em
    - enp34dkcinm1nr5999lu
    ```

* `labels` — Метки инстанса
  * Формат — key:value
  * Опциональный параметр.

#### Пример YandexInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

### Storage

Storage настраивать не нужно, модуль автоматически создаст 2 StorageClass'а, покрывающие все варианты дисков в Yandex: hdd или ssd.

1. `network-hdd`
2. `network-ssd`

#### Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer, CSI и Yandex.Cloud API, после увеличения размера PVC нужно:

1. Выполнить `kubectl cordon нода_где_находится_pod`;
2. Удалить Pod;
3. Убедиться, что ресайз произошёл успешно. В объекте PVC *не будет* condition `Resizing`. **Внимание!** `FileSystemResizePending` не является проблемой;
4. Выполнить `kubectl uncordon нода_где_находится_pod`.

### LoadBalancer

Модуль подписывается на Service объекты с типом LoadBalancer и создаёт соответствующие NetworkLoadBalancer и TargetGroup в Yandex.Cloud.

Больше информации в [документации](https://github.com/flant/yandex-cloud-controller-manager) CCM.
