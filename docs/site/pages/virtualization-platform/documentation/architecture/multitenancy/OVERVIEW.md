---
title: Multitenancy
permalink: en/virtualization-platform/documentation/architecture/multitenancy/
lang: en
---

## Internal logic

### Project creation

The following custom resources are used to create a project:

* [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate): Describes the project template.
  It defines the list of resources to be created in the project
  and the parameter schema that can be passed during project creation.
* [Project](/modules/multitenancy-manager/cr.html#project): Describes a specific project.

When a Project is created from a specified ProjectTemplate, the following occurs:

1. The provided [parameters](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) are validated against the OpenAPI specification
   (defined in the [`openAPI`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-parametersschema) parameter of the [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) resource).
1. The [resource template](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-resourcestemplate) is rendered using [Helm](https://helm.sh/docs/).
   The rendering values are taken from the [`parameters`](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) field of the [Project](/modules/multitenancy-manager/cr.html#project) resource.
1. A namespace is created with the same name as the [Project](/modules/multitenancy-manager/cr.html#project).
1. All resources described in the template are created one by one.

{% alert level="warning" %}
If you modify a project template, all previously created projects will be updated according to the new template.
{% endalert %}

### Project isolation

Project isolation is based on the resource isolation mechanism provided by namespaces.
Namespaces allow grouping pods, services, secrets, and other objects, but they do not offer full isolation.
Projects extend namespace functionality by providing additional tools to improve control and security.

To manage project isolation scale, you can use the following Kubernetes features:

* **Access control resources** (AuthorizationRule / RoleBinding): Let you manage interactions between objects within a namespace.
  With them you can define rules and assign roles to precisely control who can do what in a project.
* **Usage control resources** (ResourceQuota): Let you define limits for CPU, RAM, and number of objects within a namespace.
  This helps prevent resource overconsumption and provides monitoring over project applications.
* **Network connectivity control resources** (NetworkPolicy): Let you manage incoming and outgoing network traffic in a namespace.
  You can configure allowed connections between pods and improve security and network manageability within a project.

You can combine these tools to configure a project according to your application's requirements.
