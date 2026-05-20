{%- include getting_started/stronghold/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для управления ресурсами в Yandex Cloud, необходимо создать сервисный аккаунт с правами на редактирование. Подробная инструкция по созданию сервисного аккаунта в Yandex Cloud доступна в [документации](/modules/cloud-provider-yandex/environment.html). Ниже краткая версия:

Создайте пользователя с именем `deckhouse`:

{% snippetcut %}
```shell
yc iam service-account create --name deckhouse
```
{% endsnippetcut %}

В ответ вернутся параметры пользователя:
{% snippetcut %}
```console
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```
{% endsnippetcut %}

Назначьте роль `editor` вновь созданному пользователю для своего облака:

{% snippetcut %}
```shell
yc resource-manager folder add-access-binding <folderID> --role editor --subject serviceAccount:<userID>
```
{% endsnippetcut %}

Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будет происходить авторизация в облаке:

{% snippetcut %}
```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
{% endsnippetcut %}
