{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Prepare the {{ page.platform_name[page.lang] }} environment so that Deckhouse Platform can manage cloud resources. The full procedure is described on the [environment preparation page](/modules/cloud-provider-yandex/environment.html) of the `cloud-provider-yandex` module.

Create a service account for Deckhouse Platform and assign the `compute.editor`, `vpc.admin`, and `load-balancer.editor` roles to it:

1. Create a service account named `deckhouse`:

   ```shell
   yc iam service-account create --name deckhouse
   ```

   The command response will contain its parameters:

   ```console
   id: <userID>
   folder_id: <folderID>
   created_at: "YYYY-MM-DDTHH:MM:SSZ"
   name: deckhouse
   ```

1. Assign the roles to the service account in the folder:

   ```shell
   yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
   ```

1. Create a JSON file with the authorized key of the service account. Deckhouse Platform uses it to access the cloud:

   ```shell
   yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
   ```
