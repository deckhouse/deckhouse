---
title: "Cloud provider — Yandex Cloud: подготовка окружения"
description: "Настройка Yandex Cloud для работы облачного провайдера Deckhouse."
---

{% include notice_envinronment.liquid %}

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

Чтобы Deckhouse смог управлять ресурсами в облаке Yandex Cloud, необходимо создать сервисный аккаунт и выдать ему права на редактирование. Подробная инструкция по созданию сервисного аккаунта в Yandex Cloud доступна в [документации провайдера](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Далее представлена краткая последовательность необходимых действий:

1. Создайте пользователя с именем `deckhouse`. В ответ вернутся параметры пользователя:

   ```yaml
   yc iam service-account create --name deckhouse
   id: <userID>
   folder_id: <folderID>
   created_at: "YYYY-MM-DDTHH:MM:SSZ"
   name: deckhouse
   ```

1. Назначьте необходимые роли вновь созданному пользователю для своего облака:

   ```yaml
   yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
   ```

1. Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будет происходить авторизация в облаке:

   ```yaml
   yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
   ```

> Может потребоваться увеличение [квот](#квоты).
>
> При необходимости [зарезервируйте](faq.html#как-зарезервировать-публичный-ip-адрес) публичный IP-адрес.

## Квоты

При заказе нового кластера необходимо увеличить квоты в консоли Yandex Cloud.

Рекомендованные значения квот при создании нового кластера:

* Количество виртуальных процессоров: 64.
* Общий объем SSD-дисков: 2000 ГБ.
* Количество виртуальных машин: 25.
* Общий объем RAM виртуальных машин: 256 ГБ.
