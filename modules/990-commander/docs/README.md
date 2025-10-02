---
title: "Deckhouse Commander"
description: "Centralized management of clusters"
menuTitle: "Deckhouse Commander"
---

Deckhouse Commander is a web application that allows you to create similar clusters based on the Deckhouse Kubernetes Platform, manage their configuration and lifecycle.

### Main features

- Cluster lifecycle management based on Deckhouse Kubernetes Platform: creation, deletion, modification.
- Unification and updating of clusters through configuration templates.
- Tracking changes, bringing clusters to the configuration specified in the Deckhouse Commander.
- Managing operational needs using the built-in administration interface.
- Management of lists of computing resources used in clusters.
- API for external integration.
- Attaching existing Deckhouse Kubernetes Platform clusters.
- Access control

Coming soon:

- Project management.
- Overview of cloud resources created with the cluster.
- Multi-tenancy and self-service.
- Federated cluster capabilities.
- Integration with other products of the Deckhouse ecosystem: Observability Platform, Stronghold, Registry, etc.

### Creating, updating and deleting clusters

Creating, updating and deleting clusters based on Deckhouse Kubernetes Platform is available both on cloud platforms and on static resources. Therefore, it is necessary to provide access to the API of the cloud platform or to pre-created (static) computing resources (physical or virtual machines, network access, etc.) for the cluster, which will be used to create the cluster.

The Deckhouse Commander makes it easy to create, update and delete clusters. When creating a cluster in the cloud API, the Deckhouse Commander automatically creates the necessary cloud resources, and on static resources it performs their necessary configuration. When deleting a cluster, automatically created cloud resources are deleted, and manually created static resources are cleared of Deckhouse Kubernetes Platform components.

### Unification and updating of clusters using configuration templates

Reproduction of uniform Deckhouse Kubernetes Platform clusters is achieved through the use of cluster configuration templates. The user describes the cluster configuration, chooses which parameters to parameterize it with, and as a result gets a ready-to-use configuration template.

Clusters are created based on one template, and the individual features of the cluster configuration are determined by the input parameters of the template. Thus, based on the template, you can create many uniform clusters with the freedom of configuration provided by the author of the template.

An existing template can be updated, and then the already created clusters can be migrated to the new version of the template. A new cluster configuration will be generated based on the updated template. Deckhouse Commander will bring the existing Deckhouse Kubernetes Platform cluster in line with this configuration. This achieves the actualization of the existing fleet of clusters.

### Tracking changes, bringing clusters to the configuration set in Deckhouse Commander

Deckhouse Commander is the source of truth for the configuration of the clusters it manages. Deckhouse Commander tracks the current configuration of Deckhouse Kubernetes Platform clusters: both the infrastructure configuration on which the cluster was created and the Kubernetes configuration. When Deckhouse Commander detects a discrepancy between the configured configuration and the actual configuration, Deckhouse Commander brings the actual configuration to the specified one.

Thus, if the configuration changes on the Deckhouse Commander side, Deckhouse Commander updates the cluster and brings it to a new current state. If the configuration changes on the cluster side, the Deckhouse Commander rolls back the changes and brings the cluster to its target state.

### Managing operational needs using the built-in administration interface

In the Deckhouse Commander web interface, you can open the cluster administration page. It happens that part of the cluster configuration is more convenient to adjust manually privately than to set in the cluster template and track through Deckhouse Commander. To configure the cluster privately without leaving Deckhouse Commander, use the cluster administration page.

### Management of lists of computing resources used in clusters

To create a cluster, you may need to specify pre-allocated computing resources. Such resources can be virtual machines, networks, system accounts in the virtualization API, load balancers, etc. At the same time, these computing resources are specific instances of resources that need to be used in the Deckhouse Kubernetes Platform cluster and not used in other clusters during the existence of the cluster.

It may be convenient to keep track of such resources and have information about which resources are free and which are occupied by clusters.

To maintain a list of resources in Deckhouse Commander, there is a «Resources» section. It contains resources grouped into directories. Resource directories define a data schema, which is determined by the user. The resources in each directory are data corresponding to the data scheme of the directory.

In the cluster template, the input parameter can be set as selecting one or more resources from a specific directory. Thus, during creation or modification of the Deckhouse Kubernetes Platform cluster, resources can be used as data inside the cluster configuration. And within the directory, the resources occupied by the cluster are marked separately, and such resources cannot be used in another cluster. When a cluster is deleted, the resources used in it are released for subsequent use in other clusters. Also, unused resources can be removed from the directory.

### API for external integration

Deckhouse Commander provides a software interface (API) for managing clusters and resources. To use the Deckhouse Commander API, you need to issue an access token in the web interface in the «Settings» menu.

Access to the API allows you to monitor the status of clusters and resources, as well as create, delete and modify clusters. The API opens up the possibility for integration with third-party infrastructure management tools or for tracking it.

### Attaching and detaching clusters

You can attach a cluster based on the Deckhouse Kubernetes Platform, which was created manually or in another Deckhouse Commander, to Deckhouse Commander. You can also take a cluster out of Deckhouse Commander's control; there is an option to detach the cluster for this.
