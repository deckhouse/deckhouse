---
title: Kubeconfig generator
permalink: en/user/web/kubeconfig.html
---

The kubeconfig generator web UI can be used to create configuration files
required for connecting to the Kubernetes cluster via the `kubectl` tool.

## Accessing the web UI

1. To open the kubeconfig generator web UI, click the corresponding link in the side menu on the Grafana overview page.

   ![Opening the kubeconfig web UI](../../images/kubeconfig/kubeconfig.png)

1. If you are accessing the web UI for the first time, enter user credentials.
1. Once the authentication is complete, you will see the main documentation page.
   This page contains configuration parameters for accessing the cluster.
1. Select a tab with your operating system (Linux, macOS, or Windows) in the mid-screen menu bar.
   Depending on the system, you will see the configuration commands,
   which will create the required context for connecting to the cluster once they are executed.
   Also, you can select the **Raw Config** option that allows you to manually add the configuration file
   to the `kubectl` configuration directory.

   ![Generating the configuration file](../../images/kubeconfig/kubeconfig-config.png)
