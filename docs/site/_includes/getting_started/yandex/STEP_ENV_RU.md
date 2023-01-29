{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для управления ресурсами в Yandex Cloud, необходимо создать сервисный аккаунт с правами на редактирование. Подробная инструкция по созданию сервисного аккаунта в Yandex Cloud доступна в [документации](/documentation/v1/modules/030-cloud-provider-yandex/environment.html). Ниже краткая версия:

Создайте пользователя с именем `deckhouse`. В ответ вернутся параметры пользователя:
{% snippetcut %}
```yaml
yc iam service-account create --name deckhouse
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```
{% endsnippetcut %}

Назначьте роль `editor` вновь созданному пользователю для своего облака:
{% snippetcut %}
```yaml
yc resource-manager folder add-access-binding <folderID> --role editor --subject serviceAccount:<userID>
```
{% endsnippetcut %}

Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будем авторизовываться в облаке:
{% snippetcut %}
```yaml
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
{% endsnippetcut %}
