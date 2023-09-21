---
title: "Cloud provider — OpenStack: Preparing environment"
description: "Configuring OpenStack for Deckhouse cloud provider operation."
---

To manage resources in an OpenStack cloud, Deckhouse connects to the OpenStack API. The user credentials required to connect to the OpenStack API are located in the openrc file (OpenStack RC file).

You can read the [OpenStack documentation](https://docs.openstack.org/ocata/admin-guide/common/cli-set-environment-variables-using-openstack-rc.html#download-and-source-the-openstack-rc-file) to get information about getting and using an openrc file with the standard OpenStack web interface.

The interface for getting an openrc file may differ if you use an OpenStack API of a cloud provider.
