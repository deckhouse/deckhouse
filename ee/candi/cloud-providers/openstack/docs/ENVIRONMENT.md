---
title: "Cloud provider — OpenStack: Preparing environment"
description: "Configuring OpenStack for Deckhouse cloud provider operation."
---

{% include notice_envinronment.liquid %}

To manage resources in an OpenStack cloud, Deckhouse connects to the OpenStack API.  
The list of OpenStack API services that need to be accessed for deployment is available in the [settings](./configuration.html#list-of-required-openstack-services) section.  
The user credentials required to connect to the OpenStack API are located in the openrc file (OpenStack RC file).

You can read the OpenStack documentation to get information about getting and using an openrc file with the standard OpenStack web interface.

The interface for getting an openrc file may differ if you use an OpenStack API of a cloud provider.
