---
title: "Cloud provider — Openstack: Preparing environment"
---

Currently, Deckhouse connects to the OpenStack API using the user credentials for the OpenStack CLI.
The openrc file contains all the credentials (you can download it using this [guide](https://docs.openstack.org/zh_CN/user-guide/common/cli-set-environment-variables-using-openstack-rc.html)).
Note that if your provider has a custom web interface, the steps for downloading the openrc file may vary. Below, we provide instructions for MCS and Selectel.

### MCS — mail.ru cloud solutions

1. Follow this [link](https://mcs.mail.ru/app/project/keys/).
2. Click the "Download openrc version 3" button.

### Selectel

The provider supports adding separate projects and users within the same account. You need to create a project and a user for deploying a cluster and running Deckhouse:
* You can find the Project management section above the menu of the "Cloud Platform" on the left side;
* User management is available on the "All users" tab. Here, you can create a user and add it to the project.

1. Follow this [link](https://my.selectel.ru/vpc) and switch to the target project.
2. Select "Access" in the menu on the left.
3. In the window that opens, select the user and click the "Download" button".
4. Pass the user password along with the openrc file.

