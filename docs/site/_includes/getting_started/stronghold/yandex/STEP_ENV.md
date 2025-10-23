{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

You need to create a Yandex Cloud service account with the editor role to manage cloud resources. The detailed instructions for creating a service account with Yandex Cloud are available in the [documentation](/modules/cloud-provider-yandex/environment.html). Below, we will provide a brief overview of the necessary actions:

Create a user named `deckhouse`. The command response will contain its parameters:
{% snippetcut %}
```yaml
yc iam service-account create --name deckhouse
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```
{% endsnippetcut %}

Assign the `editor` role to the newly created user:
{% snippetcut %}
```yaml
yc resource-manager folder add-access-binding <folderID> --role editor --subject serviceAccount:<userID>
```
{% endsnippetcut %}

Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:
{% snippetcut %}
```yaml
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
{% endsnippetcut %}
