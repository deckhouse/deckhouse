---
title: "The multitenancy-manager module: usage examples"
---
{% raw %}

## Default project templates

The following project templates are included in the Deckhouse Kubernetes Platform:
- `default` — a template that covers basic project use cases:
  * resource limitation;
  * network isolation;
  * automatic alerts and log collection;
  * choice of security profile;
  * project administrators setup.

- `secure` — includes all the capabilities of the `default` template and additionally:
  * setting up permissible UID/GID for the project;
  * audit rules for project users' access to the Linux kernel;
  * scanning of launched container images for CVE presence.

## Creating a project

To create a project, you need to create a [Project](cr.html#project) resource specifying the name of the project template in the [.spec.projectTemplateName](cr.html#project-v1alpha1-spec-projecttemplate) field.
In the [.spec.template](cr.html#project-v1alpha1-spec-template) field of the `Project` resource, you need to specify the parameter values that are suitable for the [.spec.schema.openAPIV3Schema ProjectTemplate](cr.html#projecttemplate-v1alpha1-spec--schema-openAPIV3Schema).

Example of creating a project using the [Project](cr.html#project) resource from the `default` [ProjectTemplate](cr.html#projecttemplate):

```yaml
apiVersion: deckhouse.io/v1alpha1
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
    podSecurityPolicy: Restricted
    enableExtendedMonitoring: true
```

To check the status of the project, execute the command:

```shell
kubectl get projects my-project
```

A successfully created project should be in the "Synchronized" status.

## Creating your own project template

Default templates cover basic project use cases and serve as a good example of template capabilities.

To create your own template:
1. Take one of the default templates as a basis, for example, `default`.
2. Copy it to a separate file, for example, `my-project-template.yaml` using the command:

   ```shell
   kubectl get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Edit the `my-project-template.yaml` file, make the necessary changes. **Don't forget** that you need to change not only the template but also the schema of input parameters for it.
4. Change the template name in the [.metadata.name](cr.html#projecttemplate-v1alpha1-metadata-name) field.
5. Apply your new template with the command:

    ```shell
    kubectl apply -f my-project-template.yaml
    ```

> Project templates support all [Helm templating functions](https://helm.sh/docs/chart_template_guide/function_list/).

{% endraw %}
