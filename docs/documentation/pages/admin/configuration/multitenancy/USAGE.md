---
title: Project management
permalink: en/admin/multitenancy/project-management.html
description: Project management
---

Deckhouse Kubernetes Platform includes a set of templates for creating projects:

- `default` is a template for basic project use cases:
  - resource limits
  - network isolation
  - automatic alerts and log collection
  - selecting a security profile
  - configuring project administrators

- `secure` includes all capabilities of the `default` template and adds:
  - configuring allowed UID/GID ranges for the project
  - audit rules for Linux users’ interactions with the kernel
  - scanning container images at runtime for known vulnerabilities (CVEs)

- `secure-with-dedicated-nodes` includes all capabilities of the `secure` template and adds:
  - defining a node selector for all pods in the project: when a pod is created, its node selector is automatically **replaced** with the project’s node selector
  - defining default tolerations for all pods in the project: when a pod is created, the default tolerations are automatically **added** to it

To list all available parameters for a project template, run:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Project creation

1. To create a project, create a custom resource [Project](/modules/multitenancy-manager/cr.html#project) with the project template name specified in the [.spec.projectTemplateName](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-projecttemplatename) field.
1. In the [.spec.parameters](/modules/multitenancy-manager/cr.html#project-v1alpha2-spec-parameters) parameter, specify the values for the [.spec.parametersSchema.openAPIV3Schema](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema) section of the custom resource [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate).

   An example of creating a project using [Project](/modules/multitenancy-manager/cr.html#project) from the `default` [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) is shown below:

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

   A successfully created project should have the `Deployed` (synced) status. If the status is `Error`, add the `-o yaml` flag to the command (for example, `d8 k get projects my-project -o yaml`) to get more details about the error cause.

### Automatic project creation for a namespace

You can create a new project for an existing namespace. To do this, add the `projects.deckhouse.io/adopt` annotation to the namespace. For example:

1. Create a new namespace:

   ```shell
   d8 k create ns test
   ```

1. Annotate it with the following command:

   ```shell
   d8 k annotate ns test projects.deckhouse.io/adopt=""
   ```

1. Make sure the project has been created:

   ```shell
   d8 k get projects
   ```

   A new project matching the namespace will appear in the list of projects:

   ```shell
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

You can change the template of an existing project to another available template.

{% alert level="warning" %}
Note that changing the template may cause resource conflicts: if the template chart defines resources that already exist in the namespace, the template cannot be applied.
{% endalert %}

## Creating a custom project template

The default project templates cover common baseline scenarios and also serve as examples of what templates can do.

To create your own template:

1. Use one of the default templates as a starting point, for example, `default`.
1. Export it to a separate file, for example, `my-project-template.yaml`, using the following command:

   ```shell
   d8 k get projecttemplates default -o
   ```

1. Edit the `my-project-template.yaml` file and apply the required changes.

   > You must update not only the template itself, but also the input parameters schema to match it.
   >
   > Project templates support all [Helm templating functions](https://helm.sh/docs/chart_template_guide/function_list/).

1. Change the template name in the `.metadata.name` field.

1. Apply the resulting template with the following command:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Check that the new template is available by running:

   ```shell
   d8 k get projecttemplates <NEW_TEMPLATE_NAME>
   ```
