---
title: Multitenancy
permalink: en/user/multitenancy/
description: Multitenancy
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

## Limitations

Projects has several limitations:

- Creating more than one namespace within a project is not supported. If you need multiple namespaces, create a separate project for each of them.
- Template resources are applied only to a single namespace whose name matches the project name.

## Creating a project

1. To create a project, create a [Project](/modules/multitenancy-manager/cr.html#project) custom resource
   and specify the project template name in the [`.spec.projectTemplateName`](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-projecttemplatename) field.
1. In the [`.spec.parameters`](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) field,
   specify parameter values for the [`.spec.parametersSchema.openAPIV3Schema`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) section of the `ProjectTemplate` custom resource.

   Example of creating a project using [Project](/modules/multitenancy-manager/cr.html#project) from the `default` [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate):

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     parameters:
       resourceQuota:
         requests:
           cpu: 5
           memory: 5Gi
           storage: 1Gi
         limits:
           cpu: 5
           memory: 5Gi
       networkPolicy: Isolated
       podSecurityProfile: Restricted
       extendedMonitoringEnabled: true
       administrators:
       - subject: Group
         name: k8s-admins
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

You can create a new project from a namespace by adding the `projects.deckhouse.io/adopt` annotation to it.
For example:

1. Create a new namespace:

   ```shell
   d8 k create ns test
   ```

1. Annotate it:

   ```shell
   d8 k annotate ns test projects.deckhouse.io/adopt=""
   ```

1. Check that the project was created:

   ```shell
   d8 k get projects
   ```

   In the output list of projects, you should see the newly created project corresponding to the namespace:

   ```console
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

You can change the template of the created project to an existing one.

{% alert level="warning" %}
Note that changing the template might cause resource conflicts.
If the new template’s chart defines resources that already exist in the namespace, the template can't be applied.
{% endalert %}

For details on project templates and their creation, refer to the [Administration section](../../admin/multitenancy.html).
