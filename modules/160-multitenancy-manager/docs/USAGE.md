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

The `default`, `secure`, and `secure-with-dedicated-nodes` templates are described in the [structured form](#structured-templates) (`deckhouse.io/v1alpha2`); the `simple` template is a minimal legacy (`v1alpha1`) template.

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

### A project without a template

The `projectTemplateName` field is optional. A project without a template consists only of the namespace and the [standard fields](#standard-project-fields) (administrators, quota) — no template policies are created in it. This is convenient when no settings are needed or they are managed by other means:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-plain-project
spec:
  administrators:
    - kind: Group
      name: k8s-admins
  quota:
    requests.cpu: "2"
```

A template can be assigned later by setting it in `.spec.projectTemplateName`.

### Project naming rules

The project name is also the name of its main namespace, so the following rules are checked when a project is created:

- the name cannot start with `d8-` or `kube-` — these prefixes are reserved for system namespaces;
- the name cannot be longer than 61 characters;
- if a project `foo` exists, a project `foo-bar` cannot be created — and vice versa, with an existing project `foo-bar` a project `foo` cannot be created. Names like `<project>-*` are reserved for the project's [additional namespaces](#additional-project-namespaces): without this rule, an additional namespace of one project could clash with another project's name.

## Project status and diagnostics

The `.status.state` field of a project is either `Deployed` (all project resources are in sync) or `Error`. The cause of an error is detailed in the conditions (`.status.conditions`):

```shell
d8 k get project my-project -o jsonpath='{range .status.conditions[*]}{.type}={.status}: {.message}{"\n"}{end}'
```

| Condition | `False` means |
|-----------|---------------|
| `ProjectTemplateFound` | The template referenced in `.spec.projectTemplateName` was not found |
| `Validated` | The project parameters failed validation against the template schema (`parametersSchema`) |
| `ResourcesUpgraded` | The project resources could not be created or updated from the template (details in `message`) |
| `StandardFieldsApplied` | The [standard fields](#standard-project-fields) (quota or administrators) could not be applied |
| `TemplateRolesAllowed` | The template creates a binding to a role [forbidden for granting in projects](#granting-access-within-a-project) — the project switches to `Error`, the role is named in `message` |
| `TemplateResourcesFiltered` | `ResourceQuota`/`AuthorizationRule` objects were dropped from the template (see [standard fields](#standard-project-fields)). Informational — the project keeps working |

Other useful status fields:

- `.status.namespaces` — all namespaces of the project with their kind (`Main`/`Additional`);
- `.status.usage` — the current quota usage (populated when `.spec.quota` is set);
- `.status.resources` — the state of the individual resources created from the template.

### Service objects of a project

The controller creates service objects in the project namespaces. They are managed automatically — editing them manually is not possible (the attempt is rejected):

| Object | Where | Comes from |
|--------|-------|------------|
| `ResourceQuota/d8-project-quota` | The main namespace | The [`.spec.quota`](cr.html#project-v1alpha3-spec-quota) field of the project |
| `ProjectRoleBinding/d8-administrators` | The main namespace | The [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) field of the project |
| `RoleBinding/d8:prb:<name>` | Every namespace of the project | The fan-out of the [ProjectRoleBinding](cr.html#projectrolebinding) named `<name>` |
| `RoleBinding/d8:cprb:<name>` | Every namespace of every project | The fan-out of the [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) named `<name>` |

When the source object (a binding, the quota field, etc.) is removed, the corresponding service objects are removed automatically.

## Virtual projects

Besides the user-created projects, the `d8 k get projects` list always contains two **virtual** projects (labelled `projects.deckhouse.io/virtual-project: "true"`):

- `deckhouse` — groups the system namespaces (with the `d8-` and `kube-` prefixes);
- `default` — groups all other namespaces that do not belong to any project.

Virtual projects exist for completeness: with them, every namespace of the cluster belongs to some project. They cannot be managed: they are not editable, [ProjectNamespace](cr.html#projectnamespace) and [ProjectRoleBinding](cr.html#projectrolebinding) resources cannot be created in them, and [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) does not extend to them.

## Additional project namespaces

If an application needs several namespaces (for example, a separate one for a cache or queues), add them to the project with the [ProjectNamespace](cr.html#projectnamespace) resource. The resource is created **in the main namespace of the project**; the resulting namespace is named `<project name>-<spec.name>`:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: ProjectNamespace
metadata:
  name: cache
  namespace: my-project
spec:
  name: cache   # The my-project-cache namespace will be created.
```

You can check the project composition in its status:

```shell
d8 k get project my-project -o jsonpath='{.status.namespaces}'
```

The rules for working with `ProjectNamespace`:

- The `spec.name` field is immutable: to rename a namespace, delete the resource and create a new one.
- The resulting name `<project name>-<spec.name>` cannot be longer than 63 characters (the Kubernetes limit on namespace names).
- A `ProjectNamespace` can only be created in the main namespace of a project — it cannot be "nested" into an additional namespace or a foreign project. If a namespace with that name already exists and belongs to another project, the request is rejected.
- Deleting a `ProjectNamespace` resource deletes its namespace; deleting the project deletes all of its namespaces.

### What applies to the additional namespaces

The following automatically applies in **all** namespaces of the project (the main and the additional ones alike):

- **Access**: the [ProjectRoleBinding](cr.html#projectrolebinding) and [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) bindings, including the automatic access of the project administrators. When a new namespace is added, all existing bindings fan out into it without any user action.
- **Namespaced template objects**: the network policy (`networkPolicy.mode: Isolated`) and the log collection setup (`logShipping`) are created in every namespace of the project. The network isolation allows traffic between the namespaces of one project.
- **Cluster-scoped template policies** (`OperationPolicy`, the `SecurityPolicy` from `allowedUIDs`/`allowedGIDs`): they select namespaces by the `projects.deckhouse.io/project` label, that is, they cover the whole project.
- **Inherited labels**: the pod security profile (`security.deckhouse.io/pod-policy`), extended monitoring (`extended-monitoring.deckhouse.io/enabled`), vulnerability scanning (`security-scanning.deckhouse.io/enabled`), and the template label (`projects.deckhouse.io/project-template`) are synced from the main namespace to the additional ones. The sync is complete: if a feature is turned off in the template, the label is removed from the additional namespaces as well. Thanks to the template label, the [cluster resource availability rules](#granting-cluster-scoped-resources-to-projects) also apply in all namespaces of the project.

The following stays in the **main** namespace only:

- the project quota (the `ResourceQuota` from [`.spec.quota`](cr.html#project-v1alpha3-spec-quota));
- the extra labels and annotations from the template's `namespaceMetadata`;
- the node placement annotations (from the template's `nodeSelector` and `tolerations` fields).

### Labels of the project namespaces

| Label | Main | Additional | Purpose |
|-------|:----:|:----------:|---------|
| `projects.deckhouse.io/project: <project name>` | ✓ | ✓ | Project ownership — the common label of all namespaces of the project |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Marks an additional namespace (the name of the `ProjectNamespace` resource) |
| `projects.deckhouse.io/project-template: <template name>` | ✓ | ✓ | The project template; the cluster resource availability rules match by it |
| `heritage: multitenancy-manager` | ✓ | ✓ | The namespace is managed by the project controller; it cannot be modified manually |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (inherited) | Policies and features from the project template |

The common `projects.deckhouse.io/project` label makes it possible to select the project namespaces with a plain `get ns`:

```shell
# All namespaces of the project (main + additional):
d8 k get ns -l projects.deckhouse.io/project=my-project

# Additional only:
d8 k get ns -l 'projects.deckhouse.io/project=my-project,projects.deckhouse.io/project-namespace'

# Main only:
d8 k get ns -l 'projects.deckhouse.io/project=my-project,!projects.deckhouse.io/project-namespace'
```

## Creating a project automatically for a namespace

By default (the [`allowNamespacesWithoutProjects: true`](configuration.html#parameters-allownamespaceswithoutprojects) parameter), a namespace created directly (for example, `d8 k create ns my-app`) is automatically wrapped into a project with the same name:

- the project is created without a template and is labelled `multitenancy.deckhouse.io/project-managed-by-namespace: "true"`;
- the namespace is the source of truth: its labels and annotations are synced into the project parameters; edit and delete the namespace itself (deleting it deletes the project automatically);
- the specification of such a project cannot be edited manually. To turn it into a regular project (for example, to assign a template), remove the `multitenancy.deckhouse.io/project-managed-by-namespace` label from the project — after that, the project is managed as usual.

If the `allowNamespacesWithoutProjects` parameter is disabled, creating namespaces outside of projects is prohibited — a `d8 k create ns` attempt is rejected with an explanation.

An existing namespace can also be explicitly adopted into a project by adding the `projects.deckhouse.io/adopt` annotation. For example:

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

`roleRef` must reference a `ClusterRole` whose name starts with one of the allowed prefixes (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). See [the user-authz module documentation](../user-authz/) for the description of the roles.

The following checks apply when bindings are created:

- **Privilege escalation protection**: a binding can only be created by a user who has the right to bind (`bind`) the referenced role. For example, a project administrator (`d8:project:admin`) can grant the built-in `d8:project:*` and `d8:namespace:*` roles, but cannot grant a role broader than their own permissions.
- The role must exist: a binding to a non-existent role is rejected.
- A `ServiceAccount` used as a subject of a `ProjectRoleBinding` must belong to a namespace of that same project.
- System and subsystem roles (`d8:system:*`, `d8:subsystem:*`), as well as arbitrary roles outside the listed prefixes, cannot be granted via project bindings.
- Roles with the `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` annotation are forbidden for granting in projects. A cluster administrator can put this annotation on a role to phase it out: existing bindings keep working, but new ones cannot be created. If such a role is used by a project template, the project switches to the `Error` state with an explanation in the `TemplateRolesAllowed` condition.

The `d8-administrators` binding created by the controller from the [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) field is managed by the controller only — it cannot be edited manually. To change the set of administrators, change the `.spec.administrators` field of the project.

### Which roles are available in a RoleBinding inside a project

Besides the project bindings, a plain `RoleBinding` can also be used inside a project namespace — the role then applies in that single namespace only. However, in projects the set of roles available to a plain `RoleBinding` is restricted: only cluster roles carrying the `rbac.deckhouse.io/delegatable: "true"` label are allowed. Among the built-in ones these are the `d8:namespace:*` and `d8:project:*` roles, as well as the access-level roles of the legacy role model (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

A `RoleBinding` to any other cluster role (for example, `cluster-admin`, system roles, or capabilities) is rejected in a project with the message `references "<role>" which is not available to project`. This protects the project isolation from being bypassed by binding to an overly broad role.

To use a [custom role](../user-authz/faq.html#creating-a-custom-namespace-or-project-role) in projects, add the `rbac.deckhouse.io/delegatable: "true"` label to it:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

The restriction applies only in the namespaces of "real" projects. It does not apply to [automatically wrapped](#creating-a-project-automatically-for-a-namespace) namespaces (labelled `multitenancy.deckhouse.io/project-managed-by-namespace`).

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

## Structured templates

Starting with the `deckhouse.io/v1alpha2` API version, a project template is described by **structured fields** — instead of a text Helm template, you declaratively specify which settings the project namespaces get. The controller itself creates the corresponding objects (network policies, security policies, log collection settings, etc.) from these fields in every namespace of the project and keeps them up to date.

Available fields (all optional; the complete reference is [in the resource description](cr.html#projecttemplate)):

| Field | What it configures |
|-------|--------------------|
| `podSecurityStandard` | Pod security profile: `Privileged`, `Baseline`, or `Restricted` |
| `networkPolicy.mode` | Network isolation: `Isolated` (traffic is only allowed within the project and from the platform system components) or `NotRestricted` |
| `features.monitoring` | Extended monitoring of the project namespaces |
| `features.vulnerabilityScanning` | Scanning of container images for vulnerabilities |
| `logShipping.clusterDestinationRef` | Collecting the logs of the project pods into the given destination (`ClusterLogDestination`) |
| `nodeSelector`, `tolerations` | Placing the project pods on dedicated nodes |
| `allowedUIDs`, `allowedGIDs` | The allowed UID/GID ranges of the project containers |
| `runtimeAudit.enabled` | Auditing the project processes' access to the Linux kernel |
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Extra labels and annotations of the project namespaces |
| `resources`, `grantPolicies` | [Granting cluster-scoped resources to projects](#granting-cluster-scoped-resources-to-projects) |
| `parametersSchema.openAPIV3Schema` | The schema of parameters set when creating a project |

An example of a structured template:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: my-template
spec:
  title: "Team template"
  description: "Isolated project with monitoring"
  podSecurityStandard: Baseline
  networkPolicy:
    mode: Isolated
  features:
    monitoring: true
    vulnerabilityScanning: true
```

### Template parametrization

Any "leaf" value of a structured field can be turned into a parameter: instead of a concrete value, specify `{fromParam: <parameter name>}` and declare the parameter in `parametersSchema`. Each project then sets its own value in `.spec.parameters`; if the value is not set, the `default` from the schema is used.

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: my-parametrized-template
spec:
  podSecurityStandard:
    fromParam: securityProfile
  networkPolicy:
    mode:
      fromParam: networkMode
  parametersSchema:
    openAPIV3Schema:
      type: object
      properties:
        securityProfile:
          type: string
          enum: [Baseline, Restricted]
          default: Baseline
        networkMode:
          type: string
          enum: [Isolated, NotRestricted]
          default: Isolated
```

A project using such a template:

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-project
spec:
  projectTemplateName: my-parametrized-template
  parameters:
    securityProfile: Restricted
```

The `fromParam` references are validated when the template is created: a reference to an undeclared parameter or to a parameter of an incompatible type (for example, a string parameter for a boolean field) is rejected.

### Template checks

- A template used by at least one project cannot be deleted.
- A change to a template is automatically applied to all projects created from it.
- Legacy `deckhouse.io/v1alpha1` templates with the text `resourcesTemplate` field (Helm templating) keep working but are deprecated — create new templates in the structured form. `ResourceQuota` and `AuthorizationRule` resources from such templates are filtered out during rendering (see [standard project fields](#standard-project-fields)).

## Creating your own project template

To create your own template:

1. Take one of the default templates as a basis, for example, `default`.
1. Copy it to a separate file, for example, `my-project-template.yaml` using the command:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Edit the `my-project-template.yaml` file: adjust the [structured fields](#structured-templates) and the input parameters schema to your needs.
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
  catalog of what a project may use; tenants read it to discover the available names. The catalog
  objects cannot be modified or deleted manually.

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

### Granting cluster-scoped resources through a project template

The availability rules for cluster-scoped resources can be set directly in a [structured template](#structured-templates) — they then automatically apply to all projects created from that template:

- `spec.resources` — the rules "inside" the template: the same format as `resources` in a `ClusterResourceGrantPolicy` (resource name, `allowed`/`allowedSelector`, `default`);
- `spec.grantPolicies` — a list of names of **library** `ClusterResourceGrantPolicy` objects. A library policy describes a reusable set of rules and must not have a `projectSelector` — which projects it applies to is determined by the referencing template. This way, for example, a "corporate StorageClasses" policy can be maintained by one administrator and used by several templates.

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: my-template
spec:
  resources:
    - resourceName: storageclasses
      allowed: ["standard"]
      default: standard
  grantPolicies:
    - corporate-issuers   # A library ClusterResourceGrantPolicy without a projectSelector.
```

For each source, the controller creates a service policy named `template-<template>-<source>` (for `spec.resources` — `template-<template>-inline`); the `inline` name is reserved for library policies. A reference to a non-existent policy or to a policy with a `projectSelector` is rejected when the template is created.
