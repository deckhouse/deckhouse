To manage resources in {{ site.data.admin.cloud-types.types[page.cloud_type].name }} using Deckhouse Kubernetes Platform,
you must connect to the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API.

The list of {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API services required for deployment
is available in the [Configuration](./configuration-and-layout-scheme.html#list-of-required-services) section.

The user credentials required to access the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API
are provided in the OpenStack RC file (openrc file).

Instructions for downloading the openrc file via the standard web interface of {{ site.data.admin.cloud-types.types[page.cloud_type].name }} and using it are available in the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} [documentation](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file).

If you’re using a cloud provider’s implementation of the {{ site.data.admin.cloud-types.types[page.cloud_type].name }} API,
the process for obtaining the openrc file may differ.
