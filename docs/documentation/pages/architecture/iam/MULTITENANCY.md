---
title: Multitenancy
permalink: en/architecture/iam/multitenancy.html
lang: en
search: multitenancy, ProjectTemplate, Project, project isolation
description: How multitenancy works in Deckhouse Kubernetes Platform.
---

The [`multitenancy-manager`](/modules/multitenancy-manager/) module allows you to create isolated projects within the Deckhouse Kubernetes Platform (DKP). Projects provide resource quotas, network isolation, and security features that go beyond standard namespaces.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/multitenancy-manager/).

## Internal logic

### Project creation

The following custom resources are used to create a project:

* [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate): Describes the project template.
  It defines the list of resources to be created in the project
  and the parameter schema that can be passed during project creation.
* [Project](/modules/multitenancy-manager/cr.html#project): Describes a specific project.

When a Project is created from a specified ProjectTemplate, the following occurs:

1. The provided [parameters](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) are validated against the OpenAPI specification
   (defined in the [`openAPIV3Schema`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) parameter of the [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) resource).
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

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The architecture of the [`multitenancy-manager`](/modules/multitenancy-manager/) module at Level 2 of the C4 model and its interactions with other DKP components are shown in the following diagram.

<!--- Source: structurizr code --->
![Multitenancy-manager module architecture](../../images/architecture/iam/c4-l2-multitenancy-manager.png)

## Module components

The module consists of the following components:

- **Multitenancy-manager**: The component consists of a single **multitenancy-manager** container and provides the following functions:

  - Managing the Project and ProjectTemplate custom resources.
  - Validating the Project and ProjectTemplate custom resources.
  - Validating Namespace if [`.spec.settings.allowNamespacesWithoutProjects=false`](/modules/multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) is set in the `multitenancy-manager` module parameters.
  - Creating the resources specified in the ProjectTemplate custom resource based on the parameters set in Project.

   > **Warning.** Multitenancy-manager has `cluster-admin` permissions, which allow it to create any objects described in the ProjectTemplate resource.

## Module interactions

The module interacts with the following components:

- **Kube-apiserver**:
  - Managing the Project and ProjectTemplate custom resources.
  - Validating the Project and ProjectTemplate custom resources and the standard Namespace resource.
  - Creating the resources specified in the ProjectTemplate custom resource based on the parameters set in Project.
