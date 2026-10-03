{%- include getting_started/stronghold/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для управления ресурсами в Yandex Cloud необходимо создать сервисный аккаунт с правами на редактирование. Подробная инструкция — [в разделе о подготовке окружения Yandex Cloud](/modules/cloud-provider-yandex/environment.html). Ниже приведена краткая версия.

Создайте пользователя с именем `deckhouse`:

```shell
yc iam service-account create --name deckhouse
```

В ответ вернутся параметры пользователя:

```console
id: <USER_ID>
folder_id: <FOLDER_ID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

Назначьте роль `editor` вновь созданному пользователю для своего облака:

```shell
yc resource-manager folder add-access-binding <FOLDER_ID> --role editor --subject serviceAccount:<USER_ID>
```

Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будет происходить авторизация в облаке:

```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
