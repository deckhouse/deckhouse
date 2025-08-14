---
title: "Cloud provider — OpenStack: Preparing environment"
description: "Configuring OpenStack for Deckhouse cloud provider operation."
---

{% include notice_envinronment.liquid %}

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

To manage resources in an OpenStack cloud, Deckhouse connects to the OpenStack API.  
The list of OpenStack API services that need to be accessed for deployment is available in the [settings](./configuration.html#list-of-required-openstack-services) section.  
The user credentials required to connect to the OpenStack API are located in the openrc file (OpenStack RC file).

You can read the [OpenStack documentation](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file) to get information about getting and using an openrc file with the standard OpenStack web interface.

The interface for getting an openrc file may differ if you use an OpenStack API of a cloud provider.
