---
title: "Projects"
permalink: en/stronghold/documentation/admin/platform-management/access-control/projects.html
---

## Description

DVP projects (the [Project](/modules/multitenancy-manager/cr.html#project) resource) provide isolated environments for creating user resources.

Project settings let you set resource quotas and restrict network communication both within and outside DVP.

You can create a project using a template (the [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) resource).

{% alert level="warning" %}
If you modify a project template, all projects created from it will be updated to match the modified template.
{% endalert %}

## Default project templates

DVP provides the following set of project templates:

- `empty`: A blank template without predefined resources.

- `default`: A template for main project use cases:
  - Resource restrictions
  - Network isolation
  - Automated alerts and logging
  - Security profile selection
  - Project administration setup

- `secure`: Includes all features of the `default` template and some additional features:
  - Audit rules for project Linux users accessing the kernel

To get a list of all available project template parameters, run the following command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Create a project

1. For a new project, create a [Project](/modules/multitenancy-manager/cr.html#project) resource, specifying the project template name in the `.spec.projectTemplateName` field.
1. In the `.spec.parameters` parameter of the [Project](/modules/multitenancy-manager/cr.html#projecttemplate) resource.

    Example of creating a project using the Project resource based on the `default` project template:

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

    A properly created project has the `Deployed` (synchronized) status.
    If the displayed status is `Error` and you want to see error details,
    add the `-o yaml` argument to the command:

    ```shell
    d8 k get projects my-project -o yaml
    ```

## Create a custom project template

The default project templates cover main project use cases and demonstrate template capabilities.

To create a project template of your own, follow these steps:

1. Select one of the default templates, such as `default`.
1. Make a copy to a separate file (for example, to `my-project-template.yaml`) using the following command:

    ```shell
    d8 k get projecttemplates default -o yaml > my-project-template.yaml
    ```

1. Open and edit `my-project-template.yaml` to customize the template.

    > Make sure to modify not only the template, but also the corresponding parameter schema.
    >
    > Project templates support all [Helm template functions](https://helm.sh/docs/chart_template_guide/function_list/).

1. Change template name in the `.metadata.name` field.
1. Apply the customized template using the following command:

    ```shell
    d8 k apply -f my-project-template.yaml
    ```

1. To ensure the modified template is available, run the following command:

    ```shell
    d8 k get projecttemplates <NEW_TEMPLATE_NAME>
    ```
