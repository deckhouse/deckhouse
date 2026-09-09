---
title: "Cloud provider — VMware vSphere: configuration"
force_searchable: true
---

The module is automatically enabled for all cloud clusters deployed in vSphere.

{% include module-alerts.liquid %}

{% include module-enable.liquid %}

{% include module-configure.liquid %}

{% include module-requirements.liquid %}

{% include module-conversion.liquid %}

The source of the settings depends on where the cluster control plane is hosted.
If the control plane runs on virtual machines or bare metal, the module uses its own settings described below.
If the control plane is hosted in a cloud, the module uses the [VsphereClusterConfiguration](cluster_configuration.html#vsphereclusterconfiguration) resource.

The number of nodes and their provisioning parameters are set in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource of the `node-manager` module.
The same resource specifies the instance class of the node group in the `cloudInstances.classReference` parameter.
For vSphere, the instance class is the [VsphereInstanceClass](cr.html#vsphereinstanceclass) custom resource that describes the parameters of the virtual machines.

Environment requirements, connecting to vCenter, networking, inbound traffic, and storage are covered in the [Infrastructure](environment.html#infrastructure) section.
Adding and removing cluster nodes is covered in the [`node-manager`](/modules/node-manager/faq.html) module documentation, and an example of a node group for vSphere is given in the [Creating a node group](examples.html#creating-a-node-group) section.

{% include module-settings.liquid %}
