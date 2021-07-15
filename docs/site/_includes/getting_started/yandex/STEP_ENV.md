You need to create a service account with the editor role with the cloud provider so that **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}**  can manage cloud resources. The detailed instructions for creating a service account with Yandex.Cloud are available in the provider's [documentation](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Below, we will provide a brief overview of the necessary actions:

- Create a user named `candi`. The command response will contain its parameters:
  ```yaml
yc iam service-account create --name candi
id: <userId>
folder_id: <folderId>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: candi
```
- Assign the `editor` role to the newly created user:
  ```yaml
yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
```
- Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:
  ```yaml
yc iam key create --service-account-name candi --output candi-sa-key.json
```
