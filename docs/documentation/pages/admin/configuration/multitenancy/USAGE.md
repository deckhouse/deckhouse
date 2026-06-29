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

## Using labels to manage resources

When creating resources in ProjectTemplate, you can use special labels to control how the `multitenancy-manager` processes these resources.

### Skipping creation of the `heritage: multitenancy-manager` label

By default, all resources created from ProjectTemplate receive the label `heritage: multitenancy-manager`.  
This label prohibits changes to resources by users or any other controller except `multitenancy-manager`.  
If you need to allow resource modification (for example, for compatibility with other systems, or if implementing your own control over the created objects), add the label `projects.deckhouse.io/skip-heritage-label` to the resource.

Example:

{% raw %}

```yaml
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

{% endraw %}

In this case, the resource will receive the labels `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template`, but will not receive the label `heritage: multitenancy-manager`.

### Excluding resources from management by multitenancy-manager

If you need to exclude a resource from management by `multitenancy-manager` (for example, if the resource should be managed manually or by another controller), add the label `projects.deckhouse.io/unmanaged` to the resource.

Example:

{% raw %}

```yaml
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

{% endraw %}

Resources with the label `projects.deckhouse.io/unmanaged`:

- Will be created **only once** when the project is created;
- **Will not be updated** with subsequent template changes or updates;
- Will not be monitored in the project's status;
- Will receive the labels `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` but **will not receive** the label `heritage: multitenancy-manager`.

{% alert level="warning" %}
Once a resource is marked as `unmanaged`, it will be created on initial installation but not updated when the ProjectTemplate is changed.  
After creation, the resource becomes fully independent and must be managed manually.
{% endalert %}

## Implementing validation of object changes with a custom label

The `multitenancy-manager` module uses ValidatingAdmissionPolicy to protect resources labeled `heritage: multitenancy-manager` from manual changes.  
You can implement similar validation for resources with any label.

### How validation works in multitenancy-manager

Validation occurs for objects labeled `heritage: multitenancy-manager`.  
The following components are used for this:

1. ValidatingAdmissionPolicy: Defines validation rules:
   - Operations: `UPDATE` and `DELETE`.
   - Check: only operations on behalf of the controller's service account are allowed.
   - Applies to all resources and API groups.
1. ValidatingAdmissionPolicyBinding: Defines which objects the validation applies to:
   - Uses `namespaceSelector` and `objectSelector` to select resources by the label `heritage: multitenancy-manager`.

### Creating your own validation

To implement validation for resources with a different label (for example, `heritage: my-custom-label`):

1. Create a file with the ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding resource manifests:

   ```yaml
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicy
   metadata:
     name: my-custom-label-validation
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
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Replace with your service account
         reason: Forbidden
         messageExpression: 'object.kind == ''Namespace'' ? ''This resource is managed by '' + object.metadata.name + '' system. Manual modification is forbidden.''
           : ''This resource is managed by '' + object.metadata.namespace + '' system. Manual modification is forbidden.'''
   ---
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicyBinding
   metadata:
     name: my-custom-label-validation
   spec:
     policyName: my-custom-label-validation
     validationActions: [Deny, Audit]
     matchResources:
       namespaceSelector:
         matchLabels:
           heritage: my-custom-label
       objectSelector:
         matchLabels:
           heritage: my-custom-label
   ```

1. Configure the validation parameters:

   - `policyName`: Unique policy name (must match in Policy and Binding).
   - `request.userInfo.username`: The name of the service account allowed to change resources (replace with your service account).
   - `heritage: my-custom-label`: The value of the `heritage` label for your resources (replace with your value). The use of the values `multitenancy-manager`, `deckhouse` is prohibited.
   - `failurePolicy: Fail`: Policy on validation failure.
     - `Fail`: Reject the request on validation failure.
     - `Ignore`: Ignore validation errors.
   - `validationActions`: Validation actions:
     - `Deny`: Deny unauthorized operations.
     - `Audit`: Record operations in the audit log.
1. Apply the policy:

   ```shell
   d8 k apply -f my-validation-policy.yaml
   ```

1. Ensure your resources have the corresponding `heritage` label:

   ```yaml
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: my-resource
     labels:
       heritage: my-custom-label
   ```

## Granting cluster-scoped resources to projects

The `multitenancy-manager` lets cluster administrators control which cluster-scoped resources (for example StorageClass) may be referenced from within project namespaces.

To do this, custom resources are used:

- GrantableClusterResourceDefinition (cluster-scoped) — registers a cluster resource that can be
  granted to projects: which resource it is (`grantedResource`), where references to it are validated (`usageReferences`),
  the baseline availability (`defaultAvailability`), and how the per-project default is discovered
  (`defaultFrom`). Each reference opts into defaulting individually with `default: true` — set it only
  for a field whose value the resource always needs (such as a `PersistentVolumeClaim`'s
  `storageClassName`). Leave it off for a reference whose absence is meaningful, such as an annotation
  that merely toggles a feature; that reference is still validated and counted, just never filled in.
- ClusterResourceGrantPolicy (cluster-scoped) — selects projects (by namespace labels via
  `projectSelector`) and, per resource (`resourceName`), the granted names (`allowed`,
  `allowedSelector`) and the per-project `default`. An allow-list restricts the resource to it.
- AvailableClusterResource (namespaced, read-only, short name `available`) — the controller-rendered
  catalog of what a project may use; tenants read it to discover the available names.
- ClusterResourceGrant (namespaced) — the per-project object-quota pool (limits on object count and
  on measured quantities such as requested storage); its status reports current usage.

{% raw %}

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: storageclasses
spec:
  grantedResource:
    apiVersion: storage.k8s.io/v1
    kind: StorageClass
  enforcement: Managed
  defaultAvailability: All
  defaultFrom:
    annotationKey: storageclass.kubernetes.io/is-default-class
  usageReferences:
    - rule:
        apiGroups:
          - ""
        apiVersions:
          - v1
        resources:
          - persistentvolumeclaims
      fieldPath: $.spec.storageClassName
      default: true
---
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: ClusterResourceGrantPolicy
metadata:
  name: production-storage
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: storageclasses
      default: fast-ssd          # Overrides the annotation-based default.
      allowed:
        - fast-ssd
        - standard
      allowedSelector:           # Plus any StorageClass with label shared=true.
        matchLabels:
          shared: "true"
```

{% endraw %}

Enforcement notes:

- The validating webhook denies creating/updating objects in matched projects whose
  referenced value is not granted. On update, values already present in the object are
  grandfathered in, so pre-existing objects are not broken.
- The defaulting webhook fills in the granted default on creation only, and only into references
  marked `default: true`. References left without it (such as feature-toggling annotations) are never
  filled in.
- A grant that matches no project, or a project with no matching grant, imposes no
  restriction.
