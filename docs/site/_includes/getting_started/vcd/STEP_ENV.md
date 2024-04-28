{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

{% alert level="warning" %}
The provider is confirmed to work with Ubuntu 22.04-based virtual machine templates only.
{% endalert %}

To start working with the provider, you have to create a tenant with the resources listed in the [documentation](/documentation/v1/modules/030-cloud-provider-vcd/environment.html#list-of-required-vcd-resources).

Once the tenant has been provisioned, you must configure the internal network, EDGE Gateway, and prepare the virtual machine template. Follow the instructions for setting up the environment in the provider's [documentation](/documentation/v1/modules/030-cloud-provider-vcd/environment.html).
