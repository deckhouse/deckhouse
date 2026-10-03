{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Prepare the {{ page.platform_name[page.lang] }} environment so that Deckhouse Platform can manage the resources of the virtualization platform. The full procedure is described on the [environment preparation page](/modules/cloud-provider-zvirt/environment.html) of the `cloud-provider-zvirt` module.

Deckhouse Platform requires zVirt version 4.0–4.4.

Perform the following preliminary steps in zVirt:

1. [Prepare an operating system image](/modules/cloud-provider-zvirt/environment.html#prepare-an-operating-system-image).
1. [Prepare a virtual machine template](/modules/cloud-provider-zvirt/environment.html#prepare-a-virtual-machine-template).
