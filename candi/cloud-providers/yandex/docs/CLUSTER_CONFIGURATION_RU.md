---
title: "Cloud provider — Yandex.Cloud: настройки провайдера"
---

## YandexClusterConfiguration
Схема размещения (layout) описывается структурой `YandexClusterConfiguration`:
* `layout` — название схемы размещения.
  * Варианты — `Standard`, `WithoutNAT` или `WithNATInstance` (описание ниже).
* `withNATInstance` — настройки для layout'а `WithNATInstance`.
  * `natInstanceExternalAddress` — внешний [зарезервированный IP адрес](faq.html#как-зарезервировать-публичный-ip-адрес) или адрес из `externalSubnetID` при указании опции.
  * `internalSubnetID` — ID подсети для внутреннего интерфейса.
  * `externalSubnetID` — при указании данной опции к узлу будет подключен дополнительный сетевой интерфейс, в который будет идти маршрут по умолчанию.
* `provider` — параметры подключения к API Yandex.Cloud.
  * `cloudID` — идентификатор облака.
  * `folderID` — идентификатор директории.
  * `serviceAccountJSON` — JSON, выдаваемый [yc iam key create](environment.html)
* `masterNodeGroup` — спецификация для описания NodeGroup мастера.
  * `replicas` — сколько мастер-узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [YandexInstanceClass](cr.html#yandexinstanceclass). Обязательными параметрами являются `cores`, `memory`, `imageID`.
    * `cores`
    * `memory`
    * `imageID`
    * `additionalLabels` — дополнительные лейблы, с которыми будут создаваться статические узлы.
    * **`externalIPAddresses`** — список внешних адресов. Количество элементов массива должно соответствовать `replicas`.
      * При отсутствии опции `externalSubnetID` нужно использовать или [зарезервированные публичные IP адреса](faq.html#как-зарезервировать-публичный-ip-адрес) или константу `Auto`.
      * При наличии опции `externalSubnetID` необходимо выбрать конкретные свободные IP из указанной подсети.
    * **`externalSubnetID`** [DEPRECATED] — при указании данной опции к узлу будет подключен дополнительный сетевой интерфейс, в который будет идти маршрут по умолчанию.
    * **`externalSubnetIDs`** — при указании данной опции к узлу будет подключен дополнительный сетевой интерфейс, в который будет идти маршрут по умолчанию.
      Также будет добавлен маршрут для internal интерфейса ноды на всю подсеть, указанную в `nodeNetworkCIDR`
      Количество элементов массива должно соответствовать `replicas`.
* `nodeGroups` — массив дополнительных NodeGroup для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NodeGroup:
  * `name` — имя NodeGroup, будет использоваться для генерации имени нод.
  * `replicas` — сколько узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [YandexInstanceClass](cr.html#yandexinstanceclass). Обязательными параметрами являются `cores`, `memory`, `imageID`.
    * `cores`
    * `memory`
    * `imageID`
    * `coreFraction`
    * `additionalLabels` — дополнительные лейблы, с которыми будут создаваться статические узлы.
    * **`externalIPAddresses`** — список внешних адресов. Количество элементов массива должно соответствовать `replicas`.
      * При отсутствии опции `externalSubnetID` нужно использовать или [зарезервированные публичные IP-адреса](faq.html#как-зарезервировать-публичный-ip-адрес) или константу `Auto`.
      * При наличии опции `externalSubnetID` необходимо выбрать конкретные свободные IP из указанной подсети.
    * **`externalSubnetID`** [DEPRECATED] — при указании данной опции к узлу будет подключен дополнительный сетевой интерфейс, в который будет идти маршрут по умолчанию.
    * **`externalSubnetIDs`** — при указании данной опции к узлу будет подключен дополнительный сетевой интерфейс, в который будет идти маршрут по умолчанию.
      Также будет добавлен маршрут для internal интерфейса ноды на всю подсеть, указанную в `nodeNetworkCIDR`
      Количество элементов массива должно соответствовать `replicas`.
  * `nodeTemplate` — настройки Node-объектов в Kubernetes, которые будут добавлены после регистрации ноды.
    * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.labels`
      * Пример:

        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```
    * `annotations` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.annotations`
      * Пример:

        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```
    * `taints` — аналогично полю `.spec.taints` из объекта [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core). **Внимание!** Доступны только поля `effect`, `key`, `values`.
      * Пример:

        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```
* `nodeNetworkCIDR` — данная подсеть будет разделена на **три** равных части и использована для создания подсетей в трёх зонах Yandex.Cloud.
* `existingNetworkID` — существующей VPC Network.
* `dhcpOptions` — список DHCP опций, которые будут установлены на все подсети. [Возможные проблемы](faq.html#проблемы-dhcpoptions-и-пути-их-решения) при использовании.
  * `domainName` — search домен.
  * `domainNameServers` — список адресов рекурсивных DNS.
* `sshPublicKey` — публичный ключ для доступа на ноды.
* `labels` — лейблы, проставляемые на ресурсы, создаваемые в Yandex.Cloud. Если поменять лейблы в рабочем кластере, то после конвержа
  необходимо пересоздать все машины, чтобы теги применились.
* `zones` — ограничение набора зон, в которых разрешено создавать ноды.
  * Опциональный параметр.
  * Формат — массив строк.
