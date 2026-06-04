{%- include getting_started/stronghold/global/partials/NOTICES_ENVIRONMENT.liquid %}

You need to create a Yandex Cloud service account with the editor role to manage cloud resources. The detailed instructions for creating a service account with Yandex Cloud are available in the [documentation](/modules/cloud-provider-yandex/environment.html). Below, we will provide a brief overview of the necessary actions:

Create a user named `deckhouse`:

```shell
yc iam service-account create --name deckhouse
```

The command output will contain its parameters:

```console
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

Assign the `editor` role to the newly created user:

```shell
yc resource-manager folder add-access-binding <folderID> --role editor --subject serviceAccount:<userID>
```

Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:

```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
