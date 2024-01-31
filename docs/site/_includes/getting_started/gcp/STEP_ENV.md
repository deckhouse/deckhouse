{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

You need to create a service account so that Deckhouse Kubernetes Platform can manage resources in the {{ page.platform_name[page.lang] }}. The detailed instructions for creating a service account are available in the [documentation](/documentation/v1/modules/030-cloud-provider-gcp/environment.html). Below is a brief sequence of required actions (run them on the **personal computer**):

{% alert %}
List of roles required:
- `roles/compute.admin`
- `roles/iam.serviceAccountUser`
- `roles/networkmanagement.admin`
{% endalert %}

Export environment variables:
{% snippetcut %}
```shell
export PROJECT_ID=sandbox
export SERVICE_ACCOUNT_NAME=deckhouse
```
{% endsnippetcut %}

Select a project:
{% snippetcut %}
```shell
gcloud config set project $PROJECT_ID
```
{% endsnippetcut %}

Create a service account:
{% snippetcut %}
```shell
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
```
{% endsnippetcut %}

Connect roles to the service account:
{% snippetcut %}
```shell
for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin; do \
  gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role=${role}; done
```
{% endsnippetcut %}

Verify service account roles:
{% snippetcut %}
```shell
gcloud projects get-iam-policy ${PROJECT_ID} --flatten="bindings[].members" --format='table(bindings.role)' \
    --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
```
{% endsnippetcut %}

Create a service account key:
{% snippetcut %}
```shell
gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
    ~/service-account-key-${PROJECT_ID}-${SERVICE_ACCOUNT_NAME}.json
```
{% endsnippetcut %}
