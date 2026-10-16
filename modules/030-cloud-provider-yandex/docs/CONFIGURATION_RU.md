---
title: "Cloud provider — Yandex Cloud: настройки"
---

Учётные данные для доступа к API Yandex Cloud Deckhouse Platform читает из секрета `d8-credentials`, формат которого описан в разделе [«Секрет с учётными данными»](environment.html#секрет-с-учётными-данными).

{% alert level="warning" %}
При изменении настроек модуля существующие объекты Machine не пересоздаются, с новыми параметрами создаются только новые объекты. Узлы пересоздаются при изменении параметров [NodeGroup](/modules/node-manager/cr.html#nodegroup) и [YandexInstanceClass](cr.html#yandexinstanceclass). Как пересоздать узлы вручную, описано в разделе [«Как пересоздать эфемерные машины в облаке с новой конфигурацией?»](/modules/node-manager/faq.html#как-пересоздать-эфемерные-машины-в-облаке-с-новой-конфигурацией).
{% endalert %}

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

## Конфигурация модуля

Все настройки провайдера, кроме учётных данных, задаются в ресурсе ModuleConfig модуля. Платформа применяет их и при создании кластера, и в работающем кластере.

Настройки сгруппированы по четырём параметрам верхнего уровня:

- [`provider`](#parameters-provider) задаёт идентификаторы облака и каталога Yandex Cloud, параметр обязателен;
- [`nodes`](#parameters-nodes) задаёт схему размещения, параметры сети и создаваемых узлов, параметр обязателен;
- [`storage`](#parameters-storage) задаёт StorageClass, которые создаёт модуль;
- [`ccm`](#parameters-ccm) задаёт работу cloud controller manager.

Подсистемы узлов, хранения данных и cloud controller manager отключаются вложенным параметром `disabled`.

Пример конфигурации модуля с настройками провайдера, узлов, хранилища и cloud controller manager:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: <SSH_PUBLIC_KEY>
    storage:
      parameters:
        excludedStorageClasses:
          - network-hdd
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4snovl2ko4p15em
```

Где:

- `CLOUD_ID` и `FOLDER_ID` — идентификаторы облака и каталога Yandex Cloud;
- `SSH_PUBLIC_KEY` — публичный SSH-ключ для доступа к узлам.

Параметры виртуальных машин задаются в ресурсе [YandexInstanceClass](cr.html#yandexinstanceclass), на который ссылается [NodeGroup](/modules/node-manager/cr.html#nodegroup).

## Storage

Модуль автоматически создаёт StorageClass для всех типов дисков Yandex Cloud:

| Тип                     | Имя StorageClass           | Примечание               |
|-------------------------|----------------------------|---------------------------|
| network-hdd              | network-hdd               |                           |
| network-ssd              | network-ssd               |                           |
| network-ssd-nonreplicated | network-ssd-nonreplicated| Размер дисков должен быть кратен 93 ГБ |
| network-ssd-io-m3         | network-ssd-io-m3        | Размер дисков должен быть кратен 93 ГБ |

Вы можете отфильтровать ненужные StorageClass с помощью параметра [`excludedStorageClasses`](#parameters-storage-parameters-excludedstorageclasses).

Параметр [`provisionedStorageClasses`](#parameters-storage-parameters-provisionedstorageclasses) позволяет создавать дополнительные StorageClass или переопределять параметры StorageClass, создаваемых модулем по умолчанию.

С помощью параметра [`blockSize`](#parameters-storage-parameters-provisionedstorageclasses-blocksize) можно задать [размер блока](https://cloud.yandex.ru/docs/compute/operations/disk-create/empty-disk-blocksize) для создаваемых дисков. От размера блока зависит максимальный размер диска. Для значения `4Ki` он составляет `8Ti` и удваивается с каждым следующим размером блока, вплоть до `256Ti` при `128Ki`.

Пример конфигурации модуля `cloud-provider-yandex` с дополнительным StorageClass, для которого задан размер блока `64Ki`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 192.168.12.0/24
        sshPublicKey: <SSH_PUBLIC_KEY>
    storage:
      parameters:
        provisionedStorageClasses:
          - name: network-ssd-64k
            type: network-ssd
            blockSize: 64Ki
```

{% alert level="warning" %}
После создания диска изменить размер его блока нельзя. Изменение параметра `blockSize` приводит к пересозданию StorageClass, но не изменяет размер блока у ранее созданных томов.
{% endalert %}

## LoadBalancer

Модуль отслеживает объекты Service с типом `LoadBalancer` и создаёт для них [сетевой балансировщик](https://yandex.cloud/ru/docs/network-load-balancer/concepts/) и [целевую группу](https://yandex.cloud/ru/docs/network-load-balancer/concepts/target-resources) в Yandex Cloud. Аннотации, которыми настраивается балансировщик, перечислены в разделе [«Аннотации объекта Service»](examples.html#аннотации-объекта-service).

Больше информации [в документации Kubernetes Cloud Controller Manager for Yandex Cloud](https://github.com/flant/yandex-cloud-controller-manager).

{% include module-settings.liquid %}
