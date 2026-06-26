---
title: "The multitenancy-manager module: usage examples"
---
{% raw %}

## Default project templates

The following project templates are included in the Deckhouse Kubernetes Platform:

- `simple` — a minimal template that creates only the project namespace (with optional extra labels and annotations). Use it when you only need an isolated namespace managed as a project and configure access and limits through the [standard fields](#standard-project-fields) and [project role bindings](#granting-access-within-a-project).

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/simple.yaml).

- `default` — a template that covers basic project use cases:
  - network isolation
  - automatic alerts and log collection
  - choice of security profile

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/default.yaml).

- `secure` — includes all the capabilities of the `default` template and additional features:
  - setting up permissible UID/GID for the project
  - audit rules for project users' access to the Linux kernel
  - scanning of launched container images for CVE presence

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure.yaml).

- `secure-with-dedicated-nodes` — includes all the capabilities of the `secure` template and additional features:
  - defining the node selector for all the pods in the project: if a pod is created, the node selector pod will be **substituted** with the project's node selector automatically.
  - defining the default toleration for all the pods in the project: if a pod is created, the default toleration will be **added** to the pod automatically.

  Template description on [GitHub](https://github.com/deckhouse/deckhouse/blob/main/modules/160-multitenancy-manager/images/multitenancy-manager/src/templates/secure-with-dedicated-nodes.yaml).

To list all available parameters for a project template, execute the command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

## Creating a project

1. To create a project, create the [Project](cr.html#project) resource by specifying the name of the project template in [.spec.projectTemplateName](cr.html#project-v1alpha3-spec-projecttemplatename) field.
1. Set the [standard fields](#standard-project-fields) — [.spec.administrators](cr.html#project-v1alpha3-spec-administrators) and [.spec.quota](cr.html#project-v1alpha3-spec-quota) — that are now managed directly by the Project resource regardless of the template.
1. In the [.spec.parameters](cr.html#project-v1alpha3-spec-parameters) field of the Project resource, specify the parameter values suitable for the ProjectTemplate [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha1-spec-parametersschema-openapiv3schema).

   Example of creating a project using the [Project](cr.html#project) resource from the `default` [ProjectTemplate](cr.html#projecttemplate):

   ```yaml
   apiVersion: deckhouse.io/v1alpha3
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     # Standard fields, managed by the Project resource itself.
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

   {% endraw %}

   {% alert level="info" %}
   The Project API is served as `deckhouse.io/v1alpha3`. Older `v1alpha1`/`v1alpha2` manifests keep working: a conversion webhook lifts `parameters.administrators` and `parameters.resourceQuota` into the `.spec.administrators` and `.spec.quota` standard fields automatically.
   {% endalert %}

   {% raw %}

1. To check the status of the project, execute the command:

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

## Standard project fields

Project administrators and resource quotas are no longer part of the project template parameters — they are first-class fields of the [Project](cr.html#project) resource and work with any template (including `simple` and template-less projects):

- `.spec.administrators` — a list of subjects (`kind: User` or `kind: Group` and `name`) that receive administrative access to the project. The controller manages this access as an auto-generated [ProjectRoleBinding](cr.html#projectrolebinding) in the project namespace.
- `.spec.quota` — a map of [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) hard limits (for example, `requests.cpu`, `limits.memory`). The controller maintains a `ResourceQuota` in the project namespace and reports current usage in `.status.usage`.

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-project
spec:
  projectTemplateName: simple
  administrators:
    - kind: Group
      name: k8s-admins
  quota:
    requests.cpu: "5"
    requests.memory: 5Gi
    limits.cpu: "10"
    limits.memory: 10Gi
```

{% alert level="warning" %}
`ResourceQuota` and `AuthorizationRule` objects defined inside project templates are no longer rendered: such resources are now managed exclusively through `.spec.quota` and `.spec.administrators`. Existing templates that still declare them keep working, but those objects are filtered out during rendering.
{% endalert %}

## Granting access within a project

To grant access to project namespaces beyond the project administrators, use role bindings that reference cluster-wide roles and fan out into the appropriate project namespaces automatically:

- [ProjectRoleBinding](cr.html#projectrolebinding) (namespaced, short name `prb`) — grants a role within a **single** project. It must be created in the project's main namespace (the namespace whose name equals the project name). The controller creates a `RoleBinding` in every namespace of that project.
- [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) (cluster-scoped, short name `cprb`) — grants a role across **all** non-virtual projects. The controller creates a `RoleBinding` in every namespace of every project and reports the number of bound projects in `.status.boundProjects`.

`roleRef` must reference a `ClusterRole` whose name starts with one of the allowed prefixes (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). A privilege-escalation check (via `SubjectAccessReview`) ensures the requesting user is allowed to bind the role.

```yaml
---
apiVersion: deckhouse.io/v1alpha3
kind: ProjectRoleBinding
metadata:
  name: viewers
  namespace: my-project
spec:
  subjects:
    - kind: User
      name: viewer@example.com
  roleRef:
    kind: ClusterRole
    name: d8:project:viewer
---
apiVersion: deckhouse.io/v1alpha3
kind: ClusterProjectRoleBinding
metadata:
  name: platform-viewers
spec:
  subjects:
    - kind: Group
      name: platform
  roleRef:
    kind: ClusterRole
    name: d8:project:viewer
```

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

{% endraw %}

## Using labels to manage resources

When creating resources in `ProjectTemplate`, you can use special labels to control how the `multitenancy-manager` processes these resources.

### Skipping creation of the `heritage: multitenancy-manager` label

By default, all resources created from `ProjectTemplate` receive the label `heritage: multitenancy-manager`.  
This label prohibits changes to resources by users or any other controller except `multitenancy-manager`.  
If you need to allow resource modification (for example, for compatibility with other systems, or if implementing your own control over the created objects), add the label `projects.deckhouse.io/skip-heritage-label` to the resource.

Example:

{% raw %}

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

{% endraw %}

In this case, the resource will receive the labels `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template`, but will not receive the label `heritage: multitenancy-manager`.

### Excluding resources from management by multitenancy-manager

If you need to exclude a resource from management by `multitenancy-manager` (for example, if the resource should be managed manually or by another controller), add the label `projects.deckhouse.io/unmanaged` to the resource.

Example:

{% raw %}

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

{% endraw %}

Resources with the label `projects.deckhouse.io/unmanaged`:

- Will be created **only once** when the project is created;
- **Will not be updated** with subsequent template changes or updates;
- Will not be monitored in the project's status;
- Will receive the labels `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` but **will not receive** the label `heritage: multitenancy-manager`.

{% alert level="warning" %}
Once a resource is marked as `unmanaged`, it will be created on initial installation but not updated when the `ProjectTemplate` is changed.  
After creation, the resource becomes fully independent and must be managed manually.
{% endalert %}

## Implementing validation of object changes with a custom label

The `multitenancy-manager` module uses `ValidatingAdmissionPolicy` to protect resources labeled `heritage: multitenancy-manager` from manual changes.  
You can implement similar validation for resources with any label.

### How validation works in multitenancy-manager

Validation occurs for objects labeled `heritage: multitenancy-manager`.  
The following components are used for this:

1. `ValidatingAdmissionPolicy`: Defines validation rules:
   - Operations: `UPDATE` and `DELETE`.
   - Check: only operations on behalf of the controller's service account are allowed.
   - Applies to all resources and API groups.
1. `ValidatingAdmissionPolicyBinding`: Defines which objects the validation applies to:
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

- `GrantableClusterResourceDefinition` (cluster-scoped) — registers a cluster resource that can be
  granted to projects: which resource it is (`grantedResource`), where references to it are validated (`usageReferences`),
  the baseline availability (`defaultAvailability`), and how the per-project default is discovered
  (`defaultFrom`). Each reference opts into defaulting individually with `default: true` — set it only
  for a field whose value the resource always needs (such as a `PersistentVolumeClaim`'s
  `storageClassName`). Leave it off for a reference whose absence is meaningful, such as an annotation
  that merely toggles a feature; that reference is still validated and counted, just never filled in.
- `ClusterResourceGrantPolicy` (cluster-scoped) — selects projects (by namespace labels via
  `projectSelector`) and, per resource (`resourceName`), the granted names (`allowed`,
  `allowedSelector`) and the per-project `default`. An allow-list restricts the resource to it.
- `AvailableClusterResource` (namespaced, read-only, short name `available`) — the controller-rendered
  catalog of what a project may use; tenants read it to discover the available names.
- `ClusterResourceGrant` (namespaced) — the per-project object-quota pool (limits on object count and
  on measured quantities such as requested storage); its status reports current usage.

{% raw %}

```yaml
---
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
