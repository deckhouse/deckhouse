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

- `secure` — includes all the capabilities of the `default` template and additional features:
  * setting up permissible UID/GID for the project;
  * audit rules for project users' access to the Linux kernel;
  * scanning of launched container images for CVE presence.

- `secure-with-dedicated-nodes` — includes all the capabilities of the `secure` template and additional features:
  * defining the node selector for all the pods in the project: if a pod is created, the node selector pod will be **substituted** with the project's node selector automatically;
  * defining the default toleration for all the pods in the project: if a pod is created, the default toleration will be **added** to the pod automatically.

To list all available parameters for a project template, execute the command:

```shell
kubectl get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Creating a project

1. To create a project, create the [Project](cr.html#project) resource by specifying the name of the project template in [.spec.projectTemplateName](cr.html#project-v1alpha2-spec-projecttemplatename) field.
2. In the [.spec.parameters](cr.html#project-v1alpha2-spec-parameters) field of the `Project` resource, specify the parameter values suitable for the `ProjectTemplate` [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema).

   Example of creating a project using the [Project](cr.html#project) resource from the `default` [ProjectTemplate](cr.html#projecttemplate):

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
   kubectl get projects my-project
   ```

   A successfully created project should be in the `Sync` state. If the state equals `Error`, add the `-o yaml` argument to the command (e.g., `kubectl get projects my-project -o yaml`) to get more detailed information about the error.

## Creating your own project template

Default templates cover basic project use cases and serve as a good example of template capabilities.

To create your own template:
1. Take one of the default templates as a basis, for example, `default`.
2. Copy it to a separate file, for example, `my-project-template.yaml` using the command:

   ```shell
   kubectl get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Edit the `my-project-template.yaml` file, make the necessary changes.

   > It is necessary to change not only the template, but also the scheme of input parameters for it.
   >
   > Project templates support all [Helm templating functions](https://helm.sh/docs/chart_template_guide/function_list/).

4. Change the template name in the `.metadata.name` field.
5. Apply your new template with the command:

   ```shell
   kubectl apply -f my-project-template.yaml
   ```

6. Check the availability of the new template with the command:

   ```shell
   kubectl get projecttemplates <NEW_TEMPLATE_NAME>
   ```

{% endraw %}
