{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Prepare the {{ page.platform_name[page.lang] }} environment so that Deckhouse Platform can manage cloud resources. The full procedure is described on the [environment preparation page](/modules/cloud-provider-openstack/environment.html) of the `cloud-provider-openstack` module.

Create the service account and download openrc file. The data from the openrc file will be required further to fill in the `provider` section in the Deckhouse Platform configuration.
