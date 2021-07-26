---
title: "Cloud provider — Yandex.Cloud: подготовка окружения"
---

Чтобы Deckhouse смог управлять ресурсами в облаке Yandex.Cloud, необходимо создать сервисный аккаунт и выдать ему права на редактирование. Подробная инструкция по созданию сервисного аккаунта в Yandex.Cloud доступна в [документации провайдера](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Далее представлена краткая последовательность необходимых действий:

> Возможно вам потребуется увеличение [квот](#квоты).

> [Зарезервируйте](faq.html#как-зарезервировать-публичный-ip-адрес) публичный IP-адрес, при необходимости.

- Создайте пользователя с именем `deckhouse`. В ответ вернутся параметры пользователя:
  ```yaml
yc iam service-account create --name deckhouse
id: <userId>
folder_id: <folderId>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```
- Назначьте роль `editor` вновь созданному пользователю для своего облака:
  ```yaml
  yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
  ```
- Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будем авторизовываться в облаке:
  ```yaml
  yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
  ```

## Квоты

При заказе нового кластера нужно увеличить квоты в консоли Yandex.Cloud. Рекомендованные параметры:
* Количество виртуальных процессоров - 64.
* Общий объём SSD-дисков - 2000 Гб.
* Количество виртуальных машин - 25.
* Общий объём RAM виртуальных машин - 256 Гб.

