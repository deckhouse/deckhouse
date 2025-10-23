---
title: "Configuring access to the project"
permalink: en/virtualization-platform/documentation/user/project-access.html
---

To connect to the project, follow these steps:

1. Request a link to download the configuration file (`kubeconfig.<domain>`) from the Platform Administrator.
1. Enter your email address and password to access the project.
1. Copy the configuration file to your home directory at `~/.kube/config`.
1. Install the [d8 utility](/products/kubernetes-platform/documentation/v1/cli/d8/).
1. To manage the project, use the command: `d8 k -n <project_name>` or `d8 v -n <project_name>`.
