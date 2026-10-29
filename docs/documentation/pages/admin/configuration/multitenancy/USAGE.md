---
title: Project management
permalink: en/admin/multitenancy/project-management.html
description: Project management
---

Deckhouse Platform includes a set of templates for creating [projects](../../glossary.html#project). They are cumulative: each one includes the capabilities of the previous one and adds its own. Parameter values are set in the `.spec.parameters` field of a Project:

- `simple` is a minimal template that creates only the project namespace. Use it when you only need an isolated namespace managed as a project, while access and restrictions are configured through the [standard project fields](#standard-project-fields) and [project role bindings](#granting-access-within-a-project).

  Parameters:
  - `namespace.labels` and `namespace.annotations` — extra labels and annotations for the project namespace.
  - `requiredRequests` (default `false`) — rendered only if the parameter is declared in the template schema and is `true`. If `true`, the project pods must specify CPU and memory requests (an OperationPolicy in the `Deny` mode).

- `default` is a template for basic project use cases. On top of the namespace, it sets up network isolation, a pod security profile, [extended monitoring](../../glossary.html#extended-monitoring), and log shipping.

  Parameters (in addition to the ones for `simple`):
  - `networkPolicy` — `Isolated` (default) allows incoming traffic only from the project namespaces, Prometheus, the ingress-nginx controller and the service-with-healthchecks agent, and outgoing traffic only to the project namespaces and to DNS in `kube-system`; `NotRestricted` allows all traffic.
  - `podSecurityProfile` — the [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) profile for the project namespaces: `Baseline` (default) prevents known privilege escalations, `Restricted` applies the strictest hardening practices, `Privileged` restricts nothing. An empty string sets no profile: the project puts no `security.deckhouse.io/pod-policy` label on its namespaces, and the [default policy](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) of the `admission-policy-engine` module applies to them.
  - `extendedMonitoringEnabled` (default `true`) — alerts on controller outages and restarts, 5xx errors in ingress-nginx, and low free space on the project's persistent volumes.
  - `clusterLogDestinationName` — the name of the ClusterLogDestination to ship the project logs to. Left unset, the project logs are not shipped anywhere.
  - `requiredRequests` (default `true` in this template) — if `true`, the project pods must specify CPU and memory requests (an OperationPolicy in the `Deny` mode). When an existing namespace is adopted, the parameter is set to `false` so that the workloads already running are not blocked.

- `secure` includes all the capabilities of the `default` template, and additionally restricts the users and groups inside containers, audits their calls to the kernel, and scans images for vulnerabilities.

  Parameters (in addition to the ones for `default`):
  - `allowedUIDs` and `allowedGIDs` — the ranges (`min`, `max`) of IDs permitted for users and groups inside the project containers. See [Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
  - `runtimeAuditEnabled` (default `false`) — audit rules that catch calls to the kernel and find malicious activity. They work only if the UID/GID ranges are set.
  - `securityScanningEnabled` (default `true`) — periodic scanning of the launched images for known vulnerabilities (CVE) with Trivy, every 24 hours.

- `secure-with-dedicated-nodes` includes all the capabilities of the `secure` template, and additionally places the pods of the main namespace of the project on dedicated nodes.

  Parameters (in addition to the ones for `secure`). If `dedicatedNodes` is set, at least one of its two fields must be set. Without `dedicatedNodes`, the pods are not placed on dedicated nodes:
  - `dedicatedNodes.nodeSelector` — the node selector of the main namespace. It is merged into the node selector of every pod there, and a pod whose selector conflicts with it is rejected.
  - `dedicatedNodes.defaultTolerations` — tolerations in the format of the pod's `spec.tolerations`. They are **added** to the pods created in the main namespace.

The `default`, `secure`, and `secure-with-dedicated-nodes` templates are described in [structured form](#structured-templates) (`deckhouse.io/v1alpha2`). The `simple` template is a minimal structured template that creates the namespace, sets its labels and annotations from the project parameters and, with `requiredRequests: true`, creates the OperationPolicy.

For the exact set of parameters, check the template installed in your cluster — it matches your platform version.

To view the parameter schema of a template, run the following command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

To view the whole template, run the following command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o yaml
```

## Project creation

To create a project, follow these steps:

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
   The [Project](../../glossary.html#project) API is served as `deckhouse.io/v1alpha3`. `v1alpha2` manifests keep working: a conversion webhook automatically lifts `parameters.administrators` and `parameters.resourceQuota` into the `.spec.administrators` and `.spec.quota` standard fields. If one administrator does not fit `.spec.administrators` (for example, it has `accessLevel`), the whole list stays in `parameters`, and `.spec.administrators` is not filled. The same holds for a quota with anything other than quantities (for example, `scopes`), for a quota with the number `0` anywhere in it, and for an empty list or quota.
   The `deckhouse.io/v1alpha1` version is no longer served, and the API server refuses a manifest with `apiVersion: deckhouse.io/v1alpha1`. Before applying such a manifest, change `apiVersion` to `deckhouse.io/v1alpha3`, rename `spec.projectTypeName` to `spec.projectTemplateName` (a ProjectTemplate with that name must exist) and `spec.template` to `spec.parameters`, and move `administrators` and `resourceQuota` from the parameters to `.spec.administrators` and `.spec.quota`.
   {% endalert %}

1. To check the project status, run the following command:

   ```shell
   d8 k get projects my-project
   ```

   A successfully created project should have the `Deployed` (synchronized) status. If the `Error` status is displayed, add the `-o yaml` flag to the command (for example, `d8 k get projects my-project -o yaml`) to get more details about the cause of the error.

### Automatic project creation for a namespace

A namespace created directly (for example, `d8 k create ns test`) automatically becomes a project with the same name — no annotation is required:

- the template is picked from what the namespace already carries: `secure` if it has the `security-scanning.deckhouse.io/enabled` label, `default` if it has `security.deckhouse.io/pod-policy` or `extended-monitoring.deckhouse.io/enabled`, and `simple` otherwise;
- the project parameters are filled in from the current state of the namespace, so nothing inside it changes: the [network policy](../../glossary.html#network-policy) stays unrestricted and the Pod Security Standard keeps the value the namespace already had;
- from then on the project is the source of truth and is edited like any other project. The namespace of the project cannot be deleted directly: the request is refused with the `Manual modification is forbidden` message. To remove the environment, delete the Project: `d8 k delete project <NAME>` deletes the namespace with it and returns once the namespace is gone. Only a user allowed to delete the namespace can delete its Project (see [Additional project namespaces](#additional-project-namespaces)).

Users can create a namespace directly only while the [`allowNamespacesWithoutProjects`](/modules/multitenancy-manager/configuration.html#parameters-allownamespaceswithoutprojects) parameter is `true` (the default). With `false`, only cluster administrators (`system:masters`, `system:sudouser`), Deckhouse, the module controller and [system components](../../glossary.html#system-component) create namespaces directly. Other users create a Project or a ProjectNamespace instead.

[System namespaces](../../glossary.html#system-namespace) (`d8-*`, `kube-*`, `upmeter-*`, `default`, and anything labeled `heritage: deckhouse` or `heritage: upmeter`) are never adopted this way: they belong to the virtual `deckhouse`/`default` projects (see [Virtual projects](#virtual-projects)). A namespace whose name is longer than 61 characters is not adopted either: that is the Project name limit.

A namespace that gets the `default` or `secure` template because of its monitoring or scanning label and has no `security.deckhouse.io/pod-policy` label gets `podSecurityProfile: ""`. The project puts no such label on it, and the [default policy](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) of the `admission-policy-engine` module keeps applying to it.

A namespace that already belongs to another Helm release (its `meta.helm.sh/release-name` annotation names a release other than the project's) is not taken over: the project gets the `HelmOwnership` condition with the name of that release and retries, and the namespace is left as it is. Remove the foreign release or its ownership annotations to let the project proceed.

Existing RoleBinding and AuthorizationRule objects inside the namespace keep working after adoption. The namespace Admin is **not** copied into `.spec.administrators` and does **not** become `d8:project:admin`. To make them a project administrator, add them to `.spec.administrators` or create a ProjectRoleBinding.

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

   ```console
   NAME        STATE      PROJECT TEMPLATE   DESCRIPTION                                            AGE
   deckhouse   Deployed   virtual            This is a virtual project                              181d
   default     Deployed   virtual            This is a virtual project                              181d
   test        Deployed   simple                                                                    1m
   ```

To change the template of such a project, set another template in `.spec.projectTemplateName`.

{% alert level="warning" %}
If the namespace already has an object of the same kind and name as one that the new template creates, for example the `isolated` NetworkPolicy, the project switches to `Error`, and the `ResourcesUpgraded` condition names the object. Delete or rename the object first.
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
- the name cannot be `default` or `deckhouse` — these names belong to the [virtual projects](#virtual-projects);
- the name cannot be longer than 61 characters;
- the name cannot be the name of an existing namespace, or of a namespace that a [ProjectNamespace](#additional-project-namespaces) has claimed (the name is taken from the moment the ProjectNamespace is created, before the controller creates the namespace);
- the name cannot be the name of a namespace whose ServiceAccounts are still named among the subjects of a ClusterProjectRoleBinding, or of a ProjectRoleBinding of a project that the name extends with a dash and a suffix (`foo` for `foo-extra`). Such subjects are often left from a deleted additional namespace. Remove them from the binding first.

A project can be named after another project with a dash and a suffix (`foo-bar` beside `foo`). It is a separate project with a namespace of its own, not an additional namespace of `foo`, and the template, administrators, quota and bindings of `foo` do not apply to it. The webhook allows such a project with a warning, and the controller sends the `NamedLikeAdditionalNamespace` Warning event on the new project. The [`MultitenancyManagerProjectNamedLikeAdditionalNamespace`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-multitenancymanagerprojectnamedlikeadditionalnamespace) alert lists every such pair of projects. To add a namespace to a project, create a [ProjectNamespace](#additional-project-namespaces).

## Project status and diagnostics

The `.status.state` field of a [project](../../glossary.html#project) is either `Deployed` (all project resources are in sync, or the objects of `manifests` wait for their binding) or `Error`. The cause of an error is described in the conditions (`.status.conditions`):

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
| `ManifestsApplied` | The objects of the template's [`manifests`](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests) are not applied. The `message` says what they wait for, why none of them is applied, which existing objects they leave alone, or which objects the API server refused, with the code of each refusal (for example, `403 Forbidden`). |
| `HelmOwnership` | The namespace of the project belongs to another Helm release. The `message` names the release (see [Automatic project creation for a namespace](#automatic-project-creation-for-a-namespace)). |
| `NamespaceDeleted` | The project is being deleted, and its namespace is still terminating. The `message` says since when and what remains in it. |

The `HandoverPending` condition is read the other way round. It is `True` while AuthorizationRule and ResourceQuota objects kept from a Helm text wait for the standard fields that replace them, and it is removed once they are deleted (see [Switching a project from a Helm text](/modules/multitenancy-manager/usage.html#switching-a-project-from-a-helm-text)).

Other useful status fields:

- `.status.namespaces` — all namespaces of the project with their kind (`Main`/`Additional`);
- `.status.usage` — the current quota usage (populated when `.spec.quota` is set);
- `.status.resources` — the state of the individual resources created from the template;
- `.status.keptObjects` — the objects of a Helm text that were [left in place](/modules/multitenancy-manager/usage.html#switching-a-project-from-a-helm-text) instead of being deleted;
- `.status.manifestObjects` — the objects applied from the template's [`manifests`](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests).

### Service objects of a project

The controller creates service objects in the project namespaces. They are managed automatically — editing them manually is not possible (the attempt is rejected):

| Object | Where | Comes from |
|--------|-------|------------|
| `ResourceQuota/d8-project-quota` | The main namespace | The [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) field of the project. |
| `ProjectRoleBinding/d8-administrators` | The main namespace | The [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) field of the project. |
| `ProjectRoleBinding/d8-manifests` | The main namespace | The [`manifests`](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests) field of the project template. It grants the `d8:project:admin` role to the user that applies these objects. |
| `RoleBinding/d8:prb:<name>` | Every namespace of the project | The fan-out of the [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) named `<name>`. |
| `RoleBinding/d8:cprb:<name>` | Every namespace of every project | The fan-out of the [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) named `<name>`. |

When the source object (a binding, the quota field, etc.) is removed, the corresponding service objects are removed automatically.

## Virtual projects

Besides the user-created projects, the `d8 k get projects` list always contains two **virtual** projects (labeled `projects.deckhouse.io/virtual-project: "true"`):

- `deckhouse` — groups [system namespaces](../../glossary.html#system-namespace) (`d8-*`, `kube-*`, `upmeter-*`, `heritage: deckhouse` / `heritage: upmeter`);
- `default` — groups remaining namespaces that do not belong to any project (the `default` namespace itself).

Virtual-project status is rebuilt from the live namespace list: a deleted namespace disappears from that list. Virtual projects do not recreate namespaces.

Virtual projects exist for completeness: with them, every namespace of the cluster belongs to some project. They cannot be managed: they are not editable, [ProjectNamespace](/modules/multitenancy-manager/cr.html#projectnamespace) and [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) resources cannot be created in them, and [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) does not extend to them. Their project template, `virtual`, is reserved for them: a user project with `projectTemplateName: virtual` is refused by the webhook. No ClusterResourceGrantPolicy applies to their namespaces either, even one whose selectors match a virtual Project, because the [cluster resource availability rules](#managing-access-to-cluster-wide-resources) apply only in namespaces with the `projects.deckhouse.io/project` label, and the namespaces of virtual projects do not carry it.

No ProjectTemplate object named `virtual` exists, and the webhook refuses to create one. A template with this name that is already in the cluster is not used by any project and can be deleted.

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

You can check the [project](../../glossary.html#project) composition in its status:

```shell
d8 k get project my-project -o jsonpath='{.status.namespaces}'
```

The rules for working with ProjectNamespace:

- The `spec.name` field is immutable: to rename a namespace, delete the resource and create a new one.
- The resulting name `<project name>-<spec.name>` cannot be longer than 63 characters (the Kubernetes limit on namespace names).
- A ProjectNamespace can only be created in the main namespace of a project — it cannot be "nested" into an additional namespace or a foreign project.
- The resulting name must be free. The request is rejected when a namespace with that name exists and is not a namespace of this project, when a project has that name, or when another ProjectNamespace already claims it (the ProjectNamespace `a-backend` of project `team` and the ProjectNamespace `backend` of project `team-a` both claim `team-a-backend`).
- The request is also rejected when the ServiceAccounts of a namespace with the resulting name are still named among the subjects of a ClusterProjectRoleBinding, or of a ProjectRoleBinding of another project that the resulting name extends with a dash and a suffix. Remove these subjects from the binding first.
- Deleting a ProjectNamespace resource deletes its namespace. Deleting the project deletes all of its namespaces.
- A project can be deleted only by a user allowed to delete each of its namespaces (the `delete` verb on `namespaces`). Otherwise the request is refused, and the refusal names the namespace the user may not delete.

### What applies to the additional namespaces

The following automatically applies in **all** namespaces of the project (the main and the additional ones alike):

- **Access**: the [ProjectRoleBinding](#granting-access-within-a-project) and [ClusterProjectRoleBinding](#granting-access-within-a-project) bindings, including the automatic access of the project administrators. When a new namespace is added, all existing bindings fan out into it without any user action.
- **Namespaced template objects**: the [network policy](../../glossary.html#network-policy) (`networkPolicy.mode: Isolated`) and the log collection setup (`logShipping`) are created in every namespace of the project. The network isolation allows traffic between the namespaces of one project. A Helm `resourcesTemplate` creates its objects in the main namespace unless an object names another namespace of the project. So does [`manifests`](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests), whose objects are not applied while one of them names a namespace outside the project.
- **Cluster-scoped template policies** (`OperationPolicy`, the `SecurityPolicy` from `allowedUIDs`/`allowedGIDs`): they select namespaces by the `projects.deckhouse.io/project` label, that is, they cover the whole project.
- **Inherited labels**: the pod security profile (`security.deckhouse.io/pod-policy`), extended monitoring (`extended-monitoring.deckhouse.io/enabled`), vulnerability scanning (`security-scanning.deckhouse.io/enabled`), the template label (`projects.deckhouse.io/project-template`) and the labels the template sets through `namespaceMetadata.labels` are synced from the main namespace to the additional ones. The sync is complete: if a feature is turned off or a label is removed in the template, the label is removed from the additional namespaces as well. The labels of the template are listed in the `projects.deckhouse.io/template-labels` annotation of each project namespace; they are changed through the Project or its ProjectTemplate, and a direct edit of one on an additional namespace is reverted. Thanks to the template label, the [cluster resource availability rules](#managing-access-to-cluster-wide-resources) also apply in all namespaces of the project.

The following stays in the **main** namespace only:

- the project quota (the `ResourceQuota` from [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota));
- the annotations from the template's `namespaceMetadata`;
- the labels and annotations set on the main namespace directly or by a GitOps tool, including the ones the namespace had before it [became a project](#automatic-project-creation-for-a-namespace), other than the pod security, monitoring and scanning labels above;
- the GitOps tracking labels (`app.kubernetes.io/instance`, `argocd.argoproj.io/*`, `kustomize.toolkit.fluxcd.io/*`, `helm.toolkit.fluxcd.io/*`, `kapp.k14s.io/*`), even when the template sets them;
- the node placement annotations (from the template's `nodeSelector` and `tolerations` fields).

When a namespace [becomes a project automatically](#automatic-project-creation-for-a-namespace), its own labels and annotations are not copied into the Project, so the additional namespaces of the project do not inherit them. The pod security, monitoring and scanning labels are the exception: adoption turns them into parameters of the project. To put another label of such a namespace on the additional namespaces too, declare it in `spec.parameters.namespace.labels` of the Project, or set it on each additional namespace directly.

### Labels of the project namespaces

| Label | Main | Additional | Purpose |
|-------|:----:|:----------:|---------|
| `projects.deckhouse.io/project: <project name>` | ✓ | ✓ | Project ownership — the common label of all namespaces of the project. |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Marks an additional namespace (the name of the ProjectNamespace resource). |
| `projects.deckhouse.io/project-template: <template name>` | ✓ | ✓ | The project template; the cluster resource availability rules match by it. |
| `heritage: multitenancy-manager` | ✓ | ✓ | The namespace is managed by the project controller: its `spec`, finalizers and the labels listed in this table are changed through the Project; other labels and annotations may be changed directly. |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (inherited) | Policies and features from the project template. |
| Labels from the template's `namespaceMetadata.labels` | ✓ | ✓ (inherited) | Labels of the project template. They are listed in the `projects.deckhouse.io/template-labels` annotation of the namespace and changed through the Project or its ProjectTemplate. |
| Any other label set on a namespace directly or by a GitOps tool | — | — | Not set by the module. The project release neither reverts nor restores it, and the additional namespaces do not inherit it. |

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

[Project](../../glossary.html#project) administrators and resource quotas are not project template parameters — they are top-level fields of the [Project](/modules/multitenancy-manager/cr.html#project) resource and work with any template (including `simple`):

- [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) — a list of subjects (`kind: User` or `kind: Group` and `name`) that receive administrative access to the project. The controller implements this access through an auto-generated [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) named `d8-administrators` in the project's main namespace. A name cannot start with `system:multitenancy-manager:`, the prefix of the users that the controller acts as.
- [`.spec.quota`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-quota) — a set of hard [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) limits (for example, `requests.cpu`, `limits.memory`). The controller maintains a `ResourceQuota` in the project's main namespace and reports current usage in `.status.usage`. For `memory` and `storage`, a unit is required (for example, `2Gi`). Numbers without a unit mean bytes, and all of them except 0 are rejected. An update that leaves such a value as it was is accepted with a warning, and so is a create through `deckhouse.io/v1alpha2` with such numbers in `parameters.resourceQuota`.

{% alert level="warning" %}
A Helm `resourcesTemplate` that renders a ResourceQuota or an AuthorizationRule keeps rendering it, next to the ResourceQuota and the ProjectRoleBinding that the standard fields create. A pod in such a project has to fit every ResourceQuota of its namespace, and the access such AuthorizationRules grant adds to the access of the standard fields.
{% endalert %}

## Granting access within a project

To grant access to project namespaces for users beyond the project administrators, [use role](../../glossary.html#use-role) bindings that reference cluster-wide roles and fan out into the appropriate project namespaces automatically:

- [ProjectRoleBinding](/modules/multitenancy-manager/cr.html#projectrolebinding) (namespaced, short name `prb`) — grants a role within a **single** project. It must be created in the project's main namespace. The controller creates a RoleBinding in every namespace of that project.
- [ClusterProjectRoleBinding](/modules/multitenancy-manager/cr.html#clusterprojectrolebinding) (cluster-scoped, short name `cprb`) — grants a role across **all** non-virtual projects.

`roleRef` must reference a `ClusterRole` whose name starts with one of the allowed prefixes (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). See [the user-authz module documentation](/modules/user-authz/) for the description of the roles.

The following checks apply when a binding is created:

- **Privilege escalation protection**: a binding can only be created by a user who has the right to bind (`bind`) the referenced role; the right is checked by the role's name. A project administrator (`d8:project:admin`) has `bind` on exactly eight roles: `d8:project:viewer`, `d8:project:user`, `d8:project:manager`, `d8:project:admin`, `d8:namespace:viewer`, `d8:namespace:user`, `d8:namespace:manager`, and `d8:namespace:admin`. A binding to any other role — a custom `d8:custom:*` role, a capability role, or `d8:project:superadmin` — is rejected for a project administrator, even if it is within their own permissions, because nobody granted them `bind` on that name. Such bindings are created by a cluster administrator, or the cluster administrator grants `bind` on that role to project administrators separately via a dedicated ClusterRole.
- The role must exist: a binding to a non-existent role is rejected.
- A ServiceAccount used as a ProjectRoleBinding subject must belong to a namespace of that same project: its main namespace, its additional namespaces or a namespace a ProjectNamespace of the project has claimed. A namespace that is only named like an additional namespace (`<project>-*`) but belongs to a separate project does not qualify.
- System and subsystem roles (`d8:system:*`, `d8:subsystem:*`), as well as arbitrary roles outside the listed prefixes, cannot be granted through project bindings.
- Roles with the `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` annotation are forbidden for granting in projects. A cluster administrator can put this annotation on a role to stop new grants of it: existing bindings keep working, but no new ones are created. If a project template uses such a role, the project switches to `Error` with the reason in the `TemplateRolesAllowed` condition.

The controller checks the ServiceAccounts by the same rule every time it updates the RoleBindings of a ProjectRoleBinding, and also when a namespace appears, is deleted or moves to another project. The subjects that do not pass stay in `.spec.subjects` but get no rights: the `SubjectsBound` condition of the binding is `False` and lists them. Once the namespace is a namespace of the project again, the subjects get the rights back without a change to the binding.

The `d8-administrators` binding, created by the controller from the [`.spec.administrators`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-administrators) field, is managed by the controller only — it cannot be edited manually. To change the set of administrators, change the project's `.spec.administrators` field instead. The ServiceAccount rule above does not apply to this binding.

The following bindings give the `viewer@example.com` user the `d8:project:viewer` role in every namespace of `my-project`, and the `platform` group the same role in every project except the virtual ones:

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

### Roles available in a RoleBinding inside a project

Besides the [project](../../glossary.html#project) bindings, a plain RoleBinding can also be used inside a project namespace — the role then applies in that single namespace only. However, in projects the set of roles available to a plain RoleBinding is restricted: only cluster roles carrying the `rbac.deckhouse.io/delegatable: "true"` label are allowed. Among the built-in ones these are the `d8:namespace:*` and `d8:project:*` roles, as well as the access-level roles of the basic [role model](../../glossary.html#rbac) (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

A RoleBinding to any other cluster role (for example, `cluster-admin`, system roles, or capabilities) is rejected in a project with the message `references "<role>" which is not available to project`. This protects the project isolation from being bypassed by binding to an overly broad role.

To use a [custom role](/modules/user-authz/faq.html#creating-a-custom-namespace-or-project-role) in projects, add the `rbac.deckhouse.io/delegatable: "true"` label to it:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

The restriction applies in the namespaces of every project, including the ones adopted from a standalone namespace.

## Structured templates

Starting with the `deckhouse.io/v1alpha2` API version, a project template is described by **structured fields** — instead of a text Helm template, you declaratively specify which settings the project namespaces get. The controller itself creates the corresponding objects ([network policies](../../glossary.html#network-policy), security policies, log collection settings, etc.) from these fields in every namespace of the project and keeps them up to date. Other objects of the project namespaces are described in [`manifests`](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests).

Available fields (all optional; the complete reference is [in the ProjectTemplate resource description](/modules/multitenancy-manager/cr.html#projecttemplate)):

| Field | What it configures |
|-------|--------------------|
| `podSecurityStandard` | Pod security profile: `Privileged`, `Baseline`, or `Restricted`. |
| `networkPolicy.mode` | Network isolation: `Isolated` (traffic is only allowed within the project and from the platform [system components](../../glossary.html#system-component)) or `NotRestricted`. |
| `features.monitoring` | [Extended monitoring](../../glossary.html#extended-monitoring) of the project namespaces. |
| `features.vulnerabilityScanning` | Scanning of container images for vulnerabilities. |
| `logShipping.clusterDestinationRef` | Collecting the logs of the project pods into the given destination (`ClusterLogDestination`). |
| `nodeSelector`, `tolerations` | Placing the pods of the main namespace on dedicated nodes. |
| `allowedUIDs`, `allowedGIDs` | The allowed UID/GID ranges of the project containers. |
| `runtimeAudit.enabled` | Auditing the project processes' access to the Linux kernel. |
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Extra labels and annotations of the project namespaces. The labels the module manages cannot be set here: a template or a Project that sets one is refused. |
| `resources`, `grantPolicies` | Granting cluster-scoped resources through a project template — see [Managing access to cluster-wide resources](#managing-access-to-cluster-wide-resources). |
| `manifests` | Other objects in the project namespaces, applied with the rights of a project administrator. See [Describing other objects in manifests](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests). |
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

A [project](../../glossary.html#project) using such a template:

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

The `fromParam` references are checked when a template is created or changed: a reference to an undeclared parameter or to a parameter of an incompatible type (for example, a string parameter for a boolean field) is rejected.

The `requiredRequests` parameter is read by name, without `fromParam`. The controller renders an OperationPolicy in the `Deny` mode that requires CPU and memory requests from the pods in every namespace of the project only when the template declares the parameter in `parametersSchema` with `type: boolean` and its value for the project is `true`. A template that does not declare the parameter renders no such policy, even if the project sets `requiredRequests` as an undeclared key.

### Template checks

The following rules apply to template operations:

- A template cannot be deleted while at least one project uses it or a project namespace is still rendered from it. After a project switches to another template, the old template stays until the new one has rendered.
- A change to a template is automatically applied to all projects created from it.
- An existing template can describe its objects with a Helm text in [`resourcesTemplate`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-resourcestemplate) instead of structured fields. This form is deprecated, and the `resourcesTemplate` field will be removed together with `deckhouse.io/v1alpha1` in one of the upcoming releases. The `MultitenancyManagerHelmProjectTemplate` alert names every template that still renders from a Helm text, except the built-in ones. Describe new templates with structured fields and [`manifests`](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests), and move the existing ones as [Moving a template from a Helm text](/modules/multitenancy-manager/usage.html#moving-a-template-from-a-helm-text) describes.
- A template cannot get a new `resourcesTemplate` through either version. A create with a non-empty `resourcesTemplate` and an update that sets it on a template whose `resourcesTemplate` is empty are refused. A template that has a Helm text keeps it, and the text can be changed through either version. So a template with a Helm text cannot be created again, for example from a manifest exported from another cluster, from a backup, or from Git after the template was deleted. Move such a template to structured fields and `manifests` first.
- The `deckhouse.io/v1alpha1` version of ProjectTemplate is served and deprecated, and the API server returns a warning on every request through it. A template is created through `deckhouse.io/v1alpha2` only, and a create through `deckhouse.io/v1alpha1` is refused. The templates created through `deckhouse.io/v1alpha1` earlier are read, changed and deleted through it as before. A change through `v1alpha1` to a template that has fields `v1alpha1` cannot describe, such as `title`, `grantPolicies`, `podSecurityStandard` or `manifests`, is refused, because it would erase them. Change such a template through `deckhouse.io/v1alpha2`.

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
1. Remove the `heritage` label, the `metadata.resourceVersion`, `metadata.uid`, `metadata.creationTimestamp`, `metadata.generation` and `metadata.managedFields` fields and the `status` section from the file.
1. Apply the resulting template with the following command:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Check that the new template is available by running:

   ```shell
   d8 k get projecttemplates <NEW_TEMPLATE_NAME>
   ```

## Labels for the objects of a Helm text

The following labels change how the module handles the objects of a Helm text in `resourcesTemplate`. Only existing templates have such a text, because no template can get a new `resourcesTemplate`. The objects of [`manifests`](/modules/multitenancy-manager/usage.html#describing-other-objects-in-manifests) never get the `heritage: multitenancy-manager` label, and the module does not read these labels on them.

### Skipping the heritage label

By default, every object of the Helm text gets the `heritage: multitenancy-manager` label.
This label prohibits changes to the object by users or any other controller except `multitenancy-manager`.
To allow changes to the object (for example, for compatibility with other systems, or to control the created objects yourself), add the `projects.deckhouse.io/skip-heritage-label` label to it.

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

In this case, the object gets the `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` labels, but does not get the `heritage: multitenancy-manager` label.

### Excluding objects from management

To exclude an object from management by the module (for example, when it is managed manually or by another controller), add the `projects.deckhouse.io/unmanaged` label to it.

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

Objects with the `projects.deckhouse.io/unmanaged` label:

- are created **only once**, when the [project](../../glossary.html#project) release is first installed, so an object added to the template later is not created in existing projects;
- **are not updated** when the template changes;
- are not tracked in the project status;
- get the `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` labels, but **not** the `heritage: multitenancy-manager` label;
- get the `helm.sh/resource-policy: keep` annotation, so the project release does not delete them.

{% alert level="warning" %}
After it is created, an `unmanaged` object does not depend on the template and must be managed manually.
{% endalert %}

## Implementing validation of object changes with a custom label

The `multitenancy-manager` module uses ValidatingAdmissionPolicy to protect resources labeled `heritage: multitenancy-manager` from manual changes.  
You can implement similar validation for resources with any label.

### How validation works in multitenancy-manager

`multitenancy-manager` validates objects labeled `heritage: multitenancy-manager`.  
The following resources are used for this:

1. ValidatingAdmissionPolicy: Defines validation rules:
   - Operations: `CREATE`, `UPDATE` and `DELETE`.
   - Exclusions: requests from cluster components are not checked, that is, from system groups (`system:nodes`, the `kube-system` and `d8-system` service accounts) and system users (`system:apiserver`, `system:kube-controller-manager`, `system:kube-scheduler` and others).
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
           operations:  ["CREATE", "UPDATE", "DELETE"]
           resources:   ["*"]
           scope: "*"
     matchConditions:
       # Let the cluster components through, as the policy of the module does.
       - name: exclude-system-groups
         expression: '!(["system:nodes", "system:serviceaccounts:kube-system", "system:serviceaccounts:d8-system"].exists(e, (e in request.userInfo.groups)))'
       - name: exclude-system-users
         expression: '!(["system:apiserver", "system:kube-controller-manager", "system:kube-scheduler"].exists(e, (e == request.userInfo.username)))'
     variables:
       # A DELETE request carries no object, only oldObject.
       - name: subject
         expression: 'object == null ? oldObject : object'
     validations:
       - expression: 'request.userInfo.username == "system:serviceaccount:my-namespace:my-service-account"' # Replace with your ServiceAccount.
         reason: Forbidden
         messageExpression: 'variables.subject.kind == ''Namespace'' ? ''This resource is managed by '' + variables.subject.metadata.name + '' system. Manual modification is forbidden.''
           : ''This resource is managed by '' + variables.subject.metadata.namespace + '' system. Manual modification is forbidden.'''
   ---
   apiVersion: admissionregistration.k8s.io/v1
   kind: ValidatingAdmissionPolicyBinding
   metadata:
     name: my-custom-label-validation
   spec:
     policyName: my-custom-label-validation
     validationActions: [Deny, Audit]
     matchResources:
       objectSelector:
         matchLabels:
           heritage: my-custom-label
   ```

1. Configure the validation parameters:

   - `policyName`: Unique policy name (must match in Policy and Binding).
   - `request.userInfo.username`: The name of the ServiceAccount allowed to change objects (replace with your ServiceAccount).
   - `heritage: my-custom-label`: The value of the `heritage` label for your resources (replace with your value). The use of the values `multitenancy-manager`, `deckhouse` is prohibited.
   - `failurePolicy`: What happens when the policy cannot be evaluated, for example when an expression fails: `Fail` rejects the request, `Ignore` admits it. A validation that returns `false` rejects the request with the `Deny` action in any case.
   - `matchConditions`: Requests that the policy does not check. Keep the cluster components in this list, otherwise the policy blocks the deletion of namespaces and the garbage collection of the labeled objects.
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

The `multitenancy-manager` module allows cluster administrators to define, for each [project](../../glossary.html#project), which cluster-wide resources (such as StorageClass, ClusterIssuer, ClusterRole, and LoadBalancerClass) can be used from project namespaces and which values are used by default.

This mechanism works independently of RBAC. RBAC determines *who can create and modify* objects, while the cluster-wide resource access mechanism determines *which resources* those objects can use.

The mechanism uses four custom resources:

- [GrantableClusterResourceDefinition](/modules/multitenancy-manager/cr.html#grantableclusterresourcedefinition) registers a type of cluster-wide resource whose access can be managed. These resources are provided by DP or module developers.
- [GrantableClusterResourceReference](/modules/multitenancy-manager/cr.html#grantableclusterresourcereference) defines where a registered cluster-wide resource is used, for example, which resource field contains a reference to it. These resources are provided by modules.
- [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) defines access rules: the allowed and denied resources, as well as the resource used by default. A cluster administrator selects the namespaces to which the policy applies with two selectors: `projectSelector` matches the labels of the Project, and `namespaceSelector` matches the labels of the Namespace. A policy with neither selector is a library policy: it applies to a project only through `grantPolicies` of its ProjectTemplate.
- [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) is a read-only list of cluster-wide resources available to the project under the policies that apply. The controller keeps one in every namespace of every project for each GrantableClusterResourceDefinition.

Until the administrator creates a ClusterResourceGrantPolicy, resource availability is determined by their registration: resources are available to all projects if `defaultAvailability: All` (the default value) is set in GrantableClusterResourceDefinition and the resource does not match the `excluded` filters.
Access checks are performed only for objects in project namespaces. If an access policy changes, cluster-wide resources already used by existing objects remain available to those objects.

The webhooks reject requests while the module controller is unavailable. Until it is back, objects of the registered kinds (for example, PersistentVolumeClaim, Service or RoleBinding) cannot be created or changed in project namespaces, except by the platform components and members of the `system:masters` group. To keep the controller available, enable [`highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability).

For a detailed description of the access management mechanism, refer to the [`multitenancy-manager`](/modules/multitenancy-manager/#managing-access-to-cluster-wide-resources) module documentation.
