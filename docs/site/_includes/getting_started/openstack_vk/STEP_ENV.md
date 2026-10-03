{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Prepare the {{ page.platform_name[page.lang] }} environment so that Deckhouse Platform can manage cloud resources. The full procedure is described on the [environment preparation page](/modules/cloud-provider-openstack/environment.html) of the `cloud-provider-openstack` module.

To get the authorization data, run the following steps on the **personal computer**:

1. Open the [project keys page](https://mcs.mail.ru/app/project/keys/) in VK Cloud.
1. Switch to the "API keys" tab.
1. Click "Download openrc version 3".
1. Run the downloaded shell script. It sets environment variables whose values are used in the `provider` parameters of the Deckhouse Platform configuration.
