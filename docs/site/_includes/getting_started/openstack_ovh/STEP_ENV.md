{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

You need to create a service account so that Deckhouse Kubernetes Platform can manage resources in the {{ page.platform_name[page.lang] }}. The detailed instructions for creating a service account are available in the [documentation](/documentation/v1/modules/030-cloud-provider-openstack/environment.html).

Create the service account and download openrc file. The data from the openrc file will be required further to fill in the `provider` section in the Deckhouse Kubernetes Platform configuration.
