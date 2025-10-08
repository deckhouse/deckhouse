---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/yandex/authorization.html
lang: ru
---

Для того чтобы Deckhouse Kubernetes Platform (DKP) мог управлять ресурсами в Yandex Cloud, необходимо:

- создать сервисный аккаунт;
- назначить ему необходимые IAM-роли;
- сгенерировать авторизационный ключ;
- при необходимости — зарезервировать публичный IP-адрес;
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

## Создание сервисного аккаунта

Для управления ресурсами в Yandex Cloud через DKP создайте сервисный аккаунт и выдайте ему права на редактирование.
Подробную инструкцию по созданию сервисного аккаунта смотрите в [документации Yandex Cloud](https://cloud.yandex.com/ru/docs/resource-manager/operations/cloud/set-access-bindings).

Для создания сервисного аккаунта выполните команду:

```shell
yc iam service-account create --name deckhouse
```

Команда вернёт информацию о созданном сервисном аккаунте:

```console
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

{% alert level="warning" %}
Сохраните `userID` и `folderID` — они понадобятся в следующих шагах.
{% endalert %}

## Назначение IAM-ролей

Для работы DKP с ресурсами облака назначьте сервисному аккаунту следующие роли:

```shell
yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
```

## Генерация авторизованного ключа

Создайте JSON-файл с авторизацией для использования в конфигурации:

```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```

Содержимое файла `deckhouse-sa-key.json` используйте в поле `provider.serviceAccountJSON` при описании конфигурации кластера.

## Проверка и увеличение квот

Убедитесь, что облачный аккаунт имеет необходимые квоты для развёртывания и масштабирования:
- Виртуальные процессоры: 64
- Объём SSD-дисков: 2000 ГБ
- Количество ВМ: 25
- Объём RAM: 256 ГБ

Увеличьте квоты через консоль Yandex Cloud, если необходимо.

## Резервирование публичного IP

Если используется схема размещения WithoutNAT или WithNATInstance, и требуется фиксированный внешний IP-адрес (например, для указания в [`externalIPAddresses`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-externalipaddresses), [`natInstanceExternalAddress`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-withnatinstance-natinstanceexternaladdress) или для bastion-хоста), выполните команду:

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

После выполнения этих шагов у вас будут все необходимые данные для формирования ресурса YandexClusterConfiguration, описывающего кластер в Yandex Cloud.
