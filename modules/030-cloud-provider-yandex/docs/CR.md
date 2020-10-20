---
title: "Сloud provider — Yandex.Cloud: Custom Resource"
---

## YandexInstanceClass

Ресурс описывает параметры группы Yandex Instance'ов, которые будет использовать machine-controller-manager из модуля [node-manager](/modules/040-node-manager/). На этот ресурс ссылается ресурс `NodeGroup` из вышеупомянутого модуля.

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
* `assignPublicIPAddress` - Присваивать ли публичные ip адреса инстансам.
  * Формат — bool.
  * По-умолчанию `false`.
  * Опциональный параметр.
* `mainSubnet` — позволяет переопределить имя основного subnet, к которому будет подключен инстанс, по умолчанию
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

### Пример YandexInstanceClass

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: YandexInstanceClass
metadata:
  name: test
spec:
  cores: 4
  memory: 8192
```

