---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/yandex/yandex-authorization.html
lang: ru
---

Для того чтобы Deckhouse мог управлять ресурсами в Yandex Cloud, необходимо:

- подготовить виртуальные машины;
- создать сервисный аккаунт;
- назначить ему необходимые IAM-роли;
- сгенерировать авторизационный ключ;
- при необходимости — зарезервировать публичный IP-адрес;
- обеспечить достаточные квоты на ресурсы в облаке.

Ниже приведены пошаговые действия по настройке подключения.

## Подготовка окружения

На всех виртуальных машинах, создаваемых для кластера, должен быть установлен и активен пакет `cloud-init`. После запуска ВМ убедитесь, что запущены следующие сервисы:

```console
systemctl status cloud-config.service
systemctl status cloud-final.service
systemctl status cloud-init.service
```

## Создание сервисного аккаунта

Для создания сервисного аккаунта выполните команду:

```console
yc iam service-account create --name deckhouse
```

Команда вернёт информацию о созданном сервисном аккаунте:

```console
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

> **Важно**. Сохраните `userID` и `folderID` — они понадобятся в следующих шагах.

## Назначение IAM-ролей

Для работы Deckhouse с ресурсами облака сервисному аккаунту необходимо назначить следующие роли:

```console
yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role api-gateway.editor --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role connection-manager.editor --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
```

## Генерация авторизованного ключа

Создайте JSON-файл с авторизацией для использования в конфигурации:

```console
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```

Содержимое файла `deckhouse-sa-key.json` будет использоваться в поле `provider.serviceAccountJSON` при описании конфигурации кластера.

## Проверка и увеличение квот

Для развёртывания и масштабирования кластера убедитесь, что в облачном аккаунте установлены необходимые квоты. Рекомендуемые значения:

- Виртуальные процессоры: 64
- Объём SSD-дисков: 2000 ГБ
- Количество ВМ: 25
- Объём RAM: 256 ГБ

Увеличение квот можно запросить через консоль Yandex Cloud.

## (При необходимости) Резервирование публичного IP

Если используется схема размещения WithoutNAT или WithNATInstance, и требуется фиксированный внешний IP-адрес, выполните команду:

```console
yc vpc address create --external-ipv4 zone=ru-central1-a
```

Пример вывода команды:

```yaml
id: e9b4cfmmnc1mhgij75n7
folder_id: b1gog0h9k05lhqe5d88l
external_ipv4_address:
  address: 178.154.226.159
  zone_id: ru-central1-a
reserved: true
```

После выполнения этих шагов у вас будут все необходимые данные для формирования ресурса YandexClusterConfiguration, описывающего кластер в Yandex Cloud.