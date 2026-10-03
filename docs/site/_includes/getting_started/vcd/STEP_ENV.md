{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Prepare the {{ page.platform_name[page.lang] }} environment so that Deckhouse Platform can manage cloud resources. The full procedure is described on the [environment preparation page](/modules/cloud-provider-vcd/environment.html) of the `cloud-provider-vcd` module.

{% alert level="warning" %}
The provider is confirmed to work with Ubuntu 22.04-based virtual machine templates only.
{% endalert %}

Perform the following preliminary steps:

1. Get a tenant with the resources listed in [List of required VCD resources](/modules/cloud-provider-vcd/environment.html#list-of-required-vcd-resources). The virtual data center must have an Edge Gateway. The internal network of the cluster is created automatically in the `WithNAT` layout used on this page.
1. Get a user with the [required permissions](/modules/cloud-provider-vcd/environment.html#user-permissions).
1. Prepare a [virtual machine template](/modules/cloud-provider-vcd/environment.html#virtual-machine-template).
