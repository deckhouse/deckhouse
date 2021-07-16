Чтобы **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}** смог управлять ресурсами в Яндекс.Облако, необходимо создать сервисный аккаунт и выдать ему права на редактирование. Подробная инструкция по созданию сервисного аккаунта в Яндекс.Облако доступна в [документации провайдера](https://cloud.yandex.ru/docs/resource-manager/operations/cloud/set-access-bindings). Здесь мы представим краткую последовательность необходимых действий:

- Создайте пользователя с именем `candi`. В ответ вернутся параметры пользователя:
  ```yaml
yc iam service-account create --name candi
id: <userId>
folder_id: <folderId>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: candi
```
- Назначьте роль `editor` вновь созданному пользователю для своего облака:
  ```yaml
yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
```
- Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будем авторизовываться в облаке:
  ```yaml
yc iam key create --service-account-name candi --output candi-sa-key.json
```
