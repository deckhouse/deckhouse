Для управления ресурсами в Яндекс.Облаке, необходимо создать сервисный аккаунт с правами на редактирование. Подробная инструкция по созданию сервисного аккаунта в Яндекс.Облако доступна в [документации](/ru/documentation/v1/modules/030-cloud-provider-yandex/environment.html). Ниже краткая версия:

Создайте пользователя с именем `candi`. В ответ вернутся параметры пользователя:
{% snippetcut %}
```yaml
yc iam service-account create --name candi
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: candi
```
{% endsnippetcut %}

Назначьте роль `editor` вновь созданному пользователю для своего облака:
{% snippetcut %}
```yaml
yc resource-manager folder add-access-binding --id <folderID> --role editor --subject serviceAccount:<userID>
```
{% endsnippetcut %}

Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будем авторизовываться в облаке:
{% snippetcut %}
```yaml
yc iam key create --service-account-name candi --output candi-sa-key.json
```
{% endsnippetcut %}
