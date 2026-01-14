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

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

- `secure` — includes all the capabilities of the `default` template and additional features:
  * setting up permissible UID/GID for the project;
  * audit rules for project users' access to the Linux kernel;
  * scanning of launched container images for CVE presence.

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

- `secure-with-dedicated-nodes` — includes all the capabilities of the `secure` template and additional features:
  * defining the node selector for all the pods in the project: if a pod is created, the node selector pod will be **substituted** with the project's node selector automatically;
  * defining the default toleration for all the pods in the project: if a pod is created, the default toleration will be **added** to the pod automatically.

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

To list all available parameters for a project template, execute the command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
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

{% endraw %}

{% alert level="warning" %}
Note that changing the template may cause a resource conflict. If the template chart contains resources that are already present in the namespace, you will not be able to apply the template.
{% endalert %}

{% raw %}

## Special labels for resource management

When creating resources in a `ProjectTemplate`, you can use special labels to control how the multitenancy-manager handles them:

### Skipping the heritage label

By default, all resources created from a `ProjectTemplate` receive the `heritage: multitenancy-manager` label. If you need to exclude a resource from receiving this label (for example, to maintain compatibility with other systems that use the `heritage` label), add the `projects.deckhouse.io/skip-heritage-label` label to the resource.

Example:

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/skip-heritage-label: "true"
    app: my-app
data:
  key: value
```

In this case, the resource will still receive the `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` labels, but will not receive the `heritage: multitenancy-manager` label.

### Excluding resources from management

If you need to exclude a resource from multitenancy-manager control (for example, if the resource should be managed manually or by another controller), add the `projects.deckhouse.io/unmanaged` label to the resource.

Example:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: external-secret
  namespace: {{ .projectName }}
  labels:
    projects.deckhouse.io/unmanaged: "true"
type: Opaque
data:
  token: <base64-encoded-value>
```

Resources with the `projects.deckhouse.io/unmanaged` label:
- Will be created **only once** during the first installation of the project;
- Will **not be updated** on subsequent template changes or upgrades;
- Will not be tracked in the project status;
- Will receive the `helm.sh/resource-policy: keep` annotation to prevent deletion when the release is uninstalled;
- Will receive `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` labels, but **will not receive** the `heritage: multitenancy-manager` label.

{% alert level="warning" %}
Once a resource is marked as unmanaged, it will be created during the first installation but will not be updated when the `ProjectTemplate` changes. After creation, the resource becomes fully independent and must be managed manually. Make sure you understand the implications before using this label.
{% endalert %}

## Implementing validation for resources with custom heritage value

The multitenancy-manager module uses `ValidatingAdmissionPolicy` to protect resources with the `heritage: multitenancy-manager` label from manual modifications. You can implement similar validation for resources with a different `heritage` label value.

### How validation works in multitenancy-manager

The validation is implemented in the `templates/validation.yaml` file and uses the following components:

1. **ValidatingAdmissionPolicy** — defines validation rules:
   - Operations: `UPDATE` and `DELETE`
   - Check: only operations from the controller's service account are allowed
   - Applies to all resources and API groups

2. **ValidatingAdmissionPolicyBinding** — binds the policy to resources:
   - Uses `namespaceSelector` and `objectSelector` to select resources by the `heritage: multitenancy-manager` label

### Creating your own validation

To implement validation for resources with a different `heritage` value (e.g., `heritage: my-custom-manager`):

1. Create a file with `ValidatingAdmissionPolicy` and `ValidatingAdmissionPolicyBinding`:

   ```yaml
   ---
   apiVersion: admissionregistration.k8s.io/v1beta1
   kind: ValidatingAdmissionPolicy
   metadata:
     name: my-custom-manager-validation
   spec:
     failurePolicy: Fail
     matchConstraints:
       resourceRules:
         - apiGroups:   ["*"]
           apiVersions: ["*"]
           operations:  ["UPDATE", "DELETE"]
           resources:   ["*"]
           scope: "*"
     validations:
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"'
         reason: Forbidden
         messageExpression: 'object.kind == ''Namespace'' ? ''This resource is managed by '' + object.metadata.name + '' system. Manual modification is forbidden.''
           : ''This resource is managed by '' + object.metadata.namespace + '' system. Manual modification is forbidden.'''
   ---
   apiVersion: admissionregistration.k8s.io/v1beta1
   kind: ValidatingAdmissionPolicyBinding
   metadata:
     name: my-custom-manager-validation
   spec:
     policyName: my-custom-manager-validation
     validationActions: [Deny, Audit]
     matchResources:
       namespaceSelector:
         matchLabels:
           heritage: my-custom-manager
       objectSelector:
         matchLabels:
           heritage: my-custom-manager
   ```

2. Configure validation parameters:

   - **`policyName`** — unique policy name (must match in both Policy and Binding)
   - **`request.userInfo.username`** — service account name allowed to modify resources (replace with your service account)
   - **`heritage: my-custom-manager`** — `heritage` label value for your resources (replace with your value)
   - **`failurePolicy: Fail`** — policy on validation error:
     - `Fail` — reject the request on validation error
     - `Ignore` — ignore validation errors
   - **`validationActions`** — validation actions:
     - `Deny` — reject unauthorized operations
     - `Audit` — log operations for audit

3. Apply the policy:

   ```shell
   kubectl apply -f my-validation-policy.yaml
   ```

4. Ensure your resources have the corresponding `heritage` label:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: my-resource
     labels:
       heritage: my-custom-manager
   ```

### Important notes

- **Cluster requirements**: `ValidatingAdmissionPolicy` is available starting from Kubernetes 1.28. For older versions, use `ValidatingWebhookConfiguration`.
- **Service Account**: Ensure the specified service account exists and has the necessary permissions to manage resources.
- **Performance**: Validation runs for each resource modification request, which may affect performance with a large number of operations.
- **Testing**: Test the validation on a test cluster before applying it in production.

### Example for ValidatingWebhookConfiguration (Kubernetes < 1.28)

If your cluster doesn't support `ValidatingAdmissionPolicy`, use `ValidatingWebhookConfiguration`:

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: my-custom-manager-validation
webhooks:
  - name: my-custom-manager.validation.example.com
    clientConfig:
      service:
        name: my-validation-webhook
        namespace: my-namespace
        path: "/validate"
    rules:
      - apiGroups:   ["*"]
        apiVersions: ["*"]
        operations:  ["UPDATE", "DELETE"]
        resources:   ["*"]
    namespaceSelector:
      matchLabels:
        heritage: my-custom-manager
    objectSelector:
      matchLabels:
        heritage: my-custom-manager
    admissionReviewVersions: ["v1", "v1beta1"]
    sideEffects: None
    failurePolicy: Fail
```

In this case, you'll also need to implement a webhook server that checks `request.userInfo.username`.

## Creating your own project template

Default templates cover basic project use cases and serve as a good example of template capabilities.

To create your own template:
1. Take one of the default templates as a basis, for example, `default`.
2. Copy it to a separate file, for example, `my-project-template.yaml` using the command:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

3. Edit the `my-project-template.yaml` file, make the necessary changes.

   > It is necessary to change not only the template, but also the scheme of input parameters for it.
   >
   > Project templates support all [Helm templating functions](https://helm.sh/docs/chart_template_guide/function_list/).

4. Change the template name in the `.metadata.name` field.
5. Apply your new template with the command:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

6. Check the availability of the new template with the command:

   ```shell
   d8 k get projecttemplates <NEW_TEMPLATE_NAME>
   ```

{% endraw %}
