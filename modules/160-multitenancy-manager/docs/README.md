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

## What a project consists of

A project combines:

- **The main namespace** — created automatically, its name matches the project name.
- **Additional namespaces** (optional) — created with the [ProjectNamespace](./cr.html#projectnamespace) resource and named `<project name>-<name>`. The project settings (security policies, network policies, log collection, etc.) and the access granted to the project automatically apply to all of its namespaces.
- **Standard fields** — the list of administrators ([`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators)) and resource quotas ([`.spec.quota`](./cr.html#project-v1alpha3-spec-quota)), managed by the [Project](./cr.html#project) resource itself independently of the template.
- **Resources from the template** (optional) — policies and settings described in a [ProjectTemplate](./cr.html#projecttemplate) and created by the controller in the project namespaces.

## Internal Logic

### Creating a project

To create projects, the following [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) are used:
* [ProjectTemplate](./cr.html#projecttemplate) — a resource that describes the project template: which settings and policies its namespaces get and which parameters can be passed when creating a project;
* [Project](./cr.html#project) — a resource that describes a specific project.

When creating a [Project](./cr.html#project) resource from a specific [ProjectTemplate](./cr.html#projecttemplate), the following happens:
1. The [parameters](./cr.html#project-v1alpha3-spec-parameters) passed are validated against the OpenAPI specification (the [`parametersSchema.openAPIV3Schema`](./cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) field of [ProjectTemplate](./cr.html#projecttemplate));
1. A `Namespace` is created with a name matching the name of [Project](./cr.html#project);
1. The project resources are created from the template:
   - in modern templates (`deckhouse.io/v1alpha2`) the settings are described by structured fields (pod security profile, network isolation, log collection, node placement, etc. — [see details](usage.html#structured-templates)); the controller itself creates the corresponding objects in every namespace of the project;
   - in legacy templates (`deckhouse.io/v1alpha1`) the [resources template](./cr.html#projecttemplate-v1alpha1-spec-resourcestemplate) is rendered using [Helm](https://helm.sh/docs/); values are taken from the [`parameters`](./cr.html#project-v1alpha3-spec-parameters) field of the [Project](./cr.html#project) resource;
1. The standard fields of the project are applied independently of the template: [`.spec.quota`](./cr.html#project-v1alpha3-spec-quota) is reconciled into a `ResourceQuota`, and [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators) into an auto-managed [ProjectRoleBinding](./cr.html#projectrolebinding).

The Project API is served as `deckhouse.io/v1alpha3`. A conversion webhook keeps older `v1alpha1`/`v1alpha2` manifests working by lifting `parameters.administrators` and `parameters.resourceQuota` into the standard fields. The `projectTemplateName` field is optional: a project without a template only manages its namespace and standard fields.

Project names are validated on creation: names longer than 61 characters and names with the system prefixes `d8-` and `kube-` are not allowed. Also, if a project `foo` exists, a project `foo-bar` cannot be created (and vice versa): names like `foo-*` are reserved for the additional namespaces of the `foo` project.

> **Attention!** When changing the project template, all created projects will be updated according to the new template.

### Multiple namespaces in a project

A project can span several namespaces. Additional namespaces are created with the [ProjectNamespace](./cr.html#projectnamespace) resource in the main namespace of the project and are named `<project name>-<name>` (for example, `backend-cache` for the `backend` project).

The following automatically applies to the additional namespaces:

- the project template settings — the pod security profile, network policies, log collection, and other policies are created in every namespace of the project;
- the access granted via [ProjectRoleBinding](./cr.html#projectrolebinding) and [ClusterProjectRoleBinding](./cr.html#clusterprojectrolebinding), including the automatic access of the project administrators.

The project quota ([`.spec.quota`](./cr.html#project-v1alpha3-spec-quota)) applies in the main namespace.

Deleting a project deletes all of its namespaces — both the main and the additional ones. See [the usage examples](usage.html#additional-project-namespaces) for details.

### Access to a project

User access to a project is managed at the level of the whole project rather than individual namespaces:

- The subjects listed in [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators) automatically get the project administrator role (`d8:project:admin`).
- The [ProjectRoleBinding](./cr.html#projectrolebinding) resource grants a role in all namespaces of one project.
- The [ClusterProjectRoleBinding](./cr.html#clusterprojectrolebinding) resource grants a role in all projects of the cluster at once — convenient, for example, for a monitoring team.

The bindings use the project and namespace roles of the DKP role model (`d8:project:*`, `d8:namespace:*`, and their custom variants) — see [the user-authz module documentation](../user-authz/) for details on the roles. See [the usage examples](usage.html#granting-access-within-a-project) for details on the bindings.

### Automatic project creation for namespaces

By default (the [`allowNamespacesWithoutProjects: true`](configuration.html#parameters-allownamespaceswithoutprojects) parameter), users can still create namespaces directly (`kubectl create namespace`). To keep such namespaces under management, the module automatically "wraps" each of them into a project with the same name:

- the project is labelled `multitenancy.deckhouse.io/project-managed-by-namespace`, and the namespace remains the source of truth: its labels and annotations are synced into the project;
- the namespace can be freely edited and deleted — deleting it also deletes the project;
- the project itself cannot be edited manually (except for removing the `multitenancy.deckhouse.io/project-managed-by-namespace` label, which turns it into a regular project).

If the `allowNamespacesWithoutProjects` parameter is disabled, creating namespaces outside of projects is prohibited — new environments are created only via the [Project](./cr.html#project) resource.

### Isolating a project

The project is based on the `Namespace` resource mechanism. Namespaces group pods, services, secrets, and other objects but do not provide complete isolation. The project functionality enhances namespaces by offering additional tools to improve control and security levels. To manage project isolation, Kubernetes features can be leveraged, such as:

- Access control resources (`AuthorizationRule` / `RoleBinding`) — manage interaction with objects within a `Namespace`. Define rules and assign roles to precisely control who can perform actions in your project.
- Resource quotas (`ResourceQuota`) — set limits on resource usage, such as CPU time, RAM, and object counts within a `Namespace`. These quotas help prevent excessive load and maintain control over applications within the project.
- Network connectivity control resources  (`NetworkPolicy`) — control incoming and outgoing network traffic within a `Namespace`. Configure allowed connections between pods to enhance security and manage network interactions effectively.

These tools can be combined to configure the project according to the requirements of your application.

