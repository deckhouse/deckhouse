---
title: "Module multitenancy-manager"
search: multitenancy
description: The multitenancy-manager module adds the functionality to create projects for various development teams with the ability to subsequently deploy applications in them.
---
## Description

The module allows creating projects in a Kubernetes cluster. **Project** is an isolated environment where applications can be deployed.
The isolation of projects is achieved by creating separate `Namespaces` for each project with pre-filled resources such as `ResourceQuota`, `NetworkPolicy`, and so on.

## Advantages of the module

For platform administrators:
* **Security**: Projects provide isolation of resources and access policies between different projects, supporting a secure multitenant environment.
* **Resource Consumption**: Administrators can easily set quotas on resources and limitations for each project, preventing excessive resource usage.

For platform users:
* **Isolation**: Each project provides an isolated environment where developers can deploy and test their applications without impacting other projects.

## Internal Logic

To create projects, the following [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) are used:
* [ProjectTemplate](cr.html#projecttemplate) — a resource that describes the project template. It defines a list of resources to be created in the project and a schema for parameters that can be passed when creating the project;
* [Project](cr.html#project) — a resource that describes a specific project.

When creating a [Project](cr.html#project) resource from a specific [ProjectTemplate](cr.html#projecttemplate), the following happens:
1. The [parameters](cr.html#project-v1alpha2-spec-parameters) passed are validated against the OpenAPI specification (the [openAPI](cr.html#projecttemplate-v1alpha1-spec-parametersSchema) parameter of [ProjectTemplate](cr.html#projecttemplate));
1. Rendering of the [resources template](cr.html#projecttype-v1alpha1-spec-resourcestemplate) is performed using [Helm](https://helm.sh/docs/). Values for rendering are taken from the [template](cr.html#project-v1alpha2-spec-template) parameter of the [Project](cr.html#project) resource;
1. A `Namespace` is created with a name matching the name of [Project](cr.html#project);
1. All resources described in the template are created in sequence.

> **Attention!** When changing the project template, all created projects will be updated according to the new template.
