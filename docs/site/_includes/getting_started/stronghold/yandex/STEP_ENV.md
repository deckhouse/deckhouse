{%- include getting_started/stronghold/global/partials/NOTICES_ENVIRONMENT.liquid %}

You need to create a Yandex Cloud service account with the editor role to manage cloud resources. For detailed steps, see [preparing the Yandex Cloud environment](/modules/cloud-provider-yandex/environment.html). A brief overview of the steps follows.

Create a user named `deckhouse`:

```shell
yc iam service-account create --name deckhouse
```

The command output will contain its parameters:

```console
id: <USER_ID>
folder_id: <FOLDER_ID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

Assign the `editor` role to the newly created user:

```shell
yc resource-manager folder add-access-binding <FOLDER_ID> --role editor --subject serviceAccount:<USER_ID>
```

Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:

```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
