---
title: Project management
permalink: en/admin/multitenancy/project-management.html
description: Project management
---

Deckhouse Platform includes a set of templates for creating projects. They are cumulative: each one includes the capabilities of the previous one and adds its own. Parameter values are set in the `.spec.parameters` field of a Project:

- `simple` is a minimal template that creates only the project namespace. Use it when you only need an isolated namespace managed as a project, and configure access and quotas through the [standard project fields](#standard-project-fields) and [project role bindings](#granting-access-within-a-project).

  Parameters:
  - `namespace.labels` and `namespace.annotations` — extra labels and annotations for the project namespace.
  - `requiredRequests` (default `false`) — when true, workloads in the project must specify CPU and memory requests (a Deny-mode OperationPolicy). This template creates the namespace and nothing else, so it is off by default.

- `default` is a template for basic project use cases. On top of the namespace, it sets up network isolation, a pod security profile, extended monitoring, and log shipping.

  Parameters (in addition to the ones for `simple`):
  - `networkPolicy` — `Isolated` (default) denies all traffic except traffic within the project namespaces, DNS, Prometheus metrics scraping, and ingress-nginx; `NotRestricted` allows all traffic.
  - `podSecurityProfile` — the [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) profile for the project namespaces: `Baseline` (default) prevents known privilege escalations, `Restricted` applies the strictest hardening practices, `Privileged` restricts nothing.
  - `extendedMonitoringEnabled` (default `true`) — alerts on controller outages and restarts, 5xx errors in ingress-nginx, and low free space on the project's persistent volumes.
  - `clusterLogDestinationName` — the name of the ClusterLogDestination to ship the project logs to. Left unset, the project logs are not shipped anywhere.
  - `requiredRequests` (default `true` in this template) — when true, workloads in the project must specify CPU and memory requests (a Deny-mode OperationPolicy). Adopting an existing namespace seeds this to `false` so running workloads are not blocked.

- `secure` includes all the capabilities of the `default` template, and additionally restricts the users and groups inside containers, audits their calls to the kernel, and scans images for vulnerabilities.

  Parameters (in addition to the ones for `default`):
  - `allowedUIDs` and `allowedGIDs` — the ranges (`min`, `max`) of IDs permitted for users and groups inside the project containers. See [Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
  - `runtimeAuditEnabled` (default `false`) — audit rules that catch calls to the kernel and find malicious activity. They work only if the UID/GID ranges are set.
  - `securityScanningEnabled` (default `true`) — periodic scanning of the launched images for known vulnerabilities (CVE) with Trivy, every 24 hours.

- `secure-with-dedicated-nodes` includes all the capabilities of the `secure` template, and additionally places the project on dedicated nodes.

  Parameters (in addition to the ones for `secure`), at least one of the two must be set:
  - `dedicatedNodes.nodeSelector` — the node selector of the project. The node selector of a created pod is automatically **replaced** with this value.
  - `dedicatedNodes.defaultTolerations` — tolerations in the format of the pod's `spec.tolerations`. They are automatically **added** to the created pods of the project.

The `default`, `secure`, and `secure-with-dedicated-nodes` templates are described in [structured form](#structured-templates) (`deckhouse.io/v1alpha2`); the `simple` template is a minimal structured template that creates only the namespace and sets its labels and annotations from the project parameters.

For the exact set of parameters, check the template installed in your cluster — it matches your platform version.

To list all available parameters for a project template, run:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

To view the whole template, run:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o yaml
```

## Project creation

1. Create a custom resource [Project](/modules/multitenancy-manager/cr.html#project) with the project template name specified in the [.spec.projectTemplateName](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-projecttemplatename) field.
1. Set the standard fields — [.spec.administrators](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) and [.spec.quota](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) — which are managed directly by the Project resource regardless of the template.
1. In the [.spec.parameters](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-parameters) field, specify the values for the [.spec.parametersSchema.openAPIV3Schema](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) section of the custom resource [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate).

   An example of creating a project using [Project](/modules/multitenancy-manager/cr.html#project) from the `default` [ProjectTemplate](/modules/multitenancy-manager/cr.html#projecttemplate) is shown below:

   ```yaml
   apiVersion: deckhouse.io/v1alpha3
   kind: Project
   metadata:
     name: my-project
   spec:
     description: This is an example from the Deckhouse documentation.
     projectTemplateName: default
     # Standard fields, managed by the Project resource itself, independently of the template.
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

   {% alert level="info" %}
   The Project API is served as `deckhouse.io/v1alpha3`. `v1alpha2` manifests keep working: a conversion webhook automatically lifts `parameters.administrators` and `parameters.resourceQuota` into the `.spec.administrators` and `.spec.quota` standard fields. The `deckhouse.io/v1alpha1` version is no longer served.
   {% endalert %}

1. To check the project status, run the following command:

   ```shell
   d8 k get projects my-project
   ```

   A successfully created project should have the `Deployed` (synced) status. If the status is `Error`, add the `-o yaml` flag to the command (for example, `d8 k get projects my-project -o yaml`) to get more details about the error cause.

### Automatic project creation for a namespace

A namespace created directly (for example, `d8 k create ns test`) automatically becomes a project with the same name — no annotation is required:

- the template is picked from what the namespace already carries: `secure` if it has the `security-scanning.deckhouse.io/enabled` label, `default` if it has `security.deckhouse.io/pod-policy` or `extended-monitoring.deckhouse.io/enabled`, and `simple` otherwise;
- the project parameters are filled in from the current state of the namespace, so nothing inside it changes;
- from then on the project is the source of truth: deleting the namespace no longer deletes the project — the project recreates the namespace.

System namespaces (`d8-*`, `kube-*`, `upmeter-*`, `default`, and anything labeled `heritage: deckhouse` or `heritage: upmeter`) are never adopted this way: they belong to the virtual `deckhouse`/`default` projects (see [Virtual projects](#virtual-projects)).

For example:

1. Create a new namespace:

   ```shell
   d8 k create ns test
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
   test        Deployed   simple                                                                    1m
   ```

You can change the template of an existing project to another available template.

{% alert level="warning" %}
Note that changing the template may cause resource conflicts: if the template chart defines resources that already exist in the namespace, the template cannot be applied.
{% endalert %}

### Creating a project without specifying a template

The `projectTemplateName` field is optional: when omitted, the `simple` template is used. Such a project consists only of the namespace and the [standard fields](#standard-project-fields) (administrators, quota) — no template policies are created in it. This is convenient when no settings are needed or they are managed by other means:

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

The `.status.state` field of a project is either `Deployed` (all project resources are in sync) or `Error`. The cause of an error is described in the conditions (`.status.conditions`):

```shell
d8 k get project my-project -o jsonpath='{range .status.conditions[*]}{.type}={.status}: {.message}{"\n"}{end}'
```

| Condition | `False` means |
|-----------|---------------|
| `ProjectTemplateFound` | The template referenced in `.spec.projectTemplateName` was not found. |
| `Validated` | The project parameters failed validation against the template schema (`parametersSchema`). |
| `ResourcesUpgraded` | The project resources could not be created or updated from the template (details in `message`). |
| `StandardFieldsApplied` | The [standard fields](#standard-project-fields) (quota or administrators) could not be applied. |
| `TemplateRolesAllowed` | The template creates a binding to a role [forbidden for granting in projects](#granting-access-within-a-project) — the project switches to `Error`, the role is named in `message`. |
| `TemplateResourcesFiltered` | ResourceQuota/AuthorizationRule objects were dropped from the template (see [standard fields](#standard-project-fields)). Informational — the project keeps working. |

Other useful status fields:

- `.status.namespaces` — all namespaces of the project with their kind (`Main`/`Additional`);
- `.status.usage` — the current quota usage (populated when `.spec.quota` is set);
- `.status.resources` — the state of the individual resources created from the template.

### Service objects of a project

The controller creates service objects in the project namespaces. They are managed automatically — editing them manually is not possible (the attempt is rejected):

| Object | Where | Comes from |
|--------|-------|------------|
| `ResourceQuota/d8-project-quota` | The main namespace | The [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) field of the project. |
| `ProjectRoleBinding/d8-administrators` | The main namespace | The [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) field of the project. |
| `RoleBinding/d8:prb:<name>` | Every namespace of the project | The fan-out of the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) named `<name>`. |
| `RoleBinding/d8:cprb:<name>` | Every namespace of every project | The fan-out of the [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) named `<name>`. |

When the source object (a binding, the quota field, etc.) is removed, the corresponding service objects are removed automatically.

## Virtual projects

Besides the user-created projects, the `d8 k get projects` list always contains two **virtual** projects (labeled `projects.deckhouse.io/virtual-project: "true"`):

- `deckhouse` — groups system namespaces (`d8-*`, `kube-*`, `upmeter-*`, `heritage: deckhouse` / `heritage: upmeter`);
- `default` — groups remaining namespaces that do not belong to any project (the `default` namespace itself).

Virtual-project status is rebuilt from the live namespace list: a deleted namespace disappears from that list. Virtual projects do not recreate namespaces.

Virtual projects exist for completeness: with them, every namespace of the cluster belongs to some project. They cannot be managed: they are not editable, [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace) and [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) resources cannot be created in them, and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) does not extend to them.

## Additional project namespaces

If an application needs several namespaces (for example, a separate one for a cache or a queue), add them to the project with the [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace) resource. It is created **in the main namespace of the project**; the resulting namespace is named `<project name>-<spec.name>`:

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

The rules for working with ProjectNamespace:

- The `spec.name` field is immutable: to rename a namespace, delete the resource and create a new one.
- The resulting name `<project name>-<spec.name>` cannot be longer than 63 characters (the Kubernetes limit on namespace names).
- A ProjectNamespace can only be created in the main namespace of a project — it cannot be "nested" into an additional namespace or a foreign project. If a namespace with that name already exists and belongs to another project, the request is rejected.
- Deleting a ProjectNamespace resource deletes its namespace. Deleting the project deletes all of its namespaces.

### What applies to the additional namespaces

The following automatically applies in **all** namespaces of the project (the main and the additional ones alike):

- **Access**: the [ProjectRoleBinding](#granting-access-within-a-project) and [ClusterProjectRoleBinding](#granting-access-within-a-project) bindings, including the automatic access of the project administrators. When a new namespace is added, all existing bindings fan out into it without any user action.
- **Namespaced template objects**: the network policy (`networkPolicy.mode: Isolated`) and the log collection setup (`logShipping`) are created in every namespace of the project. The network isolation allows traffic between the namespaces of one project.
- **Cluster-scoped template policies** (`OperationPolicy`, the `SecurityPolicy` from `allowedUIDs`/`allowedGIDs`): they select namespaces by the `projects.deckhouse.io/project` label, that is, they cover the whole project.
- **Inherited labels**: the pod security profile (`security.deckhouse.io/pod-policy`), extended monitoring (`extended-monitoring.deckhouse.io/enabled`), vulnerability scanning (`security-scanning.deckhouse.io/enabled`), and the template label (`projects.deckhouse.io/project-template`) are synced from the main namespace to the additional ones. The sync is complete: if a feature is turned off in the template, the label is removed from the additional namespaces as well. Thanks to the template label, the [cluster resource availability rules](#managing-access-to-cluster-wide-resources) also apply in all namespaces of the project.

The following stays in the **main** namespace only:

- the project quota (the `ResourceQuota` from [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota));
- the extra labels and annotations from the template's `namespaceMetadata`;
- the node placement annotations (from the template's `nodeSelector` and `tolerations` fields).

### Labels of the project namespaces

| Label | Main | Additional | Purpose |
|-------|:----:|:----------:|---------|
| `projects.deckhouse.io/project: <project name>` | ✓ | ✓ | Project ownership — the common label of all namespaces of the project. |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Marks an additional namespace (the name of the ProjectNamespace resource). |
| `projects.deckhouse.io/project-template: <template name>` | ✓ | ✓ | The project template; the cluster resource availability rules match by it. |
| `heritage: multitenancy-manager` | ✓ | ✓ | The namespace is managed by the project controller: its `spec`, finalizers and the labels listed in this table are changed through the Project; other labels and annotations may be changed directly. |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (inherited) | Policies and features from the project template. |

The common `projects.deckhouse.io/project` label makes it possible to select the project namespaces with a plain `get ns`.

To get all namespaces of the project (main + additional):

```shell
d8 k get ns -l projects.deckhouse.io/project=my-project
```

To get additional namespaces only:

```shell
d8 k get ns -l 'projects.deckhouse.io/project=my-project,projects.deckhouse.io/project-namespace'
```

To get the main namespace only:

```shell
d8 k get ns -l 'projects.deckhouse.io/project=my-project,!projects.deckhouse.io/project-namespace'
```

## Standard project fields

Project administrators and resource quotas are not project template parameters — they are top-level fields of the [Project](/modules/multitenancy-manager/cr.html#project) resource and work with any template (including `simple`):

- [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) — a list of subjects (`kind: User` or `kind: Group` and `name`) that receive administrative access to the project. The controller implements this access through an auto-generated [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) named `d8-administrators` in the project's main namespace.
- [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) — a set of hard [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) limits (for example, `requests.cpu`, `limits.memory`). The controller maintains a `ResourceQuota` in the project's main namespace and reports current usage in `.status.usage`. For `memory` and `storage`, a unit is required (for example, `2Gi`) — numbers without a unit mean bytes and are rejected.

{% alert level="warning" %}
`ResourceQuota` and `AuthorizationRule` objects declared inside project templates are no longer rendered: these resources are now managed exclusively through `.spec.quota` and `.spec.administrators`. Existing templates that still declare them keep working, but those objects are filtered out during rendering.
{% endalert %}

## Granting access within a project

To grant access to project namespaces for users beyond the project administrators, use role bindings that reference cluster-wide roles and fan out into the appropriate project namespaces automatically:

- [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) (namespaced, short name `prb`) — grants a role within a **single** project. It must be created in the project's main namespace. The controller creates a RoleBinding in every namespace of that project.
- [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) (cluster-scoped, short name `cprb`) — grants a role across **all** non-virtual projects.

`roleRef` must reference a `ClusterRole` whose name starts with one of the allowed prefixes (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). See [the user-authz module documentation](/modules/user-authz/) for the description of the roles.

The following checks apply when a binding is created:

- **Privilege escalation protection**: a binding can only be created by a user who has the right to bind (`bind`) the referenced role; the right is checked by the role's name. A project administrator (`d8:project:admin`) has `bind` on exactly eight roles: `d8:project:viewer`, `d8:project:user`, `d8:project:manager`, `d8:project:admin`, `d8:namespace:viewer`, `d8:namespace:user`, `d8:namespace:manager`, and `d8:namespace:admin`. A binding to any other role — a custom `d8:custom:*` role, a capability role, or `d8:project:superadmin` — is rejected for a project administrator, even if it is within their own permissions, because nobody granted them `bind` on that name. Such bindings are created by a cluster administrator, or the cluster administrator grants `bind` on that role to project administrators separately via a dedicated ClusterRole.
- The role must exist: a binding to a non-existent role is rejected.
- A ServiceAccount used as a ProjectRoleBinding subject must belong to a namespace of that same project.
- System and subsystem roles (`d8:system:*`, `d8:subsystem:*`), as well as arbitrary roles outside the listed prefixes, cannot be granted through project bindings.
- Roles labeled `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` are forbidden for granting in projects. A cluster administrator can set this label on a role to stop new grants of it: existing bindings keep working, but no new ones are created. If a project template uses such a role, the project switches to `Error` with the reason in the `TemplateRolesAllowed` condition.

The `d8-administrators` binding, created by the controller from the [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) field, is managed by the controller only — it cannot be edited manually. To change the set of administrators, change the project's `.spec.administrators` field instead.

### Roles available in a RoleBinding inside a project

Besides the project bindings, a plain RoleBinding can also be used inside a project namespace — the role then applies in that single namespace only. However, in projects the set of roles available to a plain RoleBinding is restricted: only cluster roles carrying the `rbac.deckhouse.io/delegatable: "true"` label are allowed. Among the built-in ones these are the `d8:namespace:*` and `d8:project:*` roles, as well as the access-level roles of the basic role model (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

A RoleBinding to any other cluster role (for example, `cluster-admin`, system roles, or capabilities) is rejected in a project with the message `references "<role>" which is not available to project`. This protects the project isolation from being bypassed by binding to an overly broad role.

To use a [custom role](/modules/user-authz/faq.html#creating-a-custom-namespace-or-project-role) in projects, add the `rbac.deckhouse.io/delegatable: "true"` label to it:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

The restriction applies in the namespaces of every project, including the ones adopted from a standalone namespace. Examples:

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

Available fields (all optional; the complete reference is [in the ProjectTemplate resource description](/modules/multitenancy-manager/cr.html#projecttemplate)):

| Field | What it configures |
|-------|--------------------|
| `podSecurityStandard` | Pod security profile: `Privileged`, `Baseline`, or `Restricted`. |
| `networkPolicy.mode` | Network isolation: `Isolated` (traffic is only allowed within the project and from the platform system components) or `NotRestricted`. |
| `features.monitoring` | Extended monitoring of the project namespaces. |
| `features.vulnerabilityScanning` | Scanning of container images for vulnerabilities. |
| `logShipping.clusterDestinationRef` | Collecting the logs of the project pods into the given destination (`ClusterLogDestination`). |
| `nodeSelector`, `tolerations` | Placing the project pods on dedicated nodes. |
| `allowedUIDs`, `allowedGIDs` | The allowed UID/GID ranges of the project containers. |
| `runtimeAudit.enabled` | Auditing the project processes' access to the Linux kernel. |
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Extra labels and annotations of the project namespaces. |
| `resources`, `grantPolicies` | Granting cluster-scoped resources through a project template — see [Managing access to cluster-wide resources](#managing-access-to-cluster-wide-resources). |
| `parametersSchema.openAPIV3Schema` | The schema of parameters set when creating a project. |

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

Twelve fields of the template can be turned into a parameter: `podSecurityStandard`, `networkPolicy.mode`, `features.monitoring`, `features.vulnerabilityScanning`, `logShipping.clusterDestinationRef`, `nodeSelector`, `tolerations`, `allowedUIDs`, `allowedGIDs`, `runtimeAudit.enabled`, `namespaceMetadata.labels` and `namespaceMetadata.annotations`. Instead of a concrete value, specify `{fromParam: <parameter name>}` and declare the parameter in `parametersSchema`. The value does not have to be a scalar: a map (`nodeSelector`, `namespaceMetadata.labels`), a list (`tolerations`) or an object (`allowedUIDs`) works just as well. The other fields — `title`, `description`, `resources`, `grantPolicies` and `parametersSchema` itself — take literal values only. Each project then sets its own value in `.spec.parameters`; if the value is not set, the `default` from the schema is used.

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

The following rules apply to template operations:

- A template used by at least one project cannot be deleted.
- A change to a template is automatically applied to all projects created from it.
- The `deckhouse.io/v1alpha1` version of ProjectTemplate with the text `resourcesTemplate` field (Helm templating) is no longer served, and `v1alpha2` has no such field. A template that was stored as `v1alpha1` with a non-empty `resourcesTemplate` comes up in `v1alpha2` without the Helm text and with the `projects.deckhouse.io/legacy-helm-template: "true"` annotation:
  - the controller does not render the projects of such a template. They switch to the `Error` state with the `ProjectTemplateUsable` condition set to `False`, and their objects stay exactly as they were;
  - the Helm text is kept in the `projects.deckhouse.io/legacy-helm-template-body` annotation of the template, so you can read what it used to render. A text over 64 KiB is not kept: all annotations of an object together may not exceed 256 KiB;
  - to bring the projects back, rewrite the template with structured fields and remove the mark annotation in the same request — `d8 k edit` and `d8 k apply` do that. Removing the mark on its own is refused, because the template would then render a bare namespace and Helm would delete every object the Helm text used to produce.

## Creating a custom project template

The default project templates cover common baseline scenarios and also serve as examples of what templates can do.

To create your own template:

1. Use one of the default templates as a starting point, for example, `default`.
1. Export it to a separate file, for example, `my-project-template.yaml`, using the following command:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Edit the `my-project-template.yaml` file: adjust the [structured fields](#structured-templates) and the input parameters schema to your needs.

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

## Managing access to cluster-wide resources

The `multitenancy-manager` module allows cluster administrators to define, for each project, which cluster-wide resources (such as StorageClass, ClusterIssuer, ClusterRole, and LoadBalancerClass) can be used from project namespaces and which values are used by default.

This mechanism works independently of RBAC. RBAC determines *who can create and modify* objects, while the cluster-wide resource access mechanism determines *which resources* those objects can use.

The mechanism uses four custom resources:

- [GrantableClusterResourceDefinition](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition) registers a type of cluster-wide resource whose access can be managed. These resources are provided by DP or module developers.
- [GrantableClusterResourceReference](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference) defines where a registered cluster-wide resource is used, for example, which resource field contains a reference to it. These resources are provided by modules.
- [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) defines access rules. Using labels, a cluster administrator selects the projects to which the policy applies and defines the allowed and denied resources, as well as the resource used by default.
- [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) is a read-only list of cluster-wide resources available to the project, created by the controller.

Until the administrator creates a ClusterResourceGrantPolicy, resource availability is determined by their registration: resources are available to all projects if `defaultAvailability: All` (the default value) is set in GrantableClusterResourceDefinition and the resource does not match the `excluded` filters.

Access checks are performed only for objects in project namespaces. If an access policy changes, cluster-wide resources already used by existing objects remain available to those objects.

For a detailed description of the access management mechanism, refer to the [`multitenancy-manager`](/modules/multitenancy-manager/#managing-access-to-cluster-wide-resources) module documentation.
