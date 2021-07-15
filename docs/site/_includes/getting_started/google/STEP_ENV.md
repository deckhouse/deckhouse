You need to create a service account so that **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}** can manage resources in the Google Cloud. The detailed instructions for creating a service account are available in the [provider's documentation](https://cloud.google.com/iam/docs/service-accounts). Below is a brief sequence of required actions:

> List of roles required:
> - `roles/compute.admin`
> - `roles/iam.serviceAccountUser`
> - `roles/networkmanagement.admin`

- Export environment variables:
  ```shell
export PROJECT=sandbox
export SERVICE_ACCOUNT_NAME=deckhouse
```
- Select a project:
  ```shell
gcloud config set project $PROJECT
```
- Create a service account:
  ```shell
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
```
- Verify service account roles:
  ```shell
gcloud projects get-iam-policy ${PROJECT} --flatten="bindings[].members" --format='table(bindings.role)' \
    --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com"
```
- Create a service account key:
  ```shell
gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com \
    ~/service-account-key-${PROJECT}-${SERVICE_ACCOUNT_NAME}.json
```
