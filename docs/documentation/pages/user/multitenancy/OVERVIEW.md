---
title: "Using multitenancy"
description: "Configuring multitenancy in Deckhouse Platform. Creating isolated projects with resource quotas, security policies, and network isolation. Using ProjectTemplate to manage environments."
permalink: en/user/multitenancy/
lang: en
---

Multitenancy is the ability to create isolated environments (projects) within a Kubernetes cluster.
Projects are similar to namespaces but offer more capabilities.
While namespaces are used for logical separation of resources in Kubernetes,
they do not, for example, restrict network communication, pod resource consumption, or host directory mounts.
These limitations make namespaces insufficient for modern development needs.
By default, namespaces also do not include logging, auditing, and vulnerability scanning.

Using projects helps address these limitations and offers the following benefits:

* For platform administrators:
  * **Consistency**: Administrators can create projects using a shared template,
  which ensures consistency and simplifies management.
  * **Security**: Projects ensure isolation for resources and access policies between different tenants,
  supporting a secure multitenant environment.
  * **Resource consumption**: Administrators can easily set resource quotas and limits for each project
  to prevent resource overconsumption.

* For platform users:
  * **Immediate start**: Developers can request projects created per templates from administrators
  to quickly start developing new applications.
  * **Isolation**: Each project provides an isolated environment,
  allowing developers to deploy and test their applications without affecting others.

{% alert level="warning" %}
[Secret copying](/modules/secret-copier/) across all namespaces is incompatible with projects in multitenancy mode.

This mode creates isolated environments for users within their projects,
while `secret-copier` automatically distributes secrets to all namespaces.
If sensitive data is present in a user’s private environment,
it could lead to a data leak and a security model breach.
{% endalert %}

## Additional namespaces

A project is not limited to a single namespace: if an application needs several namespaces (for example, a separate one for a cache or a queue), a cluster or project administrator can add them to the project as additional namespaces. Access grants and namespaced template policies (network isolation, log shipping) automatically apply to every namespace of the project, not just the main one. For details, refer to the [Administration section](../../admin/multitenancy/project-management.html#additional-project-namespaces).

## Creating a project

1. To create a project, create a [Project](/modules/multitenancy-manager/cr.html#project) custom resource
   and specify the project template name in the [`.spec.projectTemplateName`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-projecttemplatename) field.
1. Set the standard fields — [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) and [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) — which are managed directly by the Project resource regardless of the template.
1. In the [`.spec.parameters`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-parameters) field,
   specify parameter values for the [`.spec.parametersSchema.openAPIV3Schema`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) section of the `ProjectTemplate` custom resource.

   Example of creating a project using [Project](/modules/multitenancy-manager/cr.html#project) from the `default` [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate):

   ```yaml
   apiVersion: deckhouse.io/v1alpha3
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     # Standard fields, managed by the Project resource itself, independently of the template.
     administrators:
       - kind: Group
         name: k8s-admins
     quota:
       requests.cpu: "5"
       requests.memory: 5Gi
       requests.storage: 1Gi
       limits.cpu: "5"
       limits.memory: 5Gi
     # Template-specific parameters.
     parameters:
       networkPolicy: Isolated
       podSecurityProfile: Restricted
       extendedMonitoringEnabled: true
   ```

1. To check the project status, run the following command:

   ```shell
   d8 k get projects my-project
   ```

   A successfully created project will have a `Deployed` status.
   If the `Error` status is displayed instead,
   run the same command with the `-o yaml` flag to get details about the cause:

   ```shell
   d8 k get projects my-project -o yaml
   ```

### Automatically creating a project from a namespace

A namespace created directly (for example, `d8 k create ns test`) automatically becomes a project with the same name — no annotation is required. The project parameters are filled in from the current state of the namespace, so nothing inside it changes; from then on the project is the source of truth for it.
For example:

1. Create a new namespace:

   ```shell
   d8 k create ns test
   ```

1. Check that the project was created:

   ```shell
   d8 k get projects
   ```

   In the output list of projects, you should see the newly created project corresponding to the namespace:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   simple                                                                    1m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

You can change the template of the created project to an existing one.

{% alert level="warning" %}
Note that changing the template might cause resource conflicts.
If the new template’s chart defines resources that already exist in the namespace, the template can't be applied.
{% endalert %}

For details on project templates and their creation, refer to the [Administration section](../../admin/multitenancy.html).

## Viewing available cluster-wide resources

A cluster administrator can restrict the use of cluster-wide resources in projects. For example, the administrator can define which StorageClasses, ClusterIssuers, and ClusterRoles are available to a project.

To view the cluster-wide resources available to a project, run the following command:

```shell
d8 k get available -n <PROJECT_NAME>
```

To view available resources of a specific type and the value used by default, specify the corresponding AvailableClusterResource. For example, for StorageClass:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

If you see a message such as `[multitenancy] <KIND> "<OBJECT_NAME>" references "<RESOURCE_NAME>" which is not available to project "<PROJECT_NAME>"` when creating or modifying a resource, the specified cluster-wide resource is unavailable to the project. Select an available resource from the corresponding AvailableClusterResource or ask the cluster administrator to add it.

For some fields, automatic default value assignment can be configured. For example, if `storageClassName` is not specified when creating a PersistentVolumeClaim, the StorageClass used by default in the project can be automatically assigned to this field.

For more information about using available cluster-wide resources, refer to the [`multitenancy-manager`](/modules/multitenancy-manager/usage.html#for-project-users) module documentation.
