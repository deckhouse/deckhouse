{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Prepare the {{ page.platform_name[page.lang] }} environment so that Deckhouse Platform can manage cloud resources. The full procedure is described on the [environment preparation page](/modules/cloud-provider-openstack/environment.html) of the `cloud-provider-openstack` module.

[Create a service account](https://docs.selectel.ru/en/cloud-servers/tools/openstack-cli/configure-openstack-cli/#add-service-user-for-os) and [download the openrc file](https://docs.selectel.ru/en/cloud-servers/tools/openstack-cli/configure-openstack-cli/#download-rc-file-for-os). The data from the openrc file will be required further to fill in the `provider` section in the Deckhouse Platform configuration.

To create a node with the `CloudEphemeral` type in a Selectel zone other than zone A, first create a flavor with a disk of the required size. In this case, do not set the [rootDiskSize](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) parameter.

{% offtopic title="Example of creating a flavor..." %}
```shell
openstack flavor create c4m8d50 --ram 8192 --disk 50 --vcpus 4 --private
```
{% endofftopic %}
