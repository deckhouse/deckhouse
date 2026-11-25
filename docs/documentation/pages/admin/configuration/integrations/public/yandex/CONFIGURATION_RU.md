---
title: Настройка интеграции с Yandex Cloud
permalink: ru/admin/integrations/public/yandex/configuration.html
lang: ru
---

## Конфигурация

Параметры [облачного провайдера](../../../../glossary.html#cloud-provider) Yandex Cloud задаются в четырёх ресурсах:

- [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) задаёт [схему размещения](layout.html#схемы-размещения), [сетевые параметры](network.html#сетевые-параметры-и-безопасность) и идентификаторы облака и каталога;
- секрет `d8-credentials` в неймспейсе `d8-cloud-provider-yandex` хранит ключ сервисного аккаунта, его формат описан в разделе [«Секрет с учётными данными»](authorization.html#секрет-с-учётными-данными);
- [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) описывает параметры виртуальных машин;
- [NodeGroup](/modules/node-manager/cr.html#nodegroup) описывает [группы узлов](../../../../glossary.html#node-group) и ссылается на [инстанс-класс](../../../../glossary.html#instance-class). Группа [master-узлов](../../../../glossary.html#master-node) описывается такой же группой с типом [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html).

Чтобы изменить настройки провайдера в работающем кластере, отредактируйте ModuleConfig:

```shell
d8 k edit mc cloud-provider-yandex
```

После изменения параметров узлов выполните команду:

```shell
dhctl converge
```

{% alert level="warning" %}
Если кластер был установлен со схемой [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration), необходима миграция на конфигурацию через ModuleConfig. Пока миграция не выполнена, срабатывает [алерт `D8CloudProviderYandexCloudMigrationPending`](../../../../reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending), а обновление Deckhouse Platform может блокироваться.

Инструкция приведена в разделе [«Как мигрировать облачный провайдер на конфигурацию через ModuleConfig?»](../../../../faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).

Если в настройках модуля был задан параметр `storageClass.default`, после миграции укажите StorageClass по умолчанию глобальным параметром [`defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass). Конверсия это значение не переносит.
{% endalert %}

### Пример конфигурации

Ниже приведён пример минимальной конфигурации кластера со [схемой размещения](../../../../glossary.html#layout) Standard, одним master-узлом и одной группой рабочих узлов:

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
        labels:
          billing: prod
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: master
spec:
  cores: 4
  memory: 8192
  imageID: <IMAGE_ID>
  etcdDiskSizeGB: 10
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: master
    minPerZone: 1
    maxPerZone: 1
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  cores: 4
  memory: 8192
  coreFraction: 50
  imageID: <IMAGE_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker
    minPerZone: 1
    maxPerZone: 3
    zones:
      - ru-central1-a
```

Где:

- `CLOUD_ID` и `FOLDER_ID` — идентификаторы облака и каталога Yandex Cloud, задаваемые параметрами [`cloudID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-cloudid) и [`folderID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-folderid) в секции `provider.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/). Их получение описано в разделе [«Подключение и авторизация в Yandex Cloud»](authorization.html);
- `SSH_PUBLIC_KEY` — публичный SSH-ключ для доступа к узлам, задаваемый параметром [`sshPublicKey`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-sshpublickey) в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/);
- `IMAGE_ID` — идентификатор образа операционной системы, из которого создаются виртуальные машины, задаваемый параметром [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) ресурса YandexInstanceClass. Если параметр не задан, используется образ из инстанс-класса группы `master`.

В ресурсе YandexInstanceClass для группы `master` необходимо указать параметр [`etcdDiskSizeGB`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-etcddisksizegb), который задаёт размер отдельного диска под etcd. Для остальных групп узлов этот параметр указывать нельзя.

Остальные параметры описаны в [настройках модуля](/modules/cloud-provider-yandex/configuration.html) и в описании ресурса [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass).

Ключ сервисного аккаунта в ModuleConfig не хранится, он задаётся [отдельным секретом](authorization.html#секрет-с-учётными-данными).
