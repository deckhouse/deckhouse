{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

You need to create a service account so that Deckhouse Kubernetes Platform can manage resources in the {{ page.platform_name[page.lang] }}. The detailed instructions for creating a service account are available in the [documentation](/modules/cloud-provider-openstack/environment.html).

Create the service account and [download openrc file](https://docs.selectel.ru/en/cloud/servers/tools/openstack/#configure-authorization). The data from the openrc file will be required further to fill in the `provider` section in the Deckhouse Kubernetes Platform configuration.

Please note, that to create a node with the `CloudEphemeral` type in a zone other than zone A, you must first create a flavor with a disk of the required size. The [rootDiskSize](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-rootdisksize) parameter does not need to be specified.

{% offtopic title="Example of creating a flavor..." %}
```shell
openstack flavor create c4m8d50 --ram 8192 --disk 50 --vcpus 4 --private
```
{% endofftopic %}
