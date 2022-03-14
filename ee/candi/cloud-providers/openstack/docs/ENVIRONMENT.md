---
title: "Cloud provider — Openstack: Preparing environment"
---

Currently, Deckhouse connects to the OpenStack API using the user credentials for the OpenStack CLI.
The openrc file contains all the credentials.

## How to download OpenStack RC file

### Standard OpenStack web interface

You can download it using this [guide](https://docs.openstack.org/zh_CN/user-guide/common/cli-set-environment-variables-using-openstack-rc.html) in the section "Download and source the OpenStack RC file".

### [Mail.ru Cloud Solutions](https://mcs.mail.ru/) (MCS)

1. Follow this [link](https://mcs.mail.ru/app/project/keys/).
1. Go to the "API Keys" tab.
1. Click the "Download openrc version 3" button.
1. Execute the resulting shell script, during which the values of the environment variables will be created (they will be used in the provider parameters in the Deckhouse configuration).

### [Selectel](https://selectel.ru/)

The provider supports adding separate projects and users within the same account. You need to create a project and a user for deploying a cluster and running Deckhouse:
* You can find the Project management section above the menu of the "Cloud Platform" on the left side;
* User management is available on the "All users" tab. Here, you can create a user and add it to the project:
  1. Follow this [link](https://my.selectel.ru/vpc) and switch to the target project.
  2. Select "Access" in the menu on the left.
  3. In the window that opens, select the user and click the "Download" button".
  4. Pass the user password along with the openrc file.
