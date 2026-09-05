---
title: "Module multitenancy-manager"
search: multitenancy
description: Multitenancy and Projects in Kubernetes. The multitenancy-manager module in Deckhouse allows creating projects for various development teams with the ability to subsequently deploy applications in them.
---
## Description

The module enables the creation of projects in a Kubernetes cluster. **Project** is an isolated environment where applications can be deployed.

## Why is this needed?

The standard `Namespace` resource, used for logical resource separation in Kubernetes, does not provide necessary functionalities, hence it is not an isolated environment:
* [Resource consumption by pods](https://kubernetes.io/docs/concepts/policy/resource-quotas/) is not limited by default;
* [Network communication](https://kubernetes.io/docs/concepts/services-networking/network-policies/) with other pods works by default from any point in the cluster;
* Unrestricted access to node resources: address space, network space, mounted host directories.

The configuration capabilities of `Namespace` do not fully meet modern development requirements. By default, the following features are not included for `Namespace`:
* Log collection;
* Audit;
* Vulnerability scanning.

The functionality of projects allows addressing these issues.

{% alert level="warning" %}
The [`secret-copier`](../secret-copier/) module cannot be used together with `multitenancy-manager` module.
{% endalert %}

## Advantages of the module

For platform administrators:
* **Consistency**: Administrators can create projects using the same template, ensuring consistency and simplifying management.
* **Security**: Projects provide isolation of resources and access policies between different projects, supporting a secure multitenant environment.
* **Resource Consumption**: Administrators can easily set quotas on resources and limitations for each project, preventing excessive resource usage.

For platform users:
* **Quick Start**: Developers can request projects created from ready-made templates from administrators, allowing for a quick start to developing a new application.
* **Isolation**: Each project provides an isolated environment where developers can deploy and test their applications without impacting other projects.

## Limitations

The module works only within the limits below:

- Creating more than one namespace within a project is not supported. If you need multiple namespaces, create a separate project for each of them.
- Template resources are applied only to a single namespace whose name matches the project name.

## Internal Logic

### Creating a project

To create projects, the following [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) are used:
* [ProjectTemplate](./cr.html#projecttemplate) — a resource that describes the project template. It defines a list of resources to be created in the project and a schema for parameters that can be passed when creating the project;
* [Project](./cr.html#project) — a resource that describes a specific project.

When creating a [Project](./cr.html#project) resource from a specific [ProjectTemplate](./cr.html#projecttemplate), the following happens:
1. The [parameters](./cr.html#project-v1alpha2-spec-parameters) passed are validated against the OpenAPI specification (the [`parametersSchema.openAPIV3Schema`
](./cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) field of [ProjectTemplate](./cr.html#projecttemplate));
1. Rendering of the [resources template](./cr.html#projecttemplate-v1alpha1-spec-resourcestemplate) is performed using [Helm](https://helm.sh/docs/). Values for rendering are taken from the [`parameters`](./cr.html#project-v1alpha2-spec-parameters) field of the [Project](./cr.html#project) resource;
1. A `Namespace` is created with a name matching the name of [Project](./cr.html#project);
1. All resources described in the template are created in sequence.

> **Attention!** When changing the project template, all created projects will be updated according to the new template.

### Isolating a project

The project is based on the `Namespace` resource mechanism. Namespaces group pods, services, secrets, and other objects but do not provide complete isolation. The project functionality enhances namespaces by offering additional tools to improve control and security levels. To manage project isolation, Kubernetes features can be leveraged, such as:

- Access control resources (`AuthorizationRule` / `RoleBinding`) — manage interaction with objects within a `Namespace`. Define rules and assign roles to precisely control who can perform actions in your project.
- Resource quotas (`ResourceQuota`) — set limits on resource usage, such as CPU time, RAM, and object counts within a `Namespace`. These quotas help prevent excessive load and maintain control over applications within the project.
- Network connectivity control resources  (`NetworkPolicy`) — control incoming and outgoing network traffic within a `Namespace`. Configure allowed connections between pods to enhance security and manage network interactions effectively.

These tools can be combined to configure the project according to the requirements of your application.

## Managing access to cluster-wide resources

Objects in project namespaces can reference cluster-wide resources. For example, a PersistentVolumeClaim can use a StorageClass, a Certificate can use a ClusterIssuer, and a RoleBinding can use a ClusterRole. The module allows cluster administrators to define which cluster-wide resources can be used from project namespaces and which values are used by default.

This mechanism works independently of RBAC. RBAC determines *who can create and modify* objects, while the cluster-wide resource access mechanism determines *which resources* those objects can use.

### How access management works

The following resources are used to manage access to cluster-wide resources:

* [GrantableClusterResourceDefinition](./cr.html#grantableclusterresourcedefinition) registers a type of cluster-wide resource whose access can be managed. These resources are provided by DKP or module developers.
* [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference) defines where a registered cluster-wide resource is used, for example, which resource field contains a reference to it. These resources are provided by modules.
* [ClusterResourceGrantPolicy](./cr.html#clusterresourcegrantpolicy) defines access rules. Using labels, a cluster administrator selects the projects to which the policy applies and defines the allowed and denied resources, as well as the resource used by default.
* Based on the policy, the controller creates an [AvailableClusterResource](./cr.html#availableclusterresource) in the namespace of each matching project. This resource contains the list of cluster-wide resources available to the project and is read-only.
* When an object is created or modified, webhooks check whether it is allowed to use the specified cluster-wide resource. When an object is created, the resource configured as the project default can also be inserted automatically.

<script src="/assets/js/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>

<pre class="mermaid">
flowchart LR
    A["Module developer or DKP<br/>provides GrantableClusterResourceDefinition<br/>and GrantableClusterResourceReference"] --> C
    B["Cluster administrator<br/>creates<br/>ClusterResourceGrantPolicy"] --> C["Controller"]
    C --> D["Creates AvailableClusterResource<br/>in each project namespace"]
    E["User creates an object<br/>(for example,<br/>PersistentVolumeClaim)"] --> F["Mutating webhook<br/>/defaults"]
    F --> G["Validating webhook<br/>/is-granted"]
    D -. Available names .-> G
    G --> H["Object created<br/>or rejected"]
</pre>

Until the administrator creates a ClusterResourceGrantPolicy, resource availability is determined by their registration: resources are available to all projects if `defaultAvailability: All` (the default value) is set in GrantableClusterResourceDefinition and the resource does not match the `excluded` filters.

Resource quotas are not part of this mechanism. They are managed using the standard Kubernetes ResourceQuota resource.

Access checks are performed only for objects in project namespaces. If an access policy changes, cluster-wide resources already used by existing objects remain available to those objects.

### Access checks and default value assignment

The mechanism determines cluster-wide resource availability and default values based on registered resources and policies that apply to the project.

#### Determining cluster-wide resource availability

If multiple rules apply to a cluster-wide resource, its availability is determined in the following order:

1. The [`excluded`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-excluded) value in GrantableClusterResourceDefinition: the resource is unavailable regardless of policy settings.
2. The [`denied`](./cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-denied) and [`deniedSelector`](./cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-deniedselector) values in the corresponding ClusterResourceGrantPolicy entry.
3. The [`allowed`](./cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowed) and [`allowedSelector`](./cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-allowedselector) values in the corresponding ClusterResourceGrantPolicy entry.
4. The [`availabilityDefault`](./cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-availabilitydefault) value in the corresponding ClusterResourceGrantPolicy entry.
5. The [`defaultAvailability`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultavailability) value in GrantableClusterResourceDefinition.

The first matching rule applies.

#### Assigning default values

The behavior when an object is created depends on the mode configured in [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference-v1alpha1-spec-fieldpaths-defaulting):

* `None`: The value is checked for availability but is not assigned automatically.
* `FillEmpty`: If no value is specified, the project default is assigned.
* `Coerce`: If no value is specified or the specified cluster-wide resource is unavailable to the project, the project default is assigned.

The project default is determined in the following order:

1. The [`default`](./cr.html#clusterresourcegrantpolicy-v1alpha1-spec-resources-default) value from the corresponding ClusterResourceGrantPolicy entry.
2. The value determined using [`defaultFrom`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultfrom) in GrantableClusterResourceDefinition.
3. If no value is found, no default is assigned.

#### Checking existing objects

When an access policy changes, values already used by existing objects are preserved. When an object is updated, only new values are checked. If the value of a field has not changed, it remains valid even after access to the resource has been restricted.

This allows existing objects to continue operating after access to cluster-wide resources is restricted.

#### System requests

Requests from system service accounts, such as DKP's own controllers, are not subject to cluster-wide resource access checks. This allows platform system components to use the resources they require regardless of project policies.

### Monitoring access policy violations

If an existing object uses a cluster-wide resource that becomes unavailable to the project after a policy change, the object continues to operate, but the discrepancy is detected by the monitoring system.

When such objects are detected, the [`ClusterResourceGrantPolicyViolation`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation) alert is triggered. Information about violations is available on the Grafana dashboard under "Security" → "Cluster Resource Grant Violations".

The `d8_cluster_objects_grant_violated` metric is used for monitoring.

### Resources registered by DKP

DKP registers the following cluster-wide resources:

| Definition name | Cluster-wide resource | Where it is used | Default assignment mode |
| --- | --- | --- | --- |
| `storageclasses` | StorageClass (storage.k8s.io) | PersistentVolumeClaim `.spec.storageClassName` | `Coerce` |
| `loadbalancerclasses` | `loadbalancerclass` value | Service `.spec.loadBalancerClass` (for services of the `LoadBalancer` type) | `FillEmpty` |
| `clusterissuers` | ClusterIssuer (cert-manager.io) | Certificate `.spec.issuerRef.name`; Ingress: `cert-manager.io/cluster-issuer` annotation | `FillEmpty` or `None` |
| `clusterroles` | ClusterRole (rbac.authorization.k8s.io) | RoleBinding `.roleRef.name` | `None` |

The `clusterroles` registration excludes all ClusterRole objects without the `rbac.deckhouse.io/delegatable` label. By default, only namespace-level roles (`d8:use:role:*` and the deprecated `user-authz:*` roles) are available in RoleBinding.

The `clusterissuers` definition is registered only when the `cert-manager` module is enabled.

### Access management resources

| Resource | Scope | Created by | Manual creation | Purpose |
| --- | --- | --- | --- | --- |
| [GrantableClusterResourceDefinition](./cr.html#grantableclusterresourcedefinition) | Cluster | Module developer or DKP | Allowed for custom resources | Registers a type of cluster-wide resource whose access can be managed |
| [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference) | Cluster | Module developer | Allowed for fields of custom resources | Defines where a registered cluster-wide resource is used |
| [ClusterResourceGrantPolicy](./cr.html#clusterresourcegrantpolicy) | Cluster | Cluster administrator | Required | Defines allowed and denied resources, as well as the resource used by the project by default |
| [AvailableClusterResource](./cr.html#availableclusterresource) | Namespace | Controller (automatically) | Prohibited (protected by a webhook) | Read-only catalog of resources available to the project |

For detailed access configuration scenarios, viewing resources available to projects, access checks and default assignment rules, and recommendations for module developers, refer to ["Usage examples"](usage.html#managing-access-to-cluster-wide-resources).
