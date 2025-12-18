---
title: Multitenancy
permalink: en/admin/multitenancy.html
description: Multitenancy
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
while [`secret-copier`](/modules/secret-copier/) automatically distributes secrets to all namespaces.
If sensitive data is present in a user’s private environment,
it could lead to a data leak and a security model breach.
{% endalert %}

## Limitations

Projects has several limitations:

- Creating more than one namespace within a project is not supported. If you need multiple namespaces, create a separate project for each of them.
- Template resources are applied only to a single namespace whose name matches the project name.

## Project templates

DKP includes a number of templates for creating projects:

* `default`: A template for common project use cases:
  * Resource limits.
  * Network isolation.
  * Automatic alerting and logging.
  * Security profile selection.
  * Project administrator configuration.

  Template description [on GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

* `secure`: Includes all `default` features, plus:
  * Allowed UID/GID settings for the project.
  * Audit rules for project Linux user access to the kernel.
  * Container image vulnerability scanning (CVE check).

  Template description [on GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

* `secure-with-dedicated-nodes`: Includes all `secure` features, plus:
  * Defining a node selector for all pods in the project.
  When a pod is created, its node selector will be **automatically replaced** with the project's node selector.
  * Defining default tolerations for all pods in the project.
  When a pod is created, default tolerations are **added automatically**.

  Template description [on GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

To list all available parameters for a project template, run the following command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

### Creating a custom project template

The default templates cover common use cases and demonstrate the available features.

To create a custom template:

1. Start by copying one of the default templates, for example, `default`.
2. Save it to a separate file, for example, `my-project-template.yaml`, using the following command:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Edit `my-project-template.yaml` to make necessary changes.

   > Update both the template and the input parameter schema.
   >
   > Project templates support all [Helm template functions](https://helm.sh/docs/chart_template_guide/function_list/).

4. Change the template name in `.metadata.name`.
5. Apply your new template with the following command:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

6. Ensure the new template is available with the following command:

   ```shell
   d8 k get projecttemplates <NEW_TEMPLATE_NAME>
   ```

## Creating a project

To create a project, use the [Project](/modules/multitenancy-manager/cr.html#project) custom resource
and specify the template name in the [`.spec.projectTemplateName`](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-projecttemplatename) field.

Parameter values are set in the [`.spec.parameters`](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) field,
which corresponds to the [`.spec.parametersSchema.openAPIV3Schema`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) section of the ProjectTemplate resource.

You can also automatically create a project from an existing namespace in the cluster.

For details on creating projects, refer to the [Usage section](../user/multitenancy/).
