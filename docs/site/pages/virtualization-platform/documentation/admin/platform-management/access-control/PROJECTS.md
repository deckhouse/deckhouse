---
title: "Projects"
permalink: en/virtualization-platform/documentation/admin/platform-management/access-control/projects.html
---

## Description

Deckhouse Virtualization Platform (DVP) projects (the [Project](/modules/multitenancy-manager/cr.html#project) resource) provide isolated environments for creating user resources.

Project settings let you set resource quotas and restrict network communication both within and outside DVP.

You can create a project using a template (the [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) resource).

{% alert level="warning" %}
If you modify a project template, all projects created from it will be updated to match the modified template.
{% endalert %}

## Default project templates

The following project templates are included in the DVP:

- `default` — a template that covers basic project use cases:
  * resource limitation;
  * network isolation;
  * automatic alerts and log collection;
  * choice of security profile;
  * project administrators setup.

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

- `secure` — includes all the capabilities of the `default` template and additional features:
  * setting up permissible UID/GID for the project;
  * audit rules for project users' access to the Linux kernel;
  * scanning of launched container images for CVE presence.

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

- `secure-with-dedicated-nodes` — includes all the capabilities of the `secure` template and additional features:
  * defining the node selector for all the VMs in the project: if a VM is created, the node selector VM will be **substituted** with the project's node selector automatically;
  * defining the default toleration for all the VMs in the project: if a VM is created, the default toleration will be **added** to the VM automatically.

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

To list all available parameters for a project template, execute the command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Creating a project

1. To create a project, create the [Project](/modules/multitenancy-manager/cr.html#project#project) resource by specifying the name of the project template in [.spec.projectTemplateName](/modules/multitenancy-manager/cr.html#project#project-v1alpha2-spec-projecttemplatename) field.
2. In the [.spec.parameters](/modules/multitenancy-manager/cr.html#project#project-v1alpha2-spec-parameters) field of the `Project` resource, specify the parameter values suitable for the `ProjectTemplate` [.spec.parametersSchema.openAPIV3Schema](/modules/multitenancy-manager/cr.html#projecttemplate#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema).

   Example of creating a project using the [Project](/modules/multitenancy-manager/cr.html#project) resource from the `default` [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate):

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

3. To check the status of the project, execute the command:

   ```shell
   d8 k get projects my-project
   ```

   A successfully created project should be in the `Deployed` state. If the state equals `Error`, add the `-o yaml` argument to the command (e.g., `d8 k get projects my-project -o yaml`) to get more detailed information about the error.

### Creating a project automatically for a namespace

You can create a new project for a namespace. To do this, add the `projects.deckhouse.io/adopt` annotation to the namespace. For example:

1. Create a new namespace:

   ```shell
   d8 k create ns test
   ```

1. Add the annotation:

   ```shell
   d8 k annotate ns test projects.deckhouse.io/adopt=""
   ```

1. Make sure that the project was created:

   ```shell
   d8 k get projects
   ```

   A new project corresponding to the namespace will appear in the project list:

   ```shell
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   empty                                                                     1m
   ```

You can change the template of the created project to the existing one.

{% alert level="warning" %}
Note that changing the template may cause a resource conflict. If the template chart contains resources that are already present in the namespace, you will not be able to apply the template.
{% endalert %}

## Creating your own project template

Default templates cover basic project use cases and serve as a good example of template capabilities.

To create your own template:
1. Take one of the default templates as a basis, for example, `default`.
1. Copy it to a separate file, for example, `my-project-template.yaml` using the command:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Edit the `my-project-template.yaml` file, make the necessary changes.

   > It is necessary to change not only the template, but also the scheme of input parameters for it.
   >
   > Project templates support all [Helm templating functions](https://helm.sh/docs/chart_template_guide/function_list/).

1. Change the template name in the `.metadata.name` field.
1. Apply your new template with the command:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Check the availability of the new template with the command:

   ```shell
   d8 k get projecttemplates <NEW_TEMPLATE_NAME>
   ```
