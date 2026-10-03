{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами в облаке. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-yandex/environment.html) модуля `cloud-provider-yandex`.

Создайте сервисный аккаунт для Deckhouse Platform и назначьте ему роли `compute.editor`, `vpc.admin` и `load-balancer.editor`:

1. Создайте сервисный аккаунт с именем `deckhouse`:

   ```shell
   yc iam service-account create --name deckhouse
   ```

   В ответ вернутся параметры сервисного аккаунта:

   ```console
   id: <userID>
   folder_id: <folderID>
   created_at: "YYYY-MM-DDTHH:MM:SSZ"
   name: deckhouse
   ```

1. Назначьте роли сервисному аккаунту в каталоге:

   ```shell
   yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
   ```

1. Создайте JSON-файл с авторизованным ключом сервисного аккаунта. Deckhouse Platform использует его для доступа к облаку:

   ```shell
   yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
   ```
