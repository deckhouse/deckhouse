{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

You need to create a Yandex Cloud service account with the editor role to manage cloud resources. The detailed instructions for creating a service account with Yandex Cloud are available in the [documentation](/modules/cloud-provider-yandex/environment.html). Below, we will provide a brief overview of the necessary actions:

1. Create a user named `deckhouse`:

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

1. Assign the required roles to the newly created user for your cloud:

   ```shell
   yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
   ```

1. Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:

   ```shell
   yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
   ```
