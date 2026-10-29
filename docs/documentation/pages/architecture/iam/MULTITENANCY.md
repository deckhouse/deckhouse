---
title: Multitenancy
permalink: en/architecture/iam/multitenancy.html
lang: en
search: multitenancy, ProjectTemplate, Project, project isolation
description: How multitenancy works in Deckhouse Platform.
---

The [`multitenancy-manager`](/modules/multitenancy-manager/) module allows you to create isolated [projects](../../glossary.html#project) within the Deckhouse Platform (DP). Projects provide resource quotas, network isolation, and security features that go beyond standard namespaces.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/multitenancy-manager/).

## Internal logic

### Project creation

The following custom resources are used to create a project:

* [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate): Describes the project template.
  It defines the list of resources to be created in the project
  and the parameter schema that can be passed during project creation.
* [Project](/modules/multitenancy-manager/cr.html#project): Describes a specific project.

When a Project is created from a specified ProjectTemplate, the following occurs:

1. The provided [parameters](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-parameters) are validated against the OpenAPI specification
   (defined in the [`parametersSchema.openAPIV3Schema`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) parameter of the [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) resource).
1. A namespace is created with the name of the [Project](/modules/multitenancy-manager/cr.html#project).
1. The project objects are created from the template. In a `deckhouse.io/v1alpha2` template, settings such as the pod security profile, network isolation, log collection and node placement are described by structured fields, and the controller creates the corresponding objects in every namespace of the project. Other objects are described in `manifests` and are applied with the rights of the `d8:project:admin` role in the project namespaces, not with the rights of the controller.
   A template with a Helm text in [`resourcesTemplate`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-resourcestemplate) is rendered through Helm. This form is deprecated, no template can get a new `resourcesTemplate`, and a template is created through `deckhouse.io/v1alpha2` only.
1. The standard fields of the project are applied independently of the template: [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) is reconciled into a ResourceQuota, and [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) into an auto-managed [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding).

{% alert level="warning" %}
If you modify a project template, all previously created projects will be updated according to the new template.
{% endalert %}

### Project isolation

A namespace alone does not isolate a project. The project adds the following:

* Access to the whole project through [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators), ProjectRoleBinding and ClusterProjectRoleBinding. A plain RoleBinding in one namespace is allowed for delegatable roles.
* A resource limit through [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota), applied as the `d8-project-quota` ResourceQuota in the main namespace.
* Network isolation through the [`networkPolicy.mode: Isolated`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-networkpolicy-mode) field of the template.
* The pod security profile through the [`podSecurityStandard`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-podsecuritystandard) field of the template.

### Managing access to cluster-wide resources

Objects in project namespaces can reference cluster-wide resources. For example, a PersistentVolumeClaim can use a StorageClass, a Certificate can use a ClusterIssuer, and a RoleBinding can use a ClusterRole. The module allows cluster administrators to define which cluster-wide resources can be used from project namespaces and which values are used by default.

This mechanism works independently of RBAC. RBAC determines *who can create and modify* objects, while the cluster-wide resource access mechanism determines *which resources* those objects can use.

The mechanism uses four custom resources:

* [GrantableClusterResourceDefinition](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition) registers a type of cluster-wide resource whose access can be managed. These resources are provided by DP or module developers.
* [GrantableClusterResourceReference](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference) defines where a registered cluster-wide resource is used, for example, which resource field contains a reference to it. These resources are provided by modules.
* [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) defines access rules. In the `v1alpha2` version, a cluster administrator selects the projects to which the policy applies by the labels of Project objects (`projectSelector`) and narrows the choice by the labels of project namespaces (`namespaceSelector`). The policy also defines the allowed and denied resources, as well as the resource used by default.
* In every namespace of every project, the controller keeps an [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) for each GrantableClusterResourceDefinition. It lists the cluster-wide resources available to the project under the policies that apply and is read-only.

Until the administrator creates a ClusterResourceGrantPolicy, resource availability is determined by their registration: resources are available to all projects if `defaultAvailability: All` (the default value) is set in GrantableClusterResourceDefinition and the resource does not match the `excluded` filters.

Access checks are performed only for objects in project namespaces. If an access policy changes, cluster-wide resources already used by existing objects remain available to those objects.

The webhooks reject requests while the module controller is unavailable. Until it is back, objects of the registered kinds (for example, PersistentVolumeClaim, Service or RoleBinding) cannot be created or changed in project namespaces, except by the platform components and members of the `system:masters` group. To keep the controller available, enable [`highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability).

For a detailed description of the access management mechanism, refer to the [`multitenancy-manager`](/modules/multitenancy-manager/#managing-access-to-cluster-wide-resources) module documentation.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The architecture of the [`multitenancy-manager`](/modules/multitenancy-manager/) module at Level 2 of the C4 model and its interactions with other DP components are shown in the following diagram.

![Multitenancy-manager module architecture](../../images/architecture/iam/c4-l2-multitenancy-manager.svg)

## Module components

The module consists of the following components:

- **Multitenancy-manager**: The component consists of a single **multitenancy-manager** container and provides the following functions:

  - Managing the [Project](../../glossary.html#project) and ProjectTemplate custom resources.
  - Validating the Project and ProjectTemplate custom resources.
  - Creating the resources specified in the ProjectTemplate custom resource based on the parameters set in Project.
  - Managing the grants mechanism for cluster-wide resources — the
    [`GrantableClusterResourceDefinition`](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition),
    [`GrantableClusterResourceReference`](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference),
    [`ClusterResourceGrantPolicy`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy), and
    [`AvailableClusterResource`](/modules/multitenancy-manager/cr.html#availableclusterresource) custom resources. It
    renders the per-project AvailableClusterResource catalogs and runs the validating and mutating admission webhooks
    (`/is-granted`, `/defaults`, `/protect`) that enforce which cluster resources a project may reference and
    substitute per-project defaults on CREATE.

   > **Warning.** Multitenancy-manager has `cluster-admin` permissions. Objects described in the `manifests` field of ProjectTemplate are applied not with these permissions but with the rights of the `d8:project:admin` role in the project namespaces.

## Module interactions

The module interacts with the following components:

- **Kube-apiserver**:
  - Managing the Project and ProjectTemplate custom resources.
  - Validating the Project and ProjectTemplate custom resources.
  - Creating the resources specified in the ProjectTemplate custom resource based on the parameters set in Project.
  - Managing the GrantableClusterResourceDefinition, GrantableClusterResourceReference, ClusterResourceGrantPolicy, and AvailableClusterResource custom resources.
  - Validating the AvailableClusterResource custom resources.
  - Creating validation admission webhooks based on conditions in GrantableClusterResourceDefinition and GrantableClusterResourceReference custom resources.
