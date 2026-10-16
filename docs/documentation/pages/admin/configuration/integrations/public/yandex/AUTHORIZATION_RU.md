---
title: Подключение и авторизация в Yandex Cloud
permalink: ru/admin/integrations/public/yandex/authorization.html
lang: ru
---

Для того чтобы Deckhouse Platform (DP) мог управлять ресурсами в Yandex Cloud, необходимо:

- создать сервисный аккаунт;
- назначить ему необходимые IAM-роли;
- сгенерировать ключ авторизации;
- при необходимости зарезервировать публичный IP-адрес;
- обеспечить достаточные квоты на ресурсы в облаке.

Ниже приведены пошаговые действия по настройке подключения.

## Подготовка окружения

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

На виртуальных машинах должен быть установлен пакет `cloud-init`. После запуска виртуальной машины должны быть запущены службы, связанные с этим пакетом:

- `cloud-config.service`;
- `cloud-final.service`;
- `cloud-init.service`.

Чтобы проверить, запущены ли службы, выполните команды:

```shell
systemctl status cloud-config.service
systemctl status cloud-final.service
systemctl status cloud-init.service
```

{% alert level="warning" %}
Начиная с версии 1.77, новые кластеры в Yandex Cloud по умолчанию используют CNI Cilium с туннелированием трафика подов через VXLAN. В существующих кластерах CNI не меняется.

Убедитесь, что на всех узлах используется ядро Linux версии 5.8 или новее, а группы безопасности разрешают UDP-трафик между узлами. Список портов приведён в разделе [«Сетевое взаимодействие компонентов платформы»](../../../../reference/network_interaction.html), настройка CNI описана в разделе [«Настройка внутренней сети»](../../../configuration/network/internal/configuration.html).
{% endalert %}

## Создание сервисного аккаунта

Для управления ресурсами в Yandex Cloud через DP создайте сервисный аккаунт и выдайте ему права на редактирование.
Подробную инструкцию по созданию сервисного аккаунта смотрите в [документации Yandex Cloud](https://cloud.yandex.com/ru/docs/resource-manager/operations/cloud/set-access-bindings).

Для создания сервисного аккаунта выполните команду:

```shell
yc iam service-account create --name deckhouse
```

Команда вернёт информацию о созданном сервисном аккаунте:

```console
id: <USER_ID>
folder_id: <FOLDER_ID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

{% alert level="warning" %}
Сохраните `userID` и `folderID`, они понадобятся в следующих шагах.
{% endalert %}

## Назначение IAM-ролей

Для работы DP с ресурсами облака назначьте сервисному аккаунту следующие роли:

```shell
yc resource-manager folder add-access-binding --id <FOLDER_ID> --role compute.editor --subject serviceAccount:<USER_ID>
yc resource-manager folder add-access-binding --id <FOLDER_ID> --role vpc.admin --subject serviceAccount:<USER_ID>
yc resource-manager folder add-access-binding --id <FOLDER_ID> --role load-balancer.editor --subject serviceAccount:<USER_ID>
```

## Генерация авторизованного ключа

Создайте JSON-файл с авторизацией для использования в конфигурации:

```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```

Содержимое файла `deckhouse-sa-key.json` понадобится для секрета с учётными данными.

## Секрет с учётными данными

Ключ сервисного аккаунта хранится не в [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig), а в отдельном секрете. Провайдер читает его при запуске компонентов, которые обращаются к API Yandex Cloud.

Секрет создаётся при установке кластера вместе с остальными ресурсами первичной конфигурации. Если провайдер подключается к уже работающему кластеру, секрет применяется после того, как модуль создаст неймспейс, порядок описан в разделе [«Гибридный кластер с Yandex Cloud»](../../hybrid/yandex-hybrid.html). Секрет должен отвечать следующим требованиям:

- имя `d8-credentials`, неймспейс `d8-cloud-provider-yandex`;
- тип `cloud-provider.deckhouse.io/credentials`;
- поле `authScheme` со значением `serviceAccount`;
- поле `secret` с содержимым JSON-ключа сервисного аккаунта.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-yandex
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: serviceAccount
  secret: |
    <SERVICE_ACCOUNT_JSON>
```

Замените `<SERVICE_ACCOUNT_JSON>` на содержимое файла `deckhouse-sa-key.json`.

Чтобы сменить учётные данные, обновите поле `secret`:

```shell
d8 k -n d8-cloud-provider-yandex edit secret d8-credentials
```

В [схеме размещения WithNATInstance](layout.html#withnatinstance) API-ключ для мониторинга NAT-инстанса хранится во втором секрете `d8-credentials-exporter` в том же неймспейсе. У него поле `authScheme` со значением `apiToken`, а ключ записан в поле `secret`.

Вместо готового ключа в поле `secret` можно указать значение `Auto`. В этом случае при развёртывании инфраструктуры DP создаёт в каталоге сервисный аккаунт с именем `<CLUSTER_PREFIX>-monitoring`, выдаёт ему роль `monitoring.viewer` и генерирует для него API-ключ, который дальше использует экспортер метрик.

В кластерах, переведённых на ModuleConfig, ключ сервисного аккаунта переносится в секрет `d8-credentials` из параметра [`provider.serviceAccountJSON`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-provider-serviceaccountjson) ресурса [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration). API-ключ мониторинга переносится в секрет `d8-credentials-exporter` из параметра [`withNATInstance.exporterAPIKey`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-withnatinstance-exporterapikey).

## Проверка и увеличение квот

Убедитесь, что облачный аккаунт имеет необходимые квоты для развёртывания и масштабирования:
- Виртуальные процессоры: 64
- Объём SSD-дисков: 2000 ГБ
- Количество ВМ: 25
- Объём RAM: 256 ГБ

Увеличьте квоты через консоль Yandex Cloud, если необходимо.

## Резервирование публичного IP

Если используется схема размещения WithoutNAT или WithNATInstance, и требуется фиксированный внешний IP-адрес (например, для указания в [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses), [`natInstanceExternalAddress`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-natinstanceexternaladdress) или для bastion-хоста), выполните команду:

```shell
yc vpc address create --external-ipv4 zone=ru-central1-a
```

Пример вывода команды:

```console
id: e9b4cfmmnc1mhgij75n7
folder_id: b1gog0h9k05lhqe5d88l
created_at: "2020-09-01T09:29:33Z"
external_ipv4_address:
  address: 178.154.226.159
  zone_id: ru-central1-a
  requirements: {}
reserved: true
```

После выполнения этих шагов у вас будут все необходимые данные для настройки провайдера, порядок описан в разделе [«Конфигурация»](layout.html#конфигурация).
