{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для управления ресурсами в Yandex Cloud, необходимо создать сервисный аккаунт с правами на редактирование. Подробная инструкция по созданию сервисного аккаунта в Yandex Cloud доступна в [документации](/modules/cloud-provider-yandex/environment.html). Ниже краткая версия:

1. Создайте пользователя с именем `deckhouse`:

   ```shell
   yc iam service-account create --name deckhouse
   ```

   В ответ вернутся параметры пользователя:

   ```console
   id: <userID>
   folder_id: <folderID>
   created_at: "YYYY-MM-DDTHH:MM:SSZ"
   name: deckhouse
   ```

1. Назначьте необходимые роли вновь созданному пользователю для своего облака:

   ```shell
   yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
   ```

1. Создайте JSON-файл с параметрами авторизации пользователя в облаке. В дальнейшем с помощью этих данных будет происходить авторизация в облаке:

   ```shell
   yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
   ```
