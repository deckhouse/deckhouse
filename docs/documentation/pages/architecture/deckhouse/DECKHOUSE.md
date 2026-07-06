---
title: Deckhouse module
permalink: en/architecture/deckhouse/deckhouse.html
search: deckhouse, deckhouse-controller, modules
description: Architecture of the deckhouse module in Deckhouse Kubernetes Platform.
---

The [`deckhouse`](/modules/deckhouse/) module implements the core of Deckhouse Kubernetes Platform (DKP), performing the following operations:
- Platform updates.
- Module configuration management.
- Module installation and updates.
- Module documentation build triggering.
- Validation of custom resources managed by DKP modules.

The module manages the following custom resources in the `deckhouse.io` API group:

- Module management:
  - [Module](../../reference/api/cr.html#module): Description, status, and publication of module information.
  - [ModuleConfig](../../reference/api/cr.html#moduleconfig): Description of user-defined module settings.
  - [ModulePullOverride](../../reference/api/cr.html#modulepulloverride): Description of module version selection overrides.
  - [ModuleRelease](../../reference/api/cr.html#modulerelease): Description, publication, and tracking of module releases.
  - [ModuleSettingsDefinition](../../reference/api/cr.html#modulesettingsdefinition): Schema, versions, and transformation rules for module settings.
  - [ModuleSource](../../reference/api/cr.html#modulesource): Description of a module source, repository, or storage.
  - [ModuleUpdatePolicy](../../reference/api/cr.html#moduleupdatepolicy): Rules for module updates and version transition automation.

- Platform management:
  - [DeckhouseRelease](../../reference/api/cr.html#deckhouserelease): An object that defines the Deckhouse release (version) and platform update policy.

- Package management ([Marketplace](../marketplace)):
  - [Application](../../reference/api/cr.html#application): Description and desired state of an application package (a group of components or an application).
  - [ApplicationPackage](../../reference/api/cr.html#applicationpackage): Package metadata, sources, and settings.
  - [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion): Description of a specific package version and its parameters.
  - [PackageRepository](../../reference/api/cr.html#packagerepository): An object that describes a package repository source and its parameters.
  - [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation): Operations on package repositories, such as synchronization or updates.

- Utility management:
  - [CNIMigration](../../reference/api/cr.html#cnimigration): [Container Network Interface (CNI)](https://github.com/containernetworking/cni) migration process, including migration parameters and status.
  - [CNINodeMigration](../../reference/api/cr.html#cninodemigration): Status and management of CNI migration at the individual node level.
  - ObjectKeeper: A resource that links Kubernetes resources using `ownerReference`.
  - [ModuleDocumentation](../../reference/api/cr.html#moduledocumentation): Description of parameters for generating and storing module documentation.

- Management of custom resources controlled by DKP modules:
  - [ConversionWebhook](/modules/deckhouse/latest/cr.html#conversionwebhook): Settings and handlers for resource conversion webhooks.
  - [ValidationWebhook](/modules/deckhouse/latest/cr.html#validationwebhook): Settings and handlers for resource validation webhooks.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`deckhouse`](/modules/deckhouse/) module and its interaction with other DKP components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Deckhouse module architecture](../../images/architecture/deckhouse/c4-l2-deckhouse-deckhouse.svg)

## Module components

The module consists of the following components:

1. **Deckhouse** (Deployment): A controller that implements platform management operations.

   The controller orchestrates platform management tasks using [the queueing mechanism](./queues.html).

   The Deckhouse controller can run in standard mode or in [hook](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md) isolation mode. To enable it, create the `chroot-mode` ConfigMap in the `d8-system` namespace. In isolation mode, shell hooks and module enable scripts run in a chroot environment with a limited set of mounted directories, isolating them from the controller container file system.

   If [High Availability (HA)](../../admin/configuration/high-reliability-and-availability/) mode is enabled, multiple Deckhouse controller instances are started. To ensure correct behavior, Deckhouse controllers perform leader election using the `deckhouse-leader-election` Lease resource. The controller elected as leader performs all platform management operations.

   In addition, the Deckhouse controller configures:

   | Description       | Module configuration parameter                |
   |-------------- |-------------------------------------- |
   | Logging level          | [`.spec.settings.logLevel`](/modules/deckhouse/configuration.html#parameters-loglevel)   |
   | Default enabled module set | [`.spec.settings.bundle`](/modules/deckhouse/configuration.html#parameters-bundle)   |
   | Release channel | [`.spec.settings.releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel)   |
   | Update mode | [`.spec.settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode)   |
   | Update windows | [`.spec.settings.update.windows.days`](/modules/deckhouse/configuration.html#parameters-update-windows)   |

   For details about module settings, refer to the [module documentation section](/modules/deckhouse/).

   It consists of the following containers:

   * **init-downloaded-modules**: Init container that prepares the directory structure required for module operations.
   * **deckhouse**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to the main container component debug interface.
  
1. **Webhook-handler** (Deployment): Consists of a single **handler** container and implements a generic webhook for conversion and validation of custom resources managed by DKP.

    The component watches [ConversionWebhook](/modules/deckhouse/latest/cr.html#conversionwebhook) and [ValidationWebhook](/modules/deckhouse/latest/cr.html#validationwebhook) custom resources and, based on them, generates hook Python files for [shell-operator](https://github.com/flant/shell-operator) from templates. When `kube-apiserver` sends resource validation or conversion requests, shell-operator runs the required hook and returns the processing result.

1. **Cni-migration-manager** (Deployment): An optional component running on control plane nodes, consisting of a single **manager** container. The component manages the network plugin (CNI) switching process in the DKP cluster and records the current state in the CNIMigration custom resource. Migration to Flannel, Simple bridge, and Cilium is supported. For details on switching CNI in the cluster, refer to the [corresponding guide](/products/kubernetes-platform/guides/cni-migration.html).

    {% alert level="info" %}
    The component is created by the `detect-cni-migration` global hook when the CNIMigration custom resource exists. The CNIMigration resource is created manually by an administrator or by running the `d8 network cni-migration switch --to-cni <target cni>` command.
    {% endalert %}

1. **Cni-migration-agent** (DaemonSet): An optional component running on all cluster nodes, consisting of a single **agent** container. The component watches the CNIMigration custom resource and manages the CNINodeMigration custom resource that reflects migration state for a specific node.

    {% alert level="info" %}
    The component is created by the `detect-cni-migration` global hook when the CNIMigration custom resource exists. The CNIMigration resource is created manually by an administrator or by running the `d8 network cni-migration switch --to-cni <target cni>` command.
    {% endalert %}

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:
   - Working with custom resources in the `deckhouse.io` API group.
   - Watching Pod and DaemonSet resources during network plugin switching.
   - Watching resources described in the ObjectKeeper custom resource.
   - Creating and updating Lease resources.
   - Creating, deleting, modifying, and watching resources described in DKP modules.
   - Authorizing requests.

1. [**Documentation**](/modules/documentation/): Updating documentation when a DKP module is added or updated.

1. **Image registry**: Retrieving module component images along with metadata when the [`registry`](/modules/registry/) module is installed in Unmanaged mode.

1. **`registry` module**: Retrieving module component images along with metadata when the [`registry`](/modules/registry/) module is installed in one of the Direct, Proxy, or Local modes.

The module is interacted with by the following external components:

* **Kube-apiserver**: Validation and conversion of DKP custom resources.
* **Prometheus-main**: Collecting metrics from the `deckhouse` and `webhook-handler` containers.
