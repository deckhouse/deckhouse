---
title: "The multitenancy-manager module"
search: multitenancy
description: The multitenancy-manager Deckhouse module helps to conveniently create templated projects in a Kubernetes cluster using Custom Resources. The rendering of project type template with Helm makes it possible to use any Kubernetes objects in an project type.
---

The module allows you to create isolated projects in a Kubernetes cluster.

You can use a [project type](cr.html#projecttype) and a [Project](cr.html#project) custom resource to create identical, isolated projects in a Kubernetes cluster, each with users with access rights set up (see [Examples](usage.html) for more details).

Creating isolated projects using the `multitenancy-manager` module can be handy in the following cases:
- As a part of the CI/CD process — creating developer environments for testing or showcases.
- When deploying applications — providing limited access for a developer in a cluster.
- When cluster resources are shared between multiple tenants.

## Module features

- Managing user and group access via the RBAC Kubernetes mechanism (based on the [user-authz](../140-user-authz/) module).
- Managing isolation levels of particular projects.
- Creating templates for multiple projects and customizing by parameters according to OpenAPI specification.
- Fully `Helm`-compatible resource templates.

## How the module works

When a [Project](cr.html#project) resource is being created, the following things happen:
- A `Namespace` is created with the name from the [Project](cr.html#project) resource.
- An [AuthorizationRule](../140-user-authz/cr.html#authorizationrule) is created with the data specified in the [subjects](cr.html#projecttype-v1alpha1-spec-subjects) field of the project type.
- Templates (parameter [resourcesTemplate](cr.html#projecttype-v1alpha1-spec-resourcestemplate)) are rendered using [Helm](https://helm.sh/docs/). The values used for rendering are derived from the [template](cr.html#project-v1alpha1-spec-template) parameter of the project. During rendering, values are validated against the OpenAPI specification defined in the project type (parameter [openAPI](cr.html#projecttype-v1alpha1-spec-openapi)).

Since [templates](cr.html#projecttype-v1alpha1-spec-resourcestemplate) are rendered using `Helm`, you can define any necessary Kubernetes objects, such as `NetworkPolicy`, `LimitRange`, `ResourceQuota`, etc. in them.
