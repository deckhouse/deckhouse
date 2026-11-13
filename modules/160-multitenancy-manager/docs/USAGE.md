---
title: "The multitenancy-manager module: usage examples"
---

## Default project templates

Deckhouse Platform includes the following project templates. They are cumulative: each one includes the capabilities of the previous one and adds its own. Parameter values are set in the `.spec.parameters` field of a Project:

- `simple`: Minimal template that creates only the project namespace. Use it when you only need an isolated namespace managed as a project and configure access and limits through the [standard fields](#standard-project-fields) and [project role bindings](#granting-access-within-a-project).

  Parameters:
  - `namespace.labels` and `namespace.annotations`: Extra labels and annotations for the project namespace.
  - `requiredRequests` (default `false`): When `true`, the pods in the project must specify CPU and memory requests (an OperationPolicy in the `Deny` mode).

- `default`: Template that covers basic project use cases. On top of the namespace, it sets up network isolation, a pod security profile, extended monitoring and log shipping.

  Parameters (besides the ones listed for `simple`):
  - `networkPolicy`: `Isolated` (default) allows incoming traffic only from the project namespaces, Prometheus, the ingress-nginx controller and the service-with-healthchecks agent, and outgoing traffic only to the project namespaces and to DNS in `kube-system`. `NotRestricted` allows all traffic.
  - `podSecurityProfile`: The [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) profile for the project namespaces: `Baseline` (default) prevents known privilege escalations, `Restricted` applies the strictest hardening practices, `Privileged` restricts nothing. An empty string sets no profile. The project puts no `security.deckhouse.io/pod-policy` label on its namespaces, and the [default policy](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) of the `admission-policy-engine` module applies to them.
  - `extendedMonitoringEnabled` (default `true`): Alerts on controller outages and restarts, 5xx errors in ingress-nginx and low free space on the project's persistent volumes.
  - `clusterLogDestinationName`: The name of the ClusterLogDestination to ship the project logs to. Left unset, the project logs are not shipped anywhere.
  - `requiredRequests` (default `true` in this template): When `true`, the pods in the project must specify CPU and memory requests (an OperationPolicy in the `Deny` mode). When an existing namespace becomes a project, the parameter is set to `false`, so running workloads are not blocked.

- `secure`: Includes all the capabilities of the `default` template, and additionally restricts the users and groups inside containers, audits their calls to the kernel and scans images for vulnerabilities.

  Parameters (besides the ones listed for `default`):
  - `allowedUIDs` and `allowedGIDs`: The ranges (`min`, `max`) of IDs permitted for users and groups inside the project containers. See [Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/#set-the-security-context-for-a-pod).
  - `runtimeAuditEnabled` (default `false`): Audit rules that catch calls to the kernel and find malicious activity. They work only if the UID/GID ranges are set.
  - `securityScanningEnabled` (default `true`): Periodic scanning of the launched images for known vulnerabilities (CVE) with Trivy, every 24 hours.

- `secure-with-dedicated-nodes`: Includes all the capabilities of the `secure` template, and additionally places the pods of the main namespace of the project on dedicated nodes.

  Parameters (besides the ones listed for `secure`). If `dedicatedNodes` is set, at least one of its two fields must be set. Without `dedicatedNodes`, the pods are not placed on dedicated nodes:
  - `dedicatedNodes.nodeSelector`: The node selector of the main namespace. It is merged into the node selector of every pod there, and a pod whose selector conflicts with it is rejected.
  - `dedicatedNodes.defaultTolerations`: Tolerations in the format of the pod's `spec.tolerations`. They are **added** to the pods created in the main namespace.

The `default`, `secure`, and `secure-with-dedicated-nodes` templates are described in the [structured form](#structured-templates) (`deckhouse.io/v1alpha2`). The `simple` template is a minimal structured template that creates the namespace, sets its labels and annotations from the project parameters and, with `requiredRequests: true`, creates the OperationPolicy.

The exact set of parameters is in the template installed in the cluster. To view the parameter schema of a template, use the command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o jsonpath='{.spec.parametersSchema.openAPIV3Schema}' | jq
```

To view the whole template, use the command:

```shell
d8 k get projecttemplates <PROJECT_TEMPLATE_NAME> -o yaml
```

## Creating a project

To create a project, follow these steps:

1. Create a [Project](cr.html#project), specifying the name of the project template in [.spec.projectTemplateName](cr.html#project-v1alpha3-spec-projecttemplatename) field.
1. Set the [standard fields](#standard-project-fields) [.spec.administrators](cr.html#project-v1alpha3-spec-administrators) and [.spec.quota](cr.html#project-v1alpha3-spec-quota). They work with any template.
1. In the [.spec.parameters](cr.html#project-v1alpha3-spec-parameters) field of the Project resource, specify the parameter values suitable for the ProjectTemplate [.spec.parametersSchema.openAPIV3Schema](cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema).

   Example of a Project created from the `default` [ProjectTemplate](cr.html#projecttemplate):

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

   The Project API versions are described in [Project API versions](#project-api-versions).

1. To check the status of the project, execute the command:

   ```shell
   d8 k get projects my-project
   ```

   A successfully created project should be in the `Deployed` state. If the state equals `Error`, add the `-o yaml` argument to the command (e.g., `d8 k get projects my-project -o yaml`) to get more detailed information about the error.

### Project API versions

The Project API is served as `deckhouse.io/v1alpha3`. `v1alpha2` manifests keep working: a conversion webhook lifts `parameters.administrators` and `parameters.resourceQuota` into the `.spec.administrators` and `.spec.quota` standard fields automatically. If one administrator does not fit `.spec.administrators` (for example, it has `accessLevel`), the whole list stays in `parameters` and `.spec.administrators` is not filled. The same holds for a quota with anything other than quantities (for example, `scopes`), for a quota with the number `0` anywhere in it, and for an empty list or quota. A template that checks a value with `with` skips the number `0` and sets no limit, while in `.spec.quota` it would be a hard limit of zero. The string `"0"` is lifted into `.spec.quota` as such a limit. A `v1alpha2` client reads a `0` from `.spec.quota` as the string `"0"`.
A project cannot set `.spec.administrators` together with `parameters.administrators`, or `.spec.quota` together with `parameters.resourceQuota`.
The `spec.grants` field exists only in `deckhouse.io/v1alpha3`: a change of a project that has it through `deckhouse.io/v1alpha2` is refused, see [Granting cluster-wide resources to one project](#granting-cluster-wide-resources-to-one-project).

The `deckhouse.io/v1alpha1` version is no longer served, and the API server refuses a manifest with `apiVersion: deckhouse.io/v1alpha1`. Before applying such a manifest, change `apiVersion` to `deckhouse.io/v1alpha3`, rename `spec.projectTypeName` to `spec.projectTemplateName` (a ProjectTemplate with that name must exist) and `spec.template` to `spec.parameters`, and move `administrators` and `resourceQuota` from the parameters to `.spec.administrators` and `.spec.quota`.

### Creating a project without specifying a template

The `projectTemplateName` field is optional: when omitted, the `simple` template is used. Such a project consists only of the namespace and the [standard fields](#standard-project-fields) (administrators, quota) — no template policies are created in it. Use it when the namespace needs no template settings or they are managed by other means:

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

- The name cannot start with `d8-` or `kube-`: these prefixes are reserved for system namespaces.
- The name cannot be `default` or `deckhouse`: these names belong to the [virtual projects](#virtual-projects).
- The name cannot be longer than 61 characters.
- The name cannot be the name of an existing namespace, or of a namespace that a [ProjectNamespace](#additional-project-namespaces) has claimed. The resulting name `<project>-<spec.name>` is taken from the moment the ProjectNamespace is created, before the controller creates the namespace. A namespace that belongs to no project becomes a project [automatically](#creating-a-project-automatically-for-a-namespace).
- The name cannot be the name of a namespace whose ServiceAccounts are still named among the subjects of a [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding), or of a [ProjectRoleBinding](cr.html#projectrolebinding) of a project that the name extends with a dash and a suffix (`foo` for `foo-extra`). Such subjects are often left from a deleted additional namespace, and without this rule the ServiceAccounts of the new project would get the rights of that binding. Remove these subjects from the binding first. The refusal does not name the binding.

A project can be named after another project with a dash and a suffix (`foo-bar` beside `foo`). It is a separate project with a namespace of its own, not an additional namespace of `foo`, and the template, administrators, quota and bindings of `foo` do not apply to it. The webhook allows such a project with a warning, and the controller sends the `NamedLikeAdditionalNamespace` Warning event on the new project. The [`MultitenancyManagerProjectNamedLikeAdditionalNamespace`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-multitenancymanagerprojectnamedlikeadditionalnamespace) alert lists every such pair of projects, including the ones made from namespaces that existed before. To add a namespace to a project, create a [ProjectNamespace](#additional-project-namespaces).

## Project status and diagnostics

The `.status.state` field of a project is either `Deployed` (all project resources are in sync, or the objects of `manifests` wait for their binding) or `Error`. The cause of an error is described in the conditions (`.status.conditions`):

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
| `ManifestsApplied` | The objects of the template's [`manifests`](#describing-other-objects-in-manifests) are not applied. The `message` says what they wait for, why none of them is applied, which existing objects they leave alone, or which objects the API server refused, with the code of each refusal (for example, `403 Forbidden`). |
| `HelmOwnership` | The namespace of the project belongs to another Helm release. The `message` names the release (see [Creating a project automatically for a namespace](#creating-a-project-automatically-for-a-namespace)). |
| `NamespaceDeleted` | The project is being deleted, and its namespace is still terminating. The `message` says since when and what remains in it. |
| `QuotaWithinLimits` | The usage of the project is above its [ProjectQuota](#project-quota); the `message` names the limits. A project without a quota does not have the condition. |

The `HandoverPending` condition is read the other way round. It is `True` while AuthorizationRule and ResourceQuota objects kept from a Helm text wait for the standard fields that replace them, and it is removed once they are deleted (see [Switching a project from a Helm text](#switching-a-project-from-a-helm-text)).

Other useful status fields:

- `.status.namespaces`: All namespaces of the project with their kind (`Main` or `Additional`).
- `.status.usage`: The usage of the project summed over all of its namespaces, taken from its [ProjectQuota](#project-quota).
- `.status.resources`: State of the individual objects created from the template.
- `.status.keptObjects`: Objects of a Helm text and of the release of the structured fields that the release [left in place](#switching-a-project-from-a-helm-text) instead of deleting them, until [`manifests`](#moving-an-object-of-the-release-to-manifests) take them over.
- `.status.manifestObjects`: Objects applied from [`manifests`](#describing-other-objects-in-manifests) of the template.

### Service objects of a project

The controller creates service objects in the project namespaces. They are managed automatically — editing them manually is not possible (the attempt is rejected):

| Object | Where | Comes from |
|--------|-------|------------|
| `ProjectQuota/<PROJECT_NAME>` | Cluster-scoped | The [`.spec.quota`](cr.html#project-v1alpha3-spec-quota) field of the project. A ProjectQuota created directly is not touched by the field: it is neither overwritten nor deleted, and a quota in the field next to it is refused (see [Project quota](#project-quota)). |
| `ResourceQuota/d8-project-quota` | Every namespace of the project | The [ProjectQuota](#project-quota) of the project. |
| `ProjectRoleBinding/d8-administrators` | The main namespace | The [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) field of the project. |
| `ProjectRoleBinding/d8-manifests` | The main namespace | The [`manifests`](#describing-other-objects-in-manifests) field of the project template. It grants the `d8:project:admin` role to the user that applies these objects. |
| `RoleBinding/d8:prb:<name>` | Every namespace of the project | The fan-out of the [ProjectRoleBinding](cr.html#projectrolebinding) named `<name>`. |
| `RoleBinding/d8:cprb:<name>` | Every namespace of every project | The fan-out of the [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) named `<name>`. |

When the source object (a binding, the quota field, etc.) is removed, the corresponding service objects are removed automatically.

## Virtual projects

Besides the user-created projects, the `d8 k get projects` list always contains two **virtual** projects (labeled `projects.deckhouse.io/virtual-project: "true"`):

- `deckhouse` — groups system namespaces (`d8-*`, `kube-*`, `upmeter-*`, `heritage: deckhouse` / `heritage: upmeter`);
- `default` — groups remaining namespaces that do not belong to any project (the `default` namespace itself).

Virtual-project status is rebuilt from the live namespace list: a deleted namespace disappears from that list. Virtual projects do not recreate namespaces.

Virtual projects exist for completeness: with them, every namespace of the cluster belongs to some project. They cannot be managed: they are not editable, [ProjectNamespace](cr.html#projectnamespace) and [ProjectRoleBinding](cr.html#projectrolebinding) resources cannot be created in them, and [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) does not extend to them. Their project template, `virtual`, is reserved for them: a user project with `projectTemplateName: virtual` is refused by the webhook, since such a project would get no namespace of its own.
The `projects.deckhouse.io/virtual-project: "true"` label is reserved too: the webhook refuses a project that is created with it or gets it added. A project is virtual by its template, not by the label, so a user project that already carries the label is an ordinary project; it keeps the label with a warning until the label is removed.
No ClusterResourceGrantPolicy applies to their namespaces either, even one whose selectors match a virtual Project, because the [cluster resource availability rules](#managing-access-to-cluster-wide-resources) apply only in namespaces with the `projects.deckhouse.io/project` label, and the namespaces of virtual projects do not carry it.

No ProjectTemplate object named `virtual` exists, and the webhook refuses to create one. A template with this name that is already in the cluster is not used by any project and can be deleted.

## Additional project namespaces

In Deckhouse Platform Open (CE), a project has only its main namespace.

If an application needs several namespaces (for example, a separate one for a cache or queues), add them to the project with the [ProjectNamespace](cr.html#projectnamespace) resource. A ProjectNamespace is created **in the main namespace of the project**; the resulting namespace is named `<project name>-<spec.name>`:

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

- The `spec.name` field is immutable: to rename a namespace, delete the ProjectNamespace and create a new one.
- The resulting name `<project name>-<spec.name>` cannot be longer than 63 characters (the Kubernetes limit on namespace names).
- A ProjectNamespace can only be created in the main namespace of a project. It cannot be "nested" into an additional namespace or a foreign project.
- The resulting name must be free. The request is rejected when a namespace with that name exists and is not a namespace of this project, when a project has that name, or when another ProjectNamespace already claims it (the ProjectNamespace `a-backend` of project `team` and the ProjectNamespace `backend` of project `team-a` both claim `team-a-backend`).
- The request is also rejected when the ServiceAccounts of a namespace with the resulting name are still named among the subjects of a ClusterProjectRoleBinding, or of a ProjectRoleBinding of another project that the resulting name extends with a dash and a suffix (`team` for `team-a-backend` of project `team-a`). Remove these subjects from the binding first. A ProjectRoleBinding of the same project is no obstacle, and the ServiceAccounts it names get the rights back once the namespace is created again.
- Deleting a ProjectNamespace deletes its namespace. Deleting the project deletes all of its namespaces.
- A namespace with the resulting name that is not a namespace of this project is never taken into it, even when it appears after the ProjectNamespace was created. The ProjectNamespace then does not become ready, and deleting it leaves the namespace as it is.
- A project can be deleted only by a user allowed to delete each of its namespaces (the `delete` verb on `namespaces`): the main namespace and every namespace labeled `projects.deckhouse.io/project: <project name>`. Otherwise the request is refused, and the refusal names the namespace the user may not delete. A namespace that does not exist is not checked. The identities the Project webhook skips (see [Who bypasses admission](#who-bypasses-admission)) delete projects without this check.

### What applies to the additional namespaces

The following automatically applies in **all** namespaces of the project (the main and the additional ones alike):

- **Access**: the [ProjectRoleBinding](cr.html#projectrolebinding) and [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) bindings, including the automatic access of the project administrators. When a new namespace is added, all existing bindings fan out into it without any user action.
- **Namespaced template objects**: the network policy (`networkPolicy.mode: Isolated`) and the log collection setup (`logShipping`) are created in every namespace of the project. The network isolation allows traffic between the namespaces of one project. A Helm `resourcesTemplate` creates its objects in the main namespace unless an object names another namespace of the project. So does [`manifests`](#describing-other-objects-in-manifests), whose objects are not applied while one of them names a namespace outside the project.
- **Cluster-scoped template policies** (`OperationPolicy`, the `SecurityPolicy` from `allowedUIDs`/`allowedGIDs`): they select namespaces by the `projects.deckhouse.io/project` label, that is, they cover the whole project.
- **The project quota**: the `d8-project-quota` ResourceQuota with the limits of the [ProjectQuota](#project-quota) of the project is created in every namespace of the project.
- **Inherited labels**: the pod security profile (`security.deckhouse.io/pod-policy`), extended monitoring (`extended-monitoring.deckhouse.io/enabled`), vulnerability scanning (`security-scanning.deckhouse.io/enabled`), the template label (`projects.deckhouse.io/project-template`) and the labels the template sets through `namespaceMetadata.labels` are synced from the main namespace to the additional ones. For a template with `resourcesTemplate`, these are the labels its Helm text puts on the main namespace.
  The sync is complete: if a feature is turned off or a label is removed in the template, the label is removed from the additional namespaces as well. The labels of the template are listed in the `projects.deckhouse.io/template-labels` annotation of each project namespace; they are changed through the Project or its ProjectTemplate, and a direct edit of one on an additional namespace is reverted. Thanks to the template label, the [cluster resource availability rules](#managing-access-to-cluster-wide-resources) also apply in all namespaces of the project.

The following stays in the **main** namespace only:

- The annotations from the template's `namespaceMetadata`
- The labels and annotations set on the main namespace directly or by a GitOps tool, including the ones the namespace had before it [became a project](#creating-a-project-automatically-for-a-namespace), other than the pod security, monitoring and scanning labels above
- The GitOps tracking labels (`app.kubernetes.io/instance`, `argocd.argoproj.io/*`, `kustomize.toolkit.fluxcd.io/*`, `helm.toolkit.fluxcd.io/*`, `kapp.k14s.io/*`), even when the template sets them
- The node placement annotations (from the template's `nodeSelector` and `tolerations` fields)

GitOps tracking labels are not inherited because a GitOps tool would treat an additional namespace that carries them as its own and could delete it. The module recognizes only the keys listed above. A custom tracking key that the template sets, for example the one configured with `application.instanceLabelKey` in Argo CD, is inherited.

When a namespace [becomes a project automatically](#creating-a-project-automatically-for-a-namespace), its own labels and annotations stay on it and are not copied into the Project, so the additional namespaces of the project do not inherit them. The pod security, monitoring and scanning labels are the exception. Adoption turns them into parameters of the project, and they are inherited as on any project. To put another label of such a namespace on the additional namespaces too, declare it in `spec.parameters.namespace.labels` of the Project, or set it on each additional namespace directly.

The built-in templates of adopted projects are rewritten by the controller, so a label added to their `namespaceMetadata.labels` does not stay there. A [grant policy](#managing-access-to-cluster-wide-resources) whose `namespaceSelector` matches such a label does not cover the new additional namespaces until the label is declared in the Project. To cover the whole project, select it with `projectSelector`.

### Labels of the project namespaces

| Label | Main | Additional | Purpose |
|-------|:----:|:----------:|---------|
| `projects.deckhouse.io/project: <PROJECT_NAME>` | ✓ | ✓ | Project ownership — the common label of all namespaces of the project |
| `projects.deckhouse.io/project-namespace: <spec.name>` | — | ✓ | Marks an additional namespace (the name of the ProjectNamespace resource) |
| `projects.deckhouse.io/project-template: <PROJECT_TEMPLATE_NAME>` | ✓ | ✓ | The project template; the cluster resource availability rules match by it |
| `heritage: multitenancy-manager` | ✓ | ✓ | The namespace is managed by the project controller: its `spec`, finalizers and the labels listed in this table are changed through the Project; other labels and annotations may be changed directly |
| `projects.deckhouse.io/quota: pending`, `projects.deckhouse.io/quota: enforced` | ✓ | ✓ | The namespace belongs to a project with a [quota](#project-quota); only the controller sets the label |
| `quota.projects.deckhouse.io/vm: enforced`, `quota.projects.deckhouse.io/lb: enforced` | ✓ | ✓ | The quota of the project has keys of [virtual machines](#virtual-machines-in-a-project-quota) or of [load balancers](#load-balancers-in-a-project-quota) |
| `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled` | ✓ | ✓ (inherited) | Policies and features from the project template |
| Labels from the template's `namespaceMetadata.labels` | ✓ | ✓ (inherited) | Labels of the project template. They are listed in the `projects.deckhouse.io/template-labels` annotation of the namespace and changed through the Project or its ProjectTemplate |
| Any other label set on a namespace directly or by a GitOps tool, including the labels a namespace had before it became a project | — | — | Not set by the module. The project release neither reverts nor restores it, and the additional namespaces do not inherit it |

The common `projects.deckhouse.io/project` label makes it possible to select the project namespaces with a plain `get ns`. To get all namespaces of the project (main and additional), use the command:

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

## Creating a project automatically for a namespace

A namespace created directly (for example, `d8 k create ns my-app`) becomes a project with the same name:

- the template is picked from what the namespace already carries: `secure` if it has the `security-scanning.deckhouse.io/enabled` label, `default` if it has `security.deckhouse.io/pod-policy` or `extended-monitoring.deckhouse.io/enabled`, and `simple` otherwise;
- the project parameters are filled in from the current state of the namespace, so nothing inside it changes: the network policy stays unrestricted and the Pod Security Standard keeps the value the namespace already had;
- from then on the project is the source of truth and is edited like any other project. The namespace of the project cannot be deleted directly: the request is refused with the `Manual modification is forbidden` message. To remove the environment, delete the Project: `d8 k delete project <NAME>` deletes the namespace with it and returns once the namespace is gone. Only a user allowed to delete the namespace can delete its Project (see [Additional project namespaces](#additional-project-namespaces)).

Users can create a namespace directly only while the [`allowNamespacesWithoutProjects`](configuration.html#parameters-allownamespaceswithoutprojects) parameter is `true` (the default). With `false`, only cluster administrators (`system:masters`, `system:sudouser`), Deckhouse, the module controller and system components create namespaces directly (see [Who bypasses admission](#who-bypasses-admission)). Other users create a Project or a ProjectNamespace instead.

Every such namespace becomes a project of its own, whatever its name. Of the namespaces `app` and `app-staging`, `app-staging` becomes a separate project and not an additional namespace of `app`. The controller creates the project, so no admission warning reaches the author of the namespace. Instead, the controller sends the `NamedLikeAdditionalNamespace` Warning event on the new project and on the namespace. The events of cluster-scoped objects are kept in the `default` namespace, so reading them needs access to the events there: `d8 k -n default get events --field-selector reason=NamedLikeAdditionalNamespace`. The [`MultitenancyManagerProjectNamedLikeAdditionalNamespace`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-multitenancymanagerprojectnamedlikeadditionalnamespace) alert lists such pairs, see [Project naming rules](#project-naming-rules).

System namespaces (`d8-*`, `kube-*`, `upmeter-*`, `default`, and anything labeled `heritage: deckhouse` or `heritage: upmeter`) are never adopted: they are listed on the virtual `deckhouse` project (except `default`, which stays on the virtual `default` project).
No label leaves a new user namespace without a project: users cannot put the `heritage: upmeter` label on a namespace, and only the module sets the `projects.deckhouse.io/project` label.
A namespace whose name is longer than 61 characters (the Project name limit) is also skipped, and so is a namespace named `deckhouse`: a Project of that name is the virtual project.

So that no namespace stays outside a project, the `d8-multitenancy-manager-namespace-adoption` ValidatingAdmissionPolicy refuses to create a namespace whose name is longer than 61 characters, starts with `upmeter-` or is `deckhouse`, and to put the `heritage: upmeter` label on a namespace.
The policy works whatever `allowNamespacesWithoutProjects` says and lets through the users and groups listed for the `d8-multitenancy-manager-namespace-creation` policy in [Who bypasses admission](#who-bypasses-admission).
A namespace that stays without a project for 10 minutes, for example while no replica of the module controller leads, raises the [`D8NamespaceNotAdopted`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-d8namespacenotadopted) alert.
The controller pods export the metric of the alert, so the alert is not raised while none of the pods is `Ready`.

The policy does not change the namespaces that exist already, so a namespace that was created before it, with one of these names or with the `heritage: upmeter` label, stays outside every project. To list the namespaces that belong to no project and are not system ones, run:

```bash
d8 k get namespaces -l '!projects.deckhouse.io/project,heritage!=deckhouse' -o name | grep -Ev '^namespace/(d8-|kube-|default$|upmeter-probe-namespace-)'
```

The list holds the namespaces that get no project: the names longer than 61 characters, `deckhouse`, the other `upmeter-*` names and the namespaces with the `heritage: upmeter` label. The namespaces of the upmeter agent can be among them, and they must not be changed.
To bring a namespace under a project, remove the `heritage: upmeter` label from it (if its name does not start with `upmeter-`), and the controller creates the project. Otherwise, move the workloads of the namespace to a namespace under another name. A namespace with an empty `projects.deckhouse.io/project` label is not in the list, `d8 k get namespaces -l 'projects.deckhouse.io/project='` lists such namespaces.

On adoption, a namespace that gets the `default` or `secure` template because of its monitoring or scanning label, and has no `security.deckhouse.io/pod-policy` label, gets `podSecurityProfile: ""`. The project puts no such label on it, and the [default policy](/modules/admission-policy-engine/configuration.html#parameters-podsecuritystandards-defaultpolicy) of the `admission-policy-engine` module keeps applying to it. A namespace whose label has a value other than `baseline`, `restricted` or `privileged` gets `podSecurityProfile: ""` too, and the label stays on it as it is. A namespace with none of these labels gets the `simple` template, which sets no profile and has no `podSecurityProfile` parameter.

To change the profile later, set `podSecurityProfile` in the project. For a project on `simple`, first move it to `default` with `networkPolicy: NotRestricted`, `requiredRequests: false` and `extendedMonitoringEnabled: false`, so that nothing else changes.

A namespace that already belongs to another Helm release (its `meta.helm.sh/release-name` annotation names a release other than the project's) is not taken over: the project gets the `HelmOwnership` condition with the name of the foreign release and retries, and the namespace is left as it is. Remove the foreign release or its ownership annotations to let the project proceed.

Existing RoleBinding and AuthorizationRule objects inside the namespace keep working after adoption. The namespace Admin is **not** copied into `.spec.administrators` and does **not** become `d8:project:admin`: that role additionally manages ProjectRoleBinding resources, which is a wider contract than in-namespace Admin. To make the team lead a project administrator, a platform operator adds them to `.spec.administrators` or creates a ProjectRoleBinding. The namespace Admin role allows only `get`, `list` and `watch` on the Namespace object.

The labels and annotations the module sets — `heritage`, `projects.deckhouse.io/*`, `quota.projects.deckhouse.io/vm`, `quota.projects.deckhouse.io/lb`, `security.deckhouse.io/pod-policy`, `extended-monitoring.deckhouse.io/enabled`, `security-scanning.deckhouse.io/enabled`, `app.kubernetes.io/managed-by`, `meta.helm.sh/*`, the node-selector and tolerations annotations — are changed through the Project and its template;
any other label or annotation (`istio-injection`, a Pod Security label, a GitOps tracking annotation) can still be set directly on the Namespace by anyone whose RBAC allows it, and the controller does not touch it. The `spec` and finalizers of the Namespace, and its deletion, stay with the controller.

This includes the labels and annotations the namespace had before it became a project.
Adoption does not copy them into the project, so the project release does not own them,
and the controller neither reverts a change to one of them nor brings one back once it is removed.
They stay on the main namespace, and the [additional namespaces](#what-applies-to-the-additional-namespaces) of the project do not inherit them.
Where such a label is needed on an additional namespace too, declare it in `spec.parameters.namespace.labels` of the Project or set it on that namespace directly.

The node-selector and tolerations annotations are the exception.
They belong to the module, so adoption copies them into the `namespace.annotations` parameter of the project, where they are changed or removed.
The pod security, monitoring and scanning labels are turned into parameters of the project, and the additional namespaces inherit them. The `extended-monitoring.deckhouse.io/enabled` and `security-scanning.deckhouse.io/enabled` labels keep the value `"true"` if the namespace had it. Any other value, `"false"` included, is replaced with an empty one, because adoption turns the feature on whenever the namespace carries the label, whatever its value.

The `projects.deckhouse.io/project` label itself is set only by the controller: a namespace created by hand with that label is refused, an empty value included, so a namespace cannot be made to look owned by a project it does not belong to. A namespace belongs to the project its label names and to no other: one without the label, or with an empty value, is not counted towards the project of the same name.
A namespace that already carries the empty value is not adopted either: the empty value used to keep a namespace out of adoption, and it still does until the label is removed.

For example:

1. Create a new namespace:

   ```shell
   d8 k create ns test
   ```

1. Make sure that the project was created:

   ```shell
   d8 k get projects
   ```

   A new project corresponding to the namespace will appear in the project list:

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

## Standard project fields

Project administrators and the resource quota are set in fields of the [Project](cr.html#project) and work with any template (including `simple`):

- `.spec.administrators` — a list of subjects (`kind: User` or `kind: Group` and `name`) that receive administrative access to the project. The controller manages this access as an auto-generated [ProjectRoleBinding](cr.html#projectrolebinding) in the project namespace. A name cannot contain control characters or line breaks and cannot start with `system:multitenancy-manager:`, the prefix of the users that the controller [acts as](#describing-other-objects-in-manifests). An update that keeps such a name as it was is accepted with a warning.
- `.spec.quota` — a map of [ResourceQuota](https://kubernetes.io/docs/concepts/policy/resource-quotas/) hard limits (for example, `requests.cpu`, `limits.memory`) for the whole project.
  The controller turns it into the [ProjectQuota](#project-quota) of the project and reports the usage summed over the namespaces of the project in `.status.usage`; clearing the field deletes that ProjectQuota.
  Changing the field needs the right to write the ProjectQuota of the project, and its keys follow the [rules of a ProjectQuota](#keys-of-a-project-quota).
  For `memory` and `storage`, a unit suffix is required (for example `2Gi`).
  Numbers without a unit mean bytes, and all of them except 0 are rejected. An update that leaves such a value as it was is accepted with a warning, so a project whose quota came from bare numbers in `parameters.resourceQuota` stays editable and can still be applied unchanged. A create through `deckhouse.io/v1alpha2` with such numbers in `parameters.resourceQuota` is accepted with a warning as well, so such a manifest can create the project again, for example from a backup.

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
A Helm `resourcesTemplate` that renders a ResourceQuota or an AuthorizationRule keeps rendering it, next to the ResourceQuota and the ProjectRoleBinding that the standard fields create. A pod in such a project has to fit every ResourceQuota of its namespace, and the access such AuthorizationRules grant adds to the access of the standard fields.
{% endalert %}

## Project quota

A [ProjectQuota](cr.html#projectquota) is one limit on the sum of resource usage over all namespaces of a project, the [additional namespaces](#additional-project-namespaces) included. Its name is the name of the project. [`.spec.quota`](cr.html#project-v1alpha3-spec-quota) of a Project becomes the ProjectQuota of the project; a ProjectQuota can also be created directly:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ProjectQuota
metadata:
  name: my-project
spec:
  hard:
    requests.cpu: "20"
    requests.memory: 40Gi
    projectNamespaces: "3"
```

A ProjectQuota created directly is never taken over by `.spec.quota`: the field does not overwrite or delete it. Setting `.spec.quota` of a project whose ProjectQuota was created directly is refused, and a project that already has both reports `StandardFieldsApplied` as `False`. Delete the ProjectQuota to let the field make its own ProjectQuota object, or clear the field.

How the quota applies:

- The controller creates the `d8-project-quota` ResourceQuota with the limits of `spec.hard` in every namespace of the project, the ones added later included. This ResourceQuota counts the usage of its namespace, so no namespace alone uses more than the whole project may.
- A namespace that the quota applies in carries the `projects.deckhouse.io/quota: enforced` label once its `d8-project-quota` is calculated by kube-controller-manager. A namespace created for a ProjectNamespace carries `projects.deckhouse.io/quota: pending` from the start until then, and the main namespace of a project that has not been deployed yet gets it when the quota of the project is applied, so the manifests of the template wait for the quota too, however long the first attempts to deploy the project fail. The main namespace of a project that has been deployed (in the `Deployed` state), as on an upgrade of the module or when a quota is set on a running project, is not held. The label is set by the controller only.
- In a namespace labelled `projects.deckhouse.io/quota: pending`, the `d8-multitenancy-manager-quota-pending` admission policy refuses the objects of users until `d8-project-quota` is calculated, and the ProjectNamespace reports `Ready=False`.
  That takes seconds after a namespace is created.
  In a namespace labelled `projects.deckhouse.io/quota: enforced`, the `d8-multitenancy-manager-quota-guard` admission policy refuses the objects the quota counts while `d8-project-quota` has no limits calculated at all or has scopes.
  A limit added to a quota that is calculated already does not hold anything back: it applies once kube-controller-manager has calculated it, and `Enforced` is `False` with the `KeyRollout` reason until then.
- A ProjectQuota is deleted in two steps: its `projects.deckhouse.io/project-quota` finalizer holds it until the controller has taken the label off the namespaces, and only then the `d8-project-quota` ResourceQuota objects go with it. A namespace that is being deleted loses the label at once, so that the objects waiting for a finalizer there, such as the volume claims protected from deletion, can be updated. If the module is disabled before the finalizer is removed, remove it by hand.
- The status of the ProjectQuota shows the usage summed over the namespaces (`status.used`), the number of namespaces and the 50 namespaces closest to a limit. The usage of every namespace is in its `d8-project-quota` ResourceQuota:

  ```shell
  d8 k get resourcequota -A -l projects.deckhouse.io/project=my-project
  ```

- In a namespace labelled `projects.deckhouse.io/quota: enforced`, creating an object that takes the usage of the whole project over a limit is denied with HTTP 403:

  ```text
  admission webhook "quota-ledger.multitenancy-manager.deckhouse.io" denied the request: exceeded quota: d8-project-quota, project my-project: requested: requests.cpu=500m, used: requests.cpu=19800m, limited: requests.cpu=20 (3 namespaces)
  ```

  The webhook, called the quota ledger, keeps the sum of every project in the memory of its leader in the `quota-ledger` container of the module pod. While no replica answers (for a few seconds when the leader changes), a counted create in a namespace of a quota project fails with `project quota ledger unavailable (<CAUSE>), retry`; deletions, running objects and namespaces without the quota are not affected. A Deployment or a Job retries such a create by itself. The same answer comes at once, with the `too many approvals of the project are not confirmed yet` cause, for a write that would wait for the earlier approvals of its project to be confirmed while two writes from the same API server already wait: the third waiting write of a project from one API server gets a 503, and its controller retries it.
- The sum is not checked for a request with `--dry-run=server`: the API server does not write the usage of a dry run. The quota of the namespace is still checked.
- The `Enforced` condition says whether the quota applies in every namespace, and the `Exceeded` condition whether the usage is above a limit. A limit may be lowered below the current usage: the change is accepted with a warning, nothing running is evicted, and `Exceeded` is `True` with the `HardLowered` reason. The [`D8ProjectQuotaExceeded`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-d8projectquotaexceeded) alert fires then.

### Keys of a project quota

A ProjectQuota and `.spec.quota` accept the following keys:

- `cpu`, `memory`, `ephemeral-storage`, `pods`, `requests.*` and `limits.*` of CPU, memory and ephemeral storage, `requests.storage`, `requests.hugepages-<SIZE>`, the requests of extended resources (for example, `requests.nvidia.com/gpu`);
- `services`, `services.loadbalancers`, `services.nodeports`, `persistentvolumeclaims`, `configmaps`, `secrets`, `replicationcontrollers`;
- `<CLASS>.storageclass.storage.k8s.io/requests.storage` and `<CLASS>.storageclass.storage.k8s.io/persistentvolumeclaims`, `<CLASS>.deviceclass.resource.k8s.io/devices`;
- `count/<RESOURCE>[.<GROUP>]` for a namespaced resource the API server serves, for example `count/ingresses.networking.k8s.io`;
- `projectNamespaces`, an alias of `count/projectnamespaces.deckhouse.io`: the number of [ProjectNamespace](cr.html#projectnamespace) objects of the project, that is, of its additional namespaces. The alias is expanded in the ResourceQuota objects, and the object keeps it as written.
- the keys of [virtual machines](#virtual-machines-in-a-project-quota) and of [load balancers](#load-balancers-in-a-project-quota), which no ResourceQuota counts.

Refused are `resourcequotas` and `count/resourcequotas` (`d8-project-quota` takes one in every namespace by itself), the count of a cluster-scoped resource such as `count/namespaces`, a resource misspelled in a served group, a `count/` key of a resource that lacks one of the verbs create, list, watch and delete, and the keys with the reserved `projectquota.deckhouse.io/` prefix.
A `count/` key of a group the API server does not serve yet is accepted with a warning: nothing counts it until the module that provides the group is enabled, and `Enforced` is `False` with the `KeyNotServed` reason until then.
A change that keeps a key the object already has is accepted with a warning even when the key would be refused now.

A new key in a live quota starts to apply once `d8-project-quota` of every namespace has it calculated; until then `Enforced` is `False` with the `KeyRollout` reason.

Pods and volume claims with `resource-quota-overrides.deckhouse.io/*` labels set by platform controllers are counted as a ResourceQuota counts them.

### Virtual machines in a project quota

Where the `virtualization` module is enabled, a project quota limits the virtual machines of the project by their specification. No ResourceQuota counts these keys, so they do not go into `d8-project-quota`: the quota ledger checks every create of a VirtualMachine and every change of its `spec` in the namespaces of a project whose quota has them, and counts the virtual machines itself in these keys:

- `virtualmachines.virtualization.deckhouse.io/cores`: Sum of `spec.cpu.cores`.
- `virtualmachines.virtualization.deckhouse.io/cpu`: Guaranteed CPU, the sum of cores × `spec.cpu.coreFraction`. An empty `spec.cpu.coreFraction` or `Auto` counts as whole cores.
- `virtualmachines.virtualization.deckhouse.io/memory`: Sum of `spec.memory.size`.
- `virtualmachines.virtualization.deckhouse.io/gpus`: Number of GPUs in `spec.gpus`.
- `<GPU_CLASS>.gpuclass.virtualization.deckhouse.io/gpus`: Number of GPUs of the `<GPU_CLASS>` GPUClass.

The number of virtual machines is limited by the ordinary `count/virtualmachines.virtualization.deckhouse.io` key.

Every virtual machine counts in the keys above, a stopped one too: a machine stopped by hand may be started at any moment. Only a machine with `runPolicy: AlwaysOff`, which is kept switched off on purpose, is not counted in them; the `count/virtualmachines.virtualization.deckhouse.io` key counts such machines too.
A change that does not raise a key, such as making a machine smaller or stopping it, is never refused over the quota and passes even while the quota ledger does not answer; every other create and change of `spec` needs the ledger to answer. A change that touches only the metadata of a machine does not call the ledger.
A smaller spec frees the quota only once the machine runs with it: until a restart applies it, the machine counts with the cores and memory it runs with (`status.resources`) and with the GPUs that the changes in `status.restartAwaitingChanges` show. Until the `virtualization` module has processed a change of the spec, the machine keeps what it counted before.
The namespaces of such a project carry the `quota.projects.deckhouse.io/vm: enforced` label, which the controller sets.
While the `virtualization` module is disabled, the keys count nothing, and `Enforced` is `False` with the `KeyNotServed` reason. The quota is accepted with a warning when the API server does not serve VirtualMachine.

The pod of a virtual machine counts in `requests.*` and `limits.*` as well, twice while the machine migrates. A quota that has both the keys of virtual machines and the keys of pods (`cpu`, `memory`, `requests.*`, `limits.*`) is accepted with a warning. Limit virtual machines with their keys and containers with `requests.*` and `limits.*`.

### Load balancers in a project quota

A project quota limits the Services of type LoadBalancer of the project by their class, by their pool and by the external addresses they take.
No ResourceQuota counts these keys, so they do not go into `d8-project-quota`: the quota ledger checks every create of a Service that is or was of type LoadBalancer, and every change of its `spec` or of the `network.deckhouse.io/metal-load-balancer-class`, `network.deckhouse.io/l2-load-balancer-external-ips-count`, `network.deckhouse.io/load-balancer-ips` and `network.deckhouse.io/load-balancer-pool` annotations, in the namespaces of a project whose quota has them, and counts the Services itself in these keys:

- `<CLASS>.loadbalancerclass.network.deckhouse.io/services`: Number of load balancers of the `<CLASS>` class.
- `<CLASS>.loadbalancerclass.network.deckhouse.io/addresses`: External addresses of the `<CLASS>` class.
- `<POOL>.loadbalancerpool.network.deckhouse.io/addresses`: Addresses taken from the `<POOL>` pool of the BGP mode.
- `loadbalancers.network.deckhouse.io/addresses`: All external addresses of the project.

The number of load balancers of any class is limited by the ordinary `services.loadbalancers` key.

The class of a Service is the one the `metallb` module serves it with. It comes from the first source that applies:

- The class the `metallb` module has bound the Service to in its status, even if no such MetalLoadBalancerClass exists
- `spec.loadBalancerClass`, when such a MetalLoadBalancerClass exists
- The `network.deckhouse.io/metal-load-balancer-class` annotation, when such a MetalLoadBalancerClass exists
- The MetalLoadBalancerClass with `isDefault: true`
- `spec.loadBalancerClass` without a check, the class of another load balancer controller
- `default`, the load balancers of a cloud provider

Where the API server does not serve MetalLoadBalancerClass, no class is checked: the class is `spec.loadBalancerClass`, else `default`, and the annotation, which only the `metallb` module reads, does not count.
The quota ledger notices within 30 seconds that the `metallb` module has been enabled or disabled, and counts the load balancers again without a restart.

A Service with the `network.deckhouse.io/load-balancer-pool` annotation takes one address of that pool. Any other Service takes the number of addresses from its `network.deckhouse.io/l2-load-balancer-external-ips-count` annotation when the value is a number above one, and one address when the annotation has any other value. The `network.deckhouse.io/load-balancer-ips` annotation only picks which addresses the Service takes, as the `metallb` module assigns them.

Every Service of type LoadBalancer counts its own addresses, also when it shares an address with other Services under the `network.deckhouse.io/load-balancer-shared-ip-key` annotation: two Services that share one address take two addresses of the quota. The namespaces of such a project carry the `quota.projects.deckhouse.io/lb: enforced` label, which the controller sets.

### Clusters where Deckhouse Platform does not manage the control plane

The sum over the namespaces of a project is enforced only where Deckhouse Platform manages the control plane of the cluster (the `control-plane-manager` module is enabled).
Elsewhere the `d8-multitenancy-manager-quota-pending` and `d8-multitenancy-manager-quota-guard` admission policies are not rendered, every ProjectQuota reports `Enforced=False` with the `ControlPlaneNotManaged` reason, and the [`D8ProjectQuotaNotSupported`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-d8projectquotanotsupported) alert says so.
Each namespace still keeps its `d8-project-quota` ResourceQuota, so no namespace alone uses more than the quota of the whole project.

### Dividing a project quota between namespaces

A project administrator may create ResourceQuota objects of their own in the namespaces of the project. Such a quota only narrows the project quota: an object has to fit every ResourceQuota of its namespace. A quota of a project administrator cannot be named `d8-project-quota` and cannot limit `resourcequotas` or `count/resourcequotas`. A `d8-project-quota` that the controller did not create, or one with scopes, is deleted and created again by the controller.

For example, to keep the `my-project-db` namespace within a quarter of the CPU of the project:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: db-share
  namespace: my-project-db
spec:
  hard:
    requests.cpu: "5"
```

### Who manages the quotas

ProjectQuota objects are created, changed and deleted with the `d8:system-capability:multitenancy-manager:manage_quotas` capability, which the IAM `manager` role and the roles above it include. The same right is needed to change `.spec.quota` of a Project: the Project webhook checks it, so whoever may create and change projects does not choose their quota. The status of a ProjectQuota and the usage in the status of `d8-project-quota` are written by the controller and by Kubernetes only, the latter where Deckhouse Platform manages the control plane.

### When the project quota refuses

What the response says:

| Response | What it means |
|---|---|
| `403 ... exceeded quota: d8-project-quota, project <PROJECT_NAME>: requested: ..., used: ..., limited: ... (<N> namespaces)` | The sum of project `<PROJECT_NAME>` would go over the limits of its ProjectQuota. Raise `spec.hard` or delete what the project does not need; `status.topNamespaces` shows where the usage is. |
| `403 ... exceeded quota: d8-project-quota, requested: ...` (no `project`) | The usage of this namespace alone, with the object, would go over a limit of the project, which every `d8-project-quota` ResourceQuota contains. The quota plugin of the API server refuses it before the sum of the project is checked. |
| `503 ... project quota ledger unavailable (<CAUSE>), retry` | The ledger does not answer for a moment, for example while its leader changes. Retry; if it lasts longer than a minute, see the `D8ProjectQuotaLedgerDown` alert. |
| `500 ... failed calling webhook "quota-ledger.multitenancy-manager.deckhouse.io": ...`, or the same with `quota-vm.multitenancy-manager.deckhouse.io` or `quota-lb.multitenancy-manager.deckhouse.io` | The API server does not reach the ledger: no replica runs, the network, or the certificate. While the Service points at a frozen leader, the quota plugin of the API server retries the call, so each such create waits up to about 8 seconds until the address of the leader is taken out of the Service: in about 3.5 seconds by the other replica, in up to about 15 seconds by Deckhouse when no replica answers. |
| `403 ... ValidatingAdmissionPolicy 'd8-multitenancy-manager-quota-pending' ...` | The `d8-project-quota` of a new namespace is not calculated yet. Retry in a few seconds; if it lasts, see the `Enforced` condition of the ProjectQuota and check the controller and kube-controller-manager. |
| `403 ... ValidatingAdmissionPolicy 'd8-multitenancy-manager-quota-guard' ...` | The `d8-project-quota` of the namespace is missing, has scopes or is not calculated at all yet. Retry in a few seconds; otherwise check the controller and kube-controller-manager. |
| `500 ... resource quota evaluation timed out` | The quota admission of the API server is overloaded; see the `D8ResourceQuotaAdmissionDegraded` alert. |

A new additional namespace refuses the objects of users for the few seconds until its `d8-project-quota` is calculated. A namespace where the quota applies and whose `d8-project-quota` is gone does the same until the controller makes it anew, and refuses the objects of `system:sudouser` too. Of the users, only `system:sudouser` can delete `d8-project-quota`.

If the ledger keeps the objects of projects from being created and the cause cannot be removed quickly, take these steps in order until the creates work again:

1. If the creates fail with `503 ... project quota ledger unavailable`, a replica answers, and suspending the ledger helps. Within a second every running replica lets the writes through uncounted, and every ProjectQuota reports `Enforced=False` with the `Suspended` reason. Deleting the ConfigMap resumes the check; the `D8ProjectQuotaSuspended` alert reminds of it after 15 minutes.

   ```shell
   d8 k -n d8-multitenancy-manager create configmap quota-ledger-suspend --from-literal=reason="<REASON>"
   d8 k -n d8-multitenancy-manager delete configmap quota-ledger-suspend
   ```

   Replace `<REASON>` with why the check is suspended.

1. If the creates fail with `500 ... failed calling webhook "quota-ledger.multitenancy-manager.deckhouse.io"`, the API servers do not reach a replica that answers, as while the `D8ProjectQuotaLedgerDown` alert fires, and the suspension does not help: delete the webhook of the ledger as `system:sudouser`. The check stops within a second, and every ProjectQuota reports `Enforced=False` with the `WebhookMissing` reason. Deckhouse Platform finds the webhook of the module missing and puts it back by itself within about 4 minutes, so this buys time to fix the cause rather than switching the check off.

   ```shell
   d8 k --as=system:sudouser delete validatingwebhookconfiguration d8-multitenancy-manager-quota-ledger
   ```

   Check that the webhook is back: `d8 k get validatingwebhookconfiguration d8-multitenancy-manager-quota-ledger`.

1. Disable the `multitenancy-manager` module. The check goes away together with everything else the module does for projects.

Before draining the worker nodes of a cluster with project quotas (evicting the pods from the nodes with `d8 k drain` or on a node update), check that the ledger has a leader (`d8_multitenancy_quota_ledger_leader`) and that the `D8ProjectQuotaLedgerCanaryFailing` alert is not firing: while the ledger does not answer, the pods that replace the evicted ones are not created in the namespaces of projects with a quota.

## Granting access within a project

To grant access to project namespaces for users beyond the project administrators, use role bindings that reference cluster-wide roles and fan out into the appropriate project namespaces automatically:

- [ProjectRoleBinding](cr.html#projectrolebinding) (namespaced, short name `prb`) — grants a role within a **single** project. It must be created in the project's main namespace (the namespace whose name equals the project name). The controller creates a RoleBinding in every namespace of that project.
- [ClusterProjectRoleBinding](cr.html#clusterprojectrolebinding) (cluster-scoped, short name `cprb`) — grants a role across **all** non-virtual projects. The controller creates a RoleBinding in every namespace of every project and reports the number of bound projects in `.status.boundProjects`.

`roleRef` must reference a ClusterRole whose name starts with one of the allowed prefixes (`d8:project:`, `d8:namespace:`, `d8:project-capability:`, `d8:namespace-capability:`, `d8:custom:`). The roles are described in the [`user-authz` module documentation](/modules/user-authz/).

The following checks apply when bindings are created:

- **Privilege escalation protection**: a binding can only be created by a user who has the right to bind (`bind`) the referenced role, checked by name. A project administrator (`d8:project:admin`) holds `bind` on exactly eight roles: `d8:project:viewer`, `d8:project:user`, `d8:project:manager`, `d8:project:admin`, `d8:namespace:viewer`, `d8:namespace:user`, `d8:namespace:manager` and `d8:namespace:admin`. A binding to any other role — a custom `d8:custom:*` role, a capability, `d8:project:superadmin` — is refused for the project administrator even when the role is narrower than their own permissions, because nothing grants them `bind` on that name. Such bindings are created by a cluster administrator, or the cluster administrator grants `bind` on the custom role to the project administrators with a ClusterRole of their own.
- The role must exist: a binding to a non-existent role is rejected.
- A ServiceAccount used as a subject of a ProjectRoleBinding must belong to a namespace of that same project. Its main namespace, its additional namespaces and a namespace a ProjectNamespace of the project has claimed qualify. A namespace that is only named like an additional namespace (`<project>-*`) but belongs to a separate project does not. When the binding is updated, a ServiceAccount the stored binding already has is not refused again. If its namespace is no longer a namespace of the project, the response carries a warning that this ServiceAccount gets no rights from the binding.
- System and subsystem roles (`d8:system:*`, `d8:subsystem:*`), as well as arbitrary roles outside the listed prefixes, cannot be granted via project bindings.
- Roles with the `rbac.deckhouse.io/disabled-for-direct-use-in-projects: "true"` annotation are forbidden for granting in projects. A cluster administrator can put this annotation on a role to phase it out: existing bindings keep working, but new ones cannot be created. If such a role is used by a project template, the project switches to the `Error` state with an explanation in the `TemplateRolesAllowed` condition.

The controller checks the ServiceAccounts by the same rule every time it updates the RoleBindings of a ProjectRoleBinding, and also when a namespace appears, is deleted or moves to another project. A subject that names ServiceAccounts goes into the RoleBindings only while their namespace is the main namespace of the project, or exists, is not being deleted and carries the `projects.deckhouse.io/project` label with the project name. Besides subjects of the `ServiceAccount` kind, this applies to the `User` subject named `system:serviceaccount:<namespace>:<name>` and the `Group` subject named `system:serviceaccounts:<namespace>`, which RBAC matches to the same ServiceAccounts. Other users and groups are not affected. So a ServiceAccount of a deleted additional namespace gets no rights when a separate project takes the name later.

The left-out subjects stay in `.spec.subjects`. The `SubjectsBound` condition of the binding is `False` and lists them, and the `SubjectsLeftOut` Warning event is sent when the list changes. The `Ready` condition stays `True`, as the binding keeps working for the other subjects. Once the namespace is a namespace of the project again, for example after the deleted ProjectNamespace is created again, the subjects get the rights back without a change to the binding. You can check the condition in the status of the binding:

```shell
d8 k -n my-project get projectrolebinding ci -o jsonpath='{.status.conditions[?(@.type=="SubjectsBound")]}'
```

The `d8-administrators` binding created by the controller from the [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) field is managed by the controller only — it cannot be edited manually. To change the set of administrators, change the `.spec.administrators` field of the project.

The ServiceAccount rule above does not apply to this binding. An administrator given as the `User` `system:serviceaccount:<namespace>:<name>` or the `Group` `system:serviceaccounts:<namespace>` gets the rights in every namespace of the project, whichever project that namespace belongs to, because only those allowed to change the Project set its administrators. Remove such an administrator from the project before its namespace is deleted, as the ServiceAccounts of a namespace created later under the same name would get these rights.

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

Besides the project bindings, a plain RoleBinding can also be used inside a project namespace — the role then applies in that single namespace only. However, in projects the set of roles available to a plain RoleBinding is restricted: only cluster roles carrying the `rbac.deckhouse.io/delegatable: "true"` label are allowed. Among the built-in ones these are the `d8:namespace:*` and `d8:project:*` roles, as well as the access-level roles of the basic role model (`user-authz:user`, `user-authz:privileged-user`, `user-authz:editor`, `user-authz:admin`).

A RoleBinding to any other cluster role (for example, `cluster-admin`, system roles, or capabilities) is rejected in a project with the message `references "<role>" which is not available to project`. This protects the project isolation from being bypassed by binding to an overly broad role.

To use a [custom role](/modules/user-authz/faq.html#creating-a-custom-namespace-or-project-role) in projects, add the `rbac.deckhouse.io/delegatable: "true"` label to it:

```shell
d8 k label clusterrole d8:custom:namespace:developer rbac.deckhouse.io/delegatable=true
```

The restriction applies in the namespaces of every project, including the ones adopted from a standalone namespace.

## Structured templates

A `deckhouse.io/v1alpha2` project template is described by **structured fields**. You declaratively specify which settings the project namespaces get. The controller itself creates the corresponding objects (network policies, security policies, log collection settings, etc.) from these fields in every namespace of the project and keeps them up to date. Other objects of the project namespaces are described in [`manifests`](#describing-other-objects-in-manifests).

Available fields (all optional; the complete reference is [in the ProjectTemplate resource description](cr.html#projecttemplate)):

| Field | What it configures |
|-------|--------------------|
| `podSecurityStandard` | Pod security profile: `Privileged`, `Baseline`, or `Restricted`. A parameter with an empty value sets no profile |
| `networkPolicy.mode` | Network isolation: `Isolated` (traffic is only allowed within the project and from the platform system components) or `NotRestricted` |
| `features.monitoring` | Extended monitoring of the project namespaces |
| `features.vulnerabilityScanning` | Scanning of container images for vulnerabilities |
| `logShipping.clusterDestinationRef` | Collecting the logs of the project pods into the given destination (`ClusterLogDestination`) |
| `nodeSelector`, `tolerations` | Placing the pods of the main namespace on dedicated nodes |
| `allowedUIDs`, `allowedGIDs` | The allowed UID/GID ranges of the project containers |
| `runtimeAudit.enabled` | Auditing the project processes' access to the Linux kernel |
| `namespaceMetadata.labels`, `namespaceMetadata.annotations` | Extra labels and annotations of the project namespaces. The labels the module manages cannot be set here. A template or a Project that sets one is refused. One that already has such a label can still be changed while it keeps the label as it is. The label is not applied, and the response warns about it |
| `resources`, `grantPolicies` | [Granting cluster-wide resources through a project template](#granting-cluster-wide-resources-through-a-project-template) |
| `manifests` | Other objects in the project namespaces, applied with the rights of a project administrator. For details, see [Describing other objects in manifests](#describing-other-objects-in-manifests) |
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

The following fields of the template can take a parameter: `podSecurityStandard`, `networkPolicy.mode`, `features.monitoring`, `features.vulnerabilityScanning`, `logShipping.clusterDestinationRef`, `nodeSelector`, `tolerations`, `allowedUIDs`, `allowedGIDs`, `runtimeAudit.enabled`, `namespaceMetadata.labels` and `namespaceMetadata.annotations`. Instead of a concrete value, specify `{fromParam: <parameter name>}` and declare the parameter in `parametersSchema`. The value does not have to be a scalar: a map (`nodeSelector`, `namespaceMetadata.labels`), a list (`tolerations`) or an object (`allowedUIDs`) works just as well. The other fields — `title`, `description`, `resources`, `grantPolicies` and `parametersSchema` itself — take literal values only. Each project then sets its own value in `.spec.parameters`; if the value is not set, the `default` from the schema is used.

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

A template whose `parametersSchema` still declares the `administrators` and `resourceQuota` parameters keeps accepting projects. A schema declares them by name, or through an `additionalProperties` that takes every parameter it does not name, as long as it takes these two in the form a `v1alpha2` Project holds them. The project parameters are validated against such a schema together with `.spec.administrators` and `.spec.quota`, laid out again as `parameters.administrators` and `parameters.resourceQuota` the way the conversion webhook lays them out for `v1alpha2`. A quota value is checked in the first form the schema takes among its canonical spelling (for example `1Gi` for `1024Mi`), a number (for example `1000` for `1k`) and a spelling with another unit (for example `2000m` for `2` when the schema has `pattern: ^[0-9]+m$`), in that order. A required field that the project leaves empty is checked in its empty form (`[]` for the administrators, `{}` for the quota).
A Helm `resourcesTemplate` and [`manifests`](#describing-other-objects-in-manifests) receive the parameters laid out the same way in `.parameters`, so a text written for `v1alpha2` projects keeps rendering its objects from them. A quota value reaches the text in the form it is checked in, so for a schema that takes strings a zero is the string `"0"`, which `with` and `if` take as set. The structured fields do not receive these values as parameters.

A schema that neither names the `administrators` and `resourceQuota` parameters nor takes unnamed parameters through `additionalProperties` does not declare them, and the schemas of the built-in templates do not declare them either. On a template with such a schema, these parameters grant no access and set no quota. A project that holds them, for example an empty list or a quota with the number `0` that the conversion webhook leaves in `parameters`, keeps working and can be edited.
An update that keeps such a value as it was, including on a move to another template that does not declare them either, is accepted with a warning, and so are an empty list and a quota with no value other than the number `0`. Any other new or changed value is rejected. Set the administrators in `.spec.administrators` and the quota in `.spec.quota` instead. In a `v1alpha2` manifest, write them in a form that the conversion webhook lifts into these fields. Any other parameter that the schema does not declare is still rejected.

The `fromParam` references are checked when a template is created or changed: a reference to an undeclared parameter or to a parameter of an incompatible type (for example, a string parameter for a boolean field) is rejected.

A declared parameter is type-checked even where the schema allows undeclared keys with `additionalProperties` or `x-kubernetes-preserve-unknown-fields`. A reference to an undeclared key is accepted only in such places, and the type of such a key is not checked.

The `requiredRequests` parameter is read by name, without `fromParam`. When the template declares it in `parametersSchema` with `type: boolean` and its value for the project is `true`, the controller renders an OperationPolicy in the `Deny` mode that requires CPU and memory requests from the pods in every namespace of the project. A template that does not declare the parameter renders no such policy, even if its schema allows `additionalProperties` and the project sets `requiredRequests` as an undeclared key. To keep the policy on unless a project turns it off, declare the parameter with `default: true`, as the `default`, `secure` and `secure-with-dedicated-nodes` templates do:

```yaml
spec:
  parametersSchema:
    openAPIV3Schema:
      type: object
      properties:
        requiredRequests:
          type: boolean
          default: true
```

### Template checks

The following rules apply to template operations:

- A template cannot be deleted while at least one project uses it or a project namespace is still rendered from it. After a project switches to another template, the old template stays until the new one has rendered.
- A change to a template is automatically applied to all projects created from it.
- Instead of structured fields, a template can describe its objects with a Helm text in [`resourcesTemplate`](cr.html#projecttemplate-v1alpha2-spec-resourcestemplate). Such a template is rendered through Helm with the project name (`.projectName`), the template name (`.projectTemplateName`) and the project parameters (`.parameters`), and `resourcesTemplate` cannot be set together with the fields that render namespace objects or with `manifests`. A template that projects use keeps a non-empty `resourcesTemplate` until a change sets such fields or `manifests` in its place. What such a change, or a move of a project to another template, does with the objects the Helm text rendered is described in [Switching a project from a Helm text](#switching-a-project-from-a-helm-text). A project whose parameters turn into objects or keys of the rendered manifests instead of values is refused, so quote every substitution.
  This form is deprecated, and `resourcesTemplate` goes away from `deckhouse.io/v1alpha2` together with `deckhouse.io/v1alpha1` in a future release. The `MultitenancyManagerHelmProjectTemplate` alert names every template that still renders from a Helm text, except the built-in ones. Describe new templates with structured fields and `manifests`, and move the existing ones as [Moving a template from a Helm text](#moving-a-template-from-a-helm-text) describes.
- A template cannot get a new `resourcesTemplate` through either version. A create with a non-empty `resourcesTemplate` and an update that sets it on a template whose `resourcesTemplate` is empty are refused. A template that has a Helm text keeps it, and the text can be changed through either version while `deckhouse.io/v1alpha1` can describe the whole template.
  So a template with a Helm text cannot be created again, for example from a manifest exported from another cluster, from a backup, or from Git after the template was deleted. Move such a template to structured fields and `manifests` first.
- The `deckhouse.io/v1alpha1` version of ProjectTemplate is served and deprecated, and the API server returns a warning on every request through it. A template is created through `deckhouse.io/v1alpha2` only, and a create through `deckhouse.io/v1alpha1` is refused. The templates created through `deckhouse.io/v1alpha1` earlier are read, changed and deleted through it as before. This version has the `description`, `parametersSchema` and `resourcesTemplate` fields, and `deckhouse.io/v1alpha2` keeps all three, so a template written as `v1alpha1` reads back unchanged. A change through `v1alpha1` to a template that has fields `v1alpha1` cannot describe, such as `title`, `grantPolicies`, `podSecurityStandard` or `manifests`, is refused, because it would erase them. Change such a template through `deckhouse.io/v1alpha2`.

### Describing other objects in manifests

The structured fields cover the settings that most projects need.
Other objects of the project namespaces, such as workloads, ConfigMap, RoleBinding or LimitRange objects, are described in [`manifests`](cr.html#projecttemplate-v1alpha2-spec-manifests).
It is a text in the [`go template`](https://pkg.go.dev/text/template) format with the [`helm` functions](https://helm.sh/docs/chart_template_guide/function_list/), like `resourcesTemplate`, and a template can set it together with the structured fields.

`manifests` and `resourcesTemplate` differ in whose rights create the objects.
The controller installs a Helm text from `resourcesTemplate` as a Helm release with its own rights, which cover the whole cluster.
So such a text can create any cluster-scoped object other than a namespace, and any object in the namespaces of the project.
The controller applies the objects of `manifests` as a user of the project that holds the `d8:project:admin` role in the namespaces of the project.
A template thus gets the rights of an administrator of the project, not the rights of the controller.

The text gets the project name in `.projectName`, the template name in `.projectTemplateName` and the project parameters completed with the defaults of `parametersSchema` in `.parameters`, the same values that `resourcesTemplate` gets.
A project whose parameters turn into objects or keys of the rendered manifests instead of values is refused, so quote every substitution.
The text is rendered without access to the cluster, so the `lookup` function finds nothing.
A project template can be read by users who have no access to its projects, the ones with the `User` access level included, so do not put secret values into `manifests`.

The objects are applied as follows:

- The controller creates the `d8-manifests` ProjectRoleBinding in the main namespace of the project. It grants the `d8:project:admin` role to the `system:multitenancy-manager:project:<PROJECT_NAME>` user in every namespace of the project.
- The objects are applied once this binding is `Ready` for its current generation and the `d8:project:admin` ClusterRole has its aggregated rules. Until then the `ManifestsApplied` condition of the project says what the objects wait for, and the project stays `Deployed`.
- Each object is applied with server-side apply as this user, with the `multitenancy-manager/manifests` field manager. An object without `metadata.namespace` goes to the main namespace of the project. An object can also go to an additional namespace of the project, including one that a ProjectNamespace of the same `manifests` makes, as [Additional namespaces in manifests](#additional-namespaces-in-manifests) describes.
- Each object gets the `projects.deckhouse.io/project: <PROJECT_NAME>` and `projects.deckhouse.io/source: manifests` labels, and `.status.manifestObjects` of the project lists the objects.
- An object that exists already is applied only when it belongs to the project, that is, when `.status.manifestObjects` lists it, when a switch from a Helm text left it in `.status.keptObjects`, or when it has both labels. Any other object is left alone, as [Limitations of manifests](#limitations-of-manifests) describes.
- The objects do not get the `heritage: multitenancy-manager` label. They get the `projects.deckhouse.io/source: manifests` label instead, and like the objects of the structured fields and of a Helm text, they are changed through the template only.
  The `d8-multitenancy-manager-manifests-objects` admission policy refuses to create, change or delete such an object to everyone, project and cluster administrators included.
  Only the user the `manifests` of the project are applied as, the controller, the service accounts of the `d8-*` namespaces and the cluster components can do it.
  Anyone who may update a workload may restart its rollout, when the `kubectl.kubernetes.io/restartedAt` annotation of its pod template is the only change.
  `metadata.resourceVersion`, `metadata.generation` and `metadata.managedFields` are not compared, because the API server sets them on the write.
  Within 10 minutes the controller applies the objects again, which puts back the fields that the template sets if something else changed them.
- An object that the template no longer renders is deleted as the same user after the other objects are applied, and only when none of them was refused or left alone. It is deleted through the version that the API server serves its kind at, also when it was applied through another one. An object that loses one of the two labels, and an object whose namespace left the project, leave the list and stay in place. Only the service accounts of the `d8-*` namespaces, the controller among them, and the cluster components can take a label off. The user the `manifests` are applied as can take off only the `projects.deckhouse.io/source` label.
- When one change makes the project release render an object that `manifests` applied, under the same kind, namespace and name, the release takes the object over, and the object stays in the cluster. Until the upgrade finishes, `.status.handingOver` lists the object and `.status.manifestObjects` does not. If the upgrade does not deploy the object, it goes back to `manifests`, and `.status.manifestObjects` lists it again.
- When the same change makes the release render another object that exists without the ownership marks of the release and is not an object of `manifests`, Helm cannot take that object over. The ownership marks are the `app.kubernetes.io/managed-by: Helm` label and the `meta.helm.sh/release-name` annotation with the name of the project release. The controller does not run the upgrade then and leaves the objects of `manifests` unchanged, and the `ResourcesUpgraded` condition of the project names the object that blocks the upgrade. The handover goes on once that object is deleted or gets the ownership marks.
- When the template drops `manifests` or the project moves to a template without it, the objects are deleted, and then the `d8-manifests` binding is deleted. When the project is deleted, the binding is deleted first, and the objects go with the namespaces.

Only the controller binds the users whose names start with `system:multitenancy-manager:`.
A RoleBinding, ClusterRoleBinding, ProjectRoleBinding, ClusterProjectRoleBinding, AuthorizationRule or ClusterAuthorizationRule that names such a user or group is refused for everyone else, except the garbage collector and `system:sudouser` (see [Who bypasses admission](#who-bypasses-admission)).
A Helm text in `resourcesTemplate` can also bind such a user, because the controller installs it with its own rights.
Apart from that, the objects of a template get the rights of the `d8-manifests` binding and the rights that the cluster grants to every authenticated user (the `system:authenticated` group).
For the same reason a project cannot name such a user or group among its administrators, in `.spec.administrators` or in the `administrators` parameter. A name that the project already has is accepted with a warning, so the project stays editable and the name can be removed.

The following template sets the pod security profile and the network isolation with structured fields and creates two objects in `manifests`, a LimitRange with the default limits from the project parameters and a ConfigMap with the names of the project and of its template:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: team-with-limits
spec:
  podSecurityStandard: Baseline
  networkPolicy:
    mode: Isolated
  parametersSchema:
    openAPIV3Schema:
      type: object
      properties:
        defaultCPU:
          type: string
          default: 500m
        defaultMemory:
          type: string
          default: 512Mi
  manifests: |
    apiVersion: v1
    kind: LimitRange
    metadata:
      name: container-defaults
    spec:
      limits:
        - type: Container
          default:
            cpu: {{ .parameters.defaultCPU | quote }}
            memory: {{ .parameters.defaultMemory | quote }}
    ---
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: project-info
    data:
      project: {{ .projectName | quote }}
      template: {{ .projectTemplateName | quote }}
```

{% endraw %}

Once the `ManifestsApplied` condition is `True`, the main namespace of every project of the template has the `container-defaults` LimitRange and the `project-info` ConfigMap.
To check the condition and to list the applied objects, use the commands:

```shell
d8 k get project <PROJECT_NAME> -o jsonpath='{.status.conditions[?(@.type=="ManifestsApplied")]}'
d8 k get project <PROJECT_NAME> -o jsonpath='{range .status.manifestObjects[*]}{.kind} {.namespace}/{.name}{"\n"}{end}'
```

#### Additional namespaces in manifests

`manifests` can make additional namespaces of the project with [ProjectNamespace](cr.html#projectnamespace) objects and put objects into them in the same text. A ProjectNamespace without `metadata.namespace` goes to the main namespace of the project, and its namespace is named `<PROJECT_NAME>-<spec.name>`, so an object in it names the namespace as {% raw %}`{{ .projectName }}-<spec.name>`{% endraw %}.

The namespace does not exist on the first render, so the objects are applied in two steps:

- The ProjectNamespace objects and the objects of the namespaces that exist are applied at once.
- The objects of a namespace that a ProjectNamespace of the same `manifests` makes wait until the namespace exists and the `d8-manifests` binding has reached it. Until then the `ManifestsApplied` condition is `False` and names what the objects wait for, and the project stays `Deployed`. A namespace that is being deleted is reported as terminating, and one that is not there yet as still to be created. When the ProjectNamespace is not `Ready`, the condition includes its `Ready=False` message. Objects that wait are kept. Objects that `manifests` no longer renders are deleted. The project is reconciled when the namespace is created and when the binding reaches it, so the objects are applied without any action.
- When `manifests` drops a ProjectNamespace, the ProjectNamespace is deleted, and its namespace goes with everything in it. The objects of `manifests` in that namespace leave `.status.manifestObjects` without a delete of their own.

An object in a namespace that is neither a namespace of the project nor made by a ProjectNamespace of the same `manifests` is refused, as [Limitations of manifests](#limitations-of-manifests) describes. A create or a change of a project whose `manifests` render such an object is admitted with a warning that names the namespace.

The following `manifests` make the `backend` namespace and put a ConfigMap into it:

{% raw %}

```yaml
manifests: |
  apiVersion: deckhouse.io/v1alpha3
  kind: ProjectNamespace
  metadata:
    name: backend
  spec:
    name: backend
  ---
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: backend-settings
    namespace: "{{ .projectName }}-backend"
  data:
    team: {{ .parameters.team | quote }}
```

{% endraw %}

#### Limitations of manifests

`manifests` covers only what an administrator of the project can do in its namespaces.
The following limitations apply:

- A text that does not render for a project, for example because of a syntax error or a `required` value that the project does not set, is accepted when the template is written, because the template is not rendered at that point. Such a project switches to `Error`, `ManifestsApplied` names the error, and a create or a change of the project is refused until the text renders. A change of `manifests` reaches every project of the template at once, so try it first on a copy of the template with one project.
- A cluster-scoped object, an object in a namespace that is neither a namespace of the project nor made by a ProjectNamespace of the same `manifests`, an object with the `heritage: multitenancy-manager` or `heritage: deckhouse` label, an object without `metadata.name` and an object that the text renders twice are refused. While the render holds such an object, none of its objects is applied, the project switches to `Error`, and the `ManifestsApplied` condition names the object.
- An object that exists already and does not belong to the project, for example one that a project administrator created by hand, is not changed, listed or deleted later. The other objects are applied, but nothing is deleted, the project switches to `Error`, `ManifestsApplied` names the object, and the controller looks again every minute. To hand the object over to `manifests`, give it the `projects.deckhouse.io/project: <PROJECT_NAME>` and `projects.deckhouse.io/source: manifests` labels, or delete it so that `manifests` creates it. Only a user who may update objects of its kind in every namespace, such as a cluster administrator, may give it the labels. Do not delete an AuthorizationRule of the `Admin` access level this way, because `manifests` does not create one. Give it the labels instead.
- An object that the `d8:project:admin` role does not allow is refused by the API server with `403 Forbidden`. Examples are a RoleBinding to a role that a project administrator cannot [bind](#granting-access-within-a-project) and an object of a kind that the role does not cover. The other objects are applied, the project switches to `Error`, `ManifestsApplied` names the refused object, and the controller tries again with a growing interval.
- An AuthorizationRule of the `Admin` access level is refused when `manifests` creates it, raises an existing one to this level or adds subjects to an existing one of this level, although the `d8:project:admin` role allows it. The administrators of a project are set in [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) of the Project. An AuthorizationRule of the `User`, `PrivilegedUser` or `Editor` level is applied as other objects are.
  The project switches to `Error` as for an object that the role does not allow, and a create or a change of a project whose `manifests` render a refused AuthorizationRule is admitted with a warning that names it. A create of a template whose `manifests` describe an AuthorizationRule, and a change of such `manifests`, are admitted with a warning about this refusal. An AuthorizationRule that exists already, for example one that a switch from a Helm text left in place, is taken over and updated as other objects are, unless `manifests` raises it to the `Admin` level or adds subjects to it at this level.
- An object of a kind that the API server does not serve keeps all the objects from being applied. The project switches to `Error`, and the controller tries again every minute, for example while the module that brings the kind is being enabled.
- `manifests` cannot be set together with `resourcesTemplate`.
- `deckhouse.io/v1alpha1` has no place for `manifests`, so a write through `deckhouse.io/v1alpha1` to a template that has it is refused.
- The `projects.deckhouse.io/skip-heritage-label` and `projects.deckhouse.io/unmanaged` labels of a Helm text have no effect on the objects of `manifests`.

`manifests` does not create cluster-scoped objects.
To give the projects of a template access to cluster-wide resources, use [`resources` and `grantPolicies`](#granting-cluster-wide-resources-through-a-project-template).
A cluster administrator creates other cluster-scoped objects outside the template.

### Moving a template from a Helm text

A template with a Helm text in `resourcesTemplate` keeps working, but this form is deprecated.
`resourcesTemplate` goes away from `deckhouse.io/v1alpha2` together with `deckhouse.io/v1alpha1` in a future release, and no template can get a new one (see [Template checks](#template-checks)).
The `MultitenancyManagerHelmProjectTemplate` alert names every such template, except the built-in ones.
Nothing in the cluster rewrites such a template, so move it to structured fields and `manifests` where it is applied from, for example in Git or in CI.

The built-in templates `default`, `secure` and `secure-with-dedicated-nodes` are structured and accept the same parameters as their Helm texts did, except `administrators` and `resourceQuota`, so they show how a text maps to the fields.
A field takes the value of a parameter with `{fromParam: <PARAMETER_NAME>}`, so the projects keep their parameters.
The following table shows where the objects of such a text go:

| Object of the Helm text | Replacement |
|-------------------------|-------------|
| The `security.deckhouse.io/pod-policy` label of the namespace from the `podSecurityProfile` parameter | `podSecurityStandard: {fromParam: podSecurityProfile}` |
| The NetworkPolicy objects of the `networkPolicy` parameter | `networkPolicy.mode: {fromParam: networkPolicy}` |
| The `extended-monitoring.deckhouse.io/enabled` label of the `extendedMonitoringEnabled` parameter | `features.monitoring: {fromParam: extendedMonitoringEnabled}` |
| The `security-scanning.deckhouse.io/enabled` label of the `securityScanningEnabled` parameter | `features.vulnerabilityScanning: {fromParam: securityScanningEnabled}` |
| The PodLoggingConfig of the `clusterLogDestinationName` parameter | `logShipping.clusterDestinationRef: {fromParam: clusterLogDestinationName}` |
| The SecurityPolicy of the `allowedUIDs` and `allowedGIDs` parameters | `allowedUIDs: {fromParam: allowedUIDs}` and `allowedGIDs: {fromParam: allowedGIDs}` |
| The FalcoAuditRules of the `runtimeAuditEnabled` parameter | `runtimeAudit.enabled: {fromParam: runtimeAuditEnabled}` |
| Other labels and annotations of the namespace | `namespaceMetadata.labels` and `namespaceMetadata.annotations`, in the built-in templates from the `namespace.labels` and `namespace.annotations` parameters |
| The placement on dedicated nodes | `nodeSelector` and `tolerations`, in the built-in templates from the `dedicatedNodes.nodeSelector` and `dedicatedNodes.defaultTolerations` parameters |
| The AuthorizationRule objects of the `administrators` parameter | [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) of each Project |
| Other AuthorizationRule objects of the `Admin` access level | [`.spec.administrators`](cr.html#project-v1alpha3-spec-administrators) of each Project, because `manifests` does not create an AuthorizationRule of this level |
| The `all-pods` ResourceQuota of the `resourceQuota` parameter | [`.spec.quota`](cr.html#project-v1alpha3-spec-quota) of each Project |
| The `required-requests-*` OperationPolicy, which refuses pods without CPU and memory requests | The `requiredRequests` parameter, declared with `type: boolean` and `default: true` to keep the policy or with `default: false` to leave it off |
| Any other object in the namespaces of the project, such as a workload, a ConfigMap, a RoleBinding, a LimitRange or an AuthorizationRule of another access level | [`manifests`](#describing-other-objects-in-manifests) |
| A cluster-scoped object | No replacement in a template. Access to cluster-wide resources is set with [`resources` and `grantPolicies`](#granting-cluster-wide-resources-through-a-project-template) |

For example, an existing template `team` has the following Helm text, which sets the pod security profile from a parameter and creates a LimitRange in the main namespace of each project. Such a template cannot be created again, so the manifest only shows the starting point:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ProjectTemplate
metadata:
  name: team
spec:
  parametersSchema:
    openAPIV3Schema:
      type: object
      properties:
        podSecurityProfile:
          type: string
          enum: [Baseline, Restricted]
          default: Baseline
  resourcesTemplate: |
    apiVersion: v1
    kind: Namespace
    metadata:
      name: {{ .projectName | quote }}
      labels:
        security.deckhouse.io/pod-policy: {{ .parameters.podSecurityProfile | lower | quote }}
    ---
    apiVersion: v1
    kind: LimitRange
    metadata:
      name: container-defaults
      namespace: {{ .projectName | quote }}
    spec:
      limits:
        - type: Container
          default:
            cpu: 500m
            memory: 512Mi
```

{% endraw %}

In the structured form, the profile goes to `podSecurityStandard` and the LimitRange goes to `manifests`.
The text did not render the `required-requests-*` OperationPolicy, so the template declares the `requiredRequests` parameter with `default: false`:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ProjectTemplate
metadata:
  name: team
spec:
  podSecurityStandard:
    fromParam: podSecurityProfile
  parametersSchema:
    openAPIV3Schema:
      type: object
      properties:
        podSecurityProfile:
          type: string
          enum: [Baseline, Restricted]
          default: Baseline
        requiredRequests:
          type: boolean
          default: false
  manifests: |
    apiVersion: v1
    kind: LimitRange
    metadata:
      name: container-defaults
    spec:
      limits:
        - type: Container
          default:
            cpu: 500m
            memory: 512Mi
```

After the switch, the namespace of each project keeps its label, the LimitRange stays in place, and `manifests` takes it over and removes the annotations of Helm from it.

A template that no project uses can be deleted, and the alert goes with it.
If the template is an unchanged copy of a built-in template, move its projects to that template in `.spec.projectTemplateName` instead, and then delete the copy.
To move a template that projects use, follow these steps:

1. Find the projects of the template that still set `administrators` or `resourceQuota` in `.spec.parameters`:

   ```shell
   d8 k get projects -l projects.deckhouse.io/project-template=<PROJECT_TEMPLATE_NAME> -o json | jq -r '.items[] | select((.spec.parameters // {}) | has("administrators") or has("resourceQuota")) | .metadata.name'
   ```

   These values get no standard fields and are not handed over. Move them to `.spec.administrators` and `.spec.quota` first, and only as far as they mean the same there. `.spec.administrators` gives users and groups the `d8:project:admin` role, so an administrator with another access level or another kind of subject needs a ProjectRoleBinding of its own. `.spec.quota` enforces every limit it holds, `0` included, so leave out a limit that the text did not render, such as a `0` that it skipped.

1. Make every tool that writes the template, such as a GitOps tool or a generator, write it as `deckhouse.io/v1alpha2`. Once the template has a field that `deckhouse.io/v1alpha1` cannot describe, such as `manifests`, a write of the template through `deckhouse.io/v1alpha1` is refused, because it would erase that field.

1. Write the template as `deckhouse.io/v1alpha2` with the fields from the table and with `manifests`, without `resourcesTemplate`, in one change. Keep in `parametersSchema` the parameters that the projects set. Declare the `requiredRequests` parameter with `type: boolean`, with `default: true` if the text rendered the `required-requests-*` OperationPolicy and with `default: false` if it did not, so the projects keep refusing or admitting pods without CPU and memory requests as before. A template that projects use cannot lose `resourcesTemplate` without structured fields or `manifests` in its place.

1. Check each project after the switch. Its `HandoverPending` condition is removed once the replaced objects are deleted, its `ManifestsApplied` condition says whether the objects of `manifests` are applied, and `.status.keptObjects` lists the objects left in place:

   ```shell
   d8 k get project <PROJECT_NAME> -o jsonpath='{range .status.conditions[*]}{.type}={.status}: {.message}{"\n"}{end}{range .status.keptObjects[*]}{.kind} {.namespace}/{.name}{"\n"}{end}'
   ```

1. Decide on each object that stays in `.status.keptObjects`, as [Switching a project from a Helm text](#switching-a-project-from-a-helm-text) describes. An object that `manifests` renders under the same kind, namespace and name leaves the list once `manifests` takes it over, and from then on it has what `manifests` renders.

### Switching a project from a Helm text

A project release rendered from a Helm text in `resourcesTemplate` holds whatever the text rendered. The project switches to structured fields when its template gets structured fields or `manifests` in place of `resourcesTemplate`, when the project moves to a structured template, and when a built-in template that was a Helm text is rewritten with structured fields. The release then holds only what the structured fields render. An object of the Helm text that the structured fields render under the same kind and name stays in the release and is updated in place. The other objects of the Helm text are handled as follows, and of them only an AuthorizationRule that names nobody from `.spec.administrators` any more is deleted at once.

The AuthorizationRule and ResourceQuota objects that the [standard fields](#standard-project-fields) replace go through a handover, so the administrators never lose access and the namespace never loses its quota. These are the AuthorizationRule objects the release created that grant the `Admin` access level and nothing else to users and groups who are all administrators of the project, and the ResourceQuota objects of the main namespace without scopes whose every limit `.spec.quota` sets to the same value or a lower one. The handover goes in the following order:

1. The Project gets the `HandoverPending` condition set to `True`. Then the objects get the `helm.sh/resource-policy: keep` annotation and the `projects.deckhouse.io/kept-by-handover: "true"` annotation, so the release upgrade does not delete them.
1. The `d8-administrators` ProjectRoleBinding and the `d8-project-quota` ResourceQuota are created before the release is upgraded.
1. The kept objects are deleted once `d8-administrators` names the administrators from `.spec.administrators` and is `Ready` for its current `metadata.generation`, the `d8:project:admin` ClusterRole has its aggregated rules, and `d8-project-quota` has the limits from `.spec.quota` and is calculated, and not earlier than 2 minutes after `d8-administrators` became `Ready` and after the handover began. Until then the administrators hold both roles, and the tighter of the quotas applies. The `HandoverPending` condition says what the handover waits for, and it is removed once the kept objects are deleted.

A kept AuthorizationRule that names nobody from `.spec.administrators` any more grants only what was revoked, and it is deleted at once. A subject removed from `.spec.administrators` is removed at once from a kept AuthorizationRule that names other administrators too. A kept quota that `.spec.quota` no longer covers when the handover finishes, because a limit in `.spec.quota` was raised above it or removed, is left in place like the objects below. The `HandoverPending` condition is kept in the status of the Project, so a replace of the Project (for example, `kubectl replace` or Argo CD with the `Replace=true` option) does not lose it. If the kept objects are still there after 30 minutes, the `MultitenancyManagerProjectHandoverStuck` alert names the project and says what to check.

Every other object of the Helm text that the structured fields do not render is left in place instead of being deleted. Examples are AuthorizationRule objects of other subjects or of another access level, such as the ones a text renders for roles from its own parameters, a quota that `.spec.quota` does not cover, RoleBinding objects, network policies, and workloads with their volumes.

- Such an object keeps working as before. A rule or a binding keeps granting access, a quota keeps applying, a workload keeps running.
- The project no longer manages it. The object gets the `helm.sh/resource-policy: keep` annotation and the `projects.deckhouse.io/kept-from-helm-template: "true"` label, and it loses the `heritage: multitenancy-manager` label. While it carries the label, only a requester who may make the same change to objects of its kind across the cluster may change or delete it, for example a cluster administrator. The users of the project may not, so a network policy or a quota of the text keeps holding them. The protection does not cover the `scale` and `status` subresources: whoever may scale objects of the kind or change their status in the namespace may still do so for a kept object, which matters only for a kept workload.
- The Project lists it in `.status.keptObjects`, and the `MultitenancyManagerProjectKeepsHelmTemplateObjects` alert names the project. An object leaves the list once it is deleted or the label is removed from it. When the template renders an object of the same kind and name again, the project release takes the object over. The controller then removes the label, puts the `heritage: multitenancy-manager` label back and removes the `helm.sh/resource-policy: keep` annotation, unless the template sets it itself.
- When [`manifests`](#describing-other-objects-in-manifests) of the template renders an object of the same kind, namespace and name, the controller applies it as the user of the project and takes the object over. The object then has what `manifests` renders. A field that the Helm text set and `manifests` does not set is removed, the annotations of Helm included, such as `helm.sh/resource-policy: keep` and `meta.helm.sh/*`, and so is the `projects.deckhouse.io/kept-from-helm-template` label. A field that somebody else set on the object stays. The object leaves `.status.keptObjects`.
- When the project is deleted, the object goes with it. An object in a namespace of the project goes with the namespace, and the controller deletes a cluster-scoped object that still carries the label.

Describe what is still needed in the template or in the project, for example with a ProjectRoleBinding or with a limit in `.spec.quota`, and delete the left object, or remove the label to hand the object over to the project, after which the users of the project may change and delete it as well. An object in a namespace of the project can instead be described in `manifests` of the template, which takes it over. A cluster-scoped object without the label also stays when the project is deleted. An access that a left object grants ends only when the object is deleted. To list the left objects of a project, use the command:

```shell
d8 k get project <PROJECT_NAME> -o jsonpath='{range .status.keptObjects[*]}{.kind} {.namespace}/{.name}{"\n"}{end}'
```

An edit of a Helm text, and a move of a project to another template with a Helm text, delete the objects the new text does not render, except the AuthorizationRule and ResourceQuota objects that the standard fields replace. These go through the same handover.

Administrators and a quota that are still set in `.spec.parameters`, because they could not be moved to `.spec.administrators` and `.spec.quota`, get no standard fields. Their AuthorizationRule objects and their quota are not handed over, and a switch leaves them in place. Move such values to the standard fields before the switch.

After the handover the administrators have the `d8:project:admin` role in place of the `Admin` access level. The two roles do not grant the same permissions. In particular, the access that custom ClusterRoles with the `user-authz.deckhouse.io/access-level` annotation added to the `Admin` access level is not part of `d8:project:admin`. To grant such access, create a [custom role](/modules/user-authz/faq.html#creating-a-custom-namespace-or-project-role) and bind it with a ProjectRoleBinding.

### Moving an object of the release to manifests

A structured template renders its objects into the project release. When one change makes [`manifests`](#describing-other-objects-in-manifests) render an object the release contained, under the same kind, namespace and name, the upgrade would delete the object and `manifests` would create it again. The project would be without the object in between. An example is the NetworkPolicy named `isolated` once `networkPolicy.mode` is removed and `manifests` render that NetworkPolicy.

The controller leaves the object in place instead. Before the upgrade it sets the `helm.sh/resource-policy: keep` annotation and the `projects.deckhouse.io/kept-from-helm-template: "true"` label, and lists the object in `.status.keptObjects`, the same way a [switch from a Helm text](#switching-a-project-from-a-helm-text) leaves an object. In the same reconcile `manifests` apply the object as the user of the project, remove the label and take the object off the list. The object then has what `manifests` render. A field that the release set and `manifests` do not set is removed. A field that somebody else set stays.

If `manifests` cannot apply the object, for example because the API server refuses the kind or returns `403 Forbidden`, the object stays in `.status.keptObjects`. The `MultitenancyManagerProjectKeepsHelmTemplateObjects` alert names the project. The object keeps working, and only a requester who may make the same change to objects of its kind across the cluster may change or delete it, until `manifests` take it over or the label is removed.

An object the release drops and that `manifests` do not render is deleted by the upgrade.

## Creating your own project template

To create your own template:

1. Take one of the default templates as a basis, for example, `default`.
1. Copy it to a separate file, for example, `my-project-template.yaml` using the command:

   ```shell
   d8 k get projecttemplates default -o yaml > my-project-template.yaml
   ```

1. Edit the `my-project-template.yaml` file and adjust the [structured fields](#structured-templates), the objects in [`manifests`](#describing-other-objects-in-manifests) and the input parameters schema to your needs.
1. Change the template name in the `.metadata.name` field.
1. Remove the `heritage` label, the `metadata.resourceVersion`, `metadata.uid`, `metadata.creationTimestamp`, `metadata.generation` and `metadata.managedFields` fields and the `status` section from the file.
1. Apply your new template with the command:

   ```shell
   d8 k apply -f my-project-template.yaml
   ```

1. Check the availability of the new template with the command:

   ```shell
   d8 k get projecttemplates <NEW_TEMPLATE_NAME>
   ```

## Labels for the objects of a Helm text

The following labels change how the module handles the objects of a Helm text in `resourcesTemplate`. Only existing templates have such a text, because no template can get a new `resourcesTemplate`. The objects of [`manifests`](#describing-other-objects-in-manifests) never get the `heritage: multitenancy-manager` label, and the module does not read these labels on them.

### Skipping the heritage label

By default, every object of the Helm text gets the `heritage: multitenancy-manager` label. With this label, only the identities listed in [Who bypasses admission](#who-bypasses-admission) can change the object.
To allow changes to the object (for example, for compatibility with other systems, or to control the created objects yourself), add the `projects.deckhouse.io/skip-heritage-label` label to it.

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

In this case, the object gets the `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` labels, but not the `heritage: multitenancy-manager` label.

### Excluding objects from management

To exclude an object from management by the module (for example, when it is managed manually or by another controller), add the `projects.deckhouse.io/unmanaged` label to it.

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

Objects with the `projects.deckhouse.io/unmanaged` label:

- are created **only once**, when the project release is first installed, so an object added to the template later is not created in existing projects;
- **are not updated** when the template changes;
- are not tracked in the project status;
- get the `projects.deckhouse.io/project` and `projects.deckhouse.io/project-template` labels, but **not** the `heritage: multitenancy-manager` label;
- get the `helm.sh/resource-policy: keep` annotation, so the project release does not delete them.

{% alert level="warning" %}
After it is created, an `unmanaged` object does not depend on the template and must be managed manually.
{% endalert %}

## Implementing validation of object changes with a custom label

The `multitenancy-manager` module uses a ValidatingAdmissionPolicy to protect objects labeled `heritage: multitenancy-manager` from manual changes.
You can implement similar validation for objects with any label.

### How validation works in multitenancy-manager

Validation occurs for objects labeled `heritage: multitenancy-manager`.
The following components are used for this:

1. ValidatingAdmissionPolicy: Defines validation rules:
   - Operations: `CREATE`, `UPDATE` and `DELETE`. `system:masters` is not exempt.
   - Check: only the module's controller and the identities this policy lets through (for details, see [Who bypasses admission](#who-bypasses-admission)) may change such an object. For a project namespace, an update that changes only labels and annotations outside the keys the module owns is allowed as well (see [Creating a project automatically for a namespace](#creating-a-project-automatically-for-a-namespace)).
   - Applies to all resources and API groups.
1. ValidatingAdmissionPolicyBinding: Defines which objects the validation applies to:
   - Uses `namespaceSelector` and `objectSelector` to select resources by the label `heritage: multitenancy-manager`.

### Who bypasses admission

The webhooks and policies of the module have different exclusion lists:

- The **Project and ProjectTemplate webhooks** skip requests from platform components — the API server (`system:apiserver`), the service accounts of Deckhouse (`d8-system:deckhouse`), of this module (`d8-multitenancy-manager:multitenancy-manager`) and of `user-authz` (`d8-user-authz:controller`), every service account of the `d8-system`, `kube-system` and `d8-user-authz` namespaces, kubelets (`system:nodes`) — and from the `system:sudouser` identity. Everyone else is validated, `system:masters` included.
- The **ProjectRoleBinding, ProjectNamespace and ClusterProjectRoleBinding webhooks** skip nobody and validate every request. Inside them only the controller and Deckhouse service accounts are privileged; `system:sudouser` and `system:masters` are ordinary users there.
- The **ValidatingAdmissionPolicy** above lets through this module's controller; the users `system:apiserver`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability` and `system:sudouser`; the groups `system:nodes`, `system:serviceaccounts:kube-system` and `system:serviceaccounts:d8-system` (not `d8-user-authz`); and a rollout restart of a project workload that changes only the `kubectl.kubernetes.io/restartedAt` annotation of the pod template. Everyone else is refused, `system:masters` included.
- The **`d8-multitenancy-manager-manifests-objects` ValidatingAdmissionPolicy** guards objects labelled `projects.deckhouse.io/source: manifests`.
  It lets through this module's controller; the users `system:apiserver`, `system:kube-controller-manager`, `system:kube-scheduler`, `system:volume-scheduler`, `dhctl`, `observability` and `system:sudouser`; and the groups `system:nodes`, `system:serviceaccounts:kube-system` and `system:serviceaccounts:d8-system`.
  Every service account whose name starts with `system:serviceaccount:d8-` may create, change and delete such objects.
  The user `system:multitenancy-manager:project:<PROJECT_NAME>` may create, change and delete the objects of that project.
  A rollout restart that changes only the `kubectl.kubernetes.io/restartedAt` annotation of the pod template is allowed.
  An update that puts the `projects.deckhouse.io/source: manifests` label, together with the `projects.deckhouse.io/project` label, on an object that does not have the `projects.deckhouse.io/source: manifests` label yet is allowed to a requester who may update that resource across the cluster.
  Everyone else is refused, `system:masters` included.
- The **ValidatingAdmissionPolicy that reserves the users the controller acts as** (the names that start with `system:multitenancy-manager:`, see [Describing other objects in manifests](#describing-other-objects-in-manifests)) lets through this module's controller, the garbage collector, which runs as `system:serviceaccount:kube-system:generic-garbage-collector` or as `system:kube-controller-manager`, and `system:sudouser`. Everyone else who names such a user or group in a binding is refused, the other service accounts of `kube-system` and `system:masters` included.
- The **`d8-multitenancy-manager-namespace-creation` ValidatingAdmissionPolicy** checks namespace creation and is in force only while [`allowNamespacesWithoutProjects`](configuration.html#parameters-allownamespaceswithoutprojects) is `false`. It lets through the API server (`system:apiserver`), the service accounts of Deckhouse (`d8-system:deckhouse`), of this module (`d8-multitenancy-manager:multitenancy-manager`) and of the `upmeter` agent (`d8-upmeter:upmeter-agent`), every service account of the `d8-system` and `kube-system` namespaces, the groups `system:masters` and `system:nodes`, and `system:sudouser`. Unlike the lists above, it includes `system:masters`, so cluster administrators still create namespaces directly. Everyone else can create only the `default` namespace.
- The **`d8-multitenancy-manager-namespace-adoption` ValidatingAdmissionPolicy** refuses to create a namespace whose name is longer than 61 characters, starts with `upmeter-` or is `deckhouse`, and to put the `heritage: upmeter` label on a namespace, whatever `allowNamespacesWithoutProjects` says. It lets through the same users and groups as the `d8-multitenancy-manager-namespace-creation` policy.
- The **`d8-multitenancy-manager-manifests` ValidatingAdmissionPolicy** checks only the users that the controller applies `manifests` as, whose names start with `system:multitenancy-manager:project:`, and refuses them to create an AuthorizationRule of the `Admin` access level, to raise one to this level or to add subjects to one of this level. Nobody else is checked by it.
- The **`d8-multitenancy-manager-v1alpha1-template-create` ValidatingAdmissionPolicy** refuses every create of a ProjectTemplate through `deckhouse.io/v1alpha1` and lets nobody through, `system:sudouser`, `system:masters` and the platform components included. The refusal of a new `resourcesTemplate` through `deckhouse.io/v1alpha2` belongs to the ProjectTemplate webhook, so the requesters that the webhook skips, `system:sudouser` among them, can still create a template with a Helm text that way.
- The **`d8-multitenancy-manager-namespace-ownership` ValidatingAdmissionPolicy** keeps the `projects.deckhouse.io/project` label and the labels and annotations that the module sets on a project namespace. It lets through only this module's controller and `d8-system:deckhouse`. Everyone else is refused, `system:sudouser` and `system:masters` included.
- The **`d8-multitenancy-manager-project-grants` ValidatingAdmissionPolicy** keeps the ClusterResourceGrantPolicy objects that the controller makes from `spec.grants` of a Project.
  It refuses to create an object whose name starts with `project-`, and to create, change or delete an object that has the `multitenancy.deckhouse.io/managed-by-project` label.
  It lets through this module's controller, `d8-system:deckhouse`, `system:sudouser` and the service accounts of the `kube-system` namespace, the garbage collector among them.
  Everyone else is refused, `system:masters` included.
- The **`d8-multitenancy-manager-kept-objects` ValidatingAdmissionPolicy** guards the objects listed in `.status.keptObjects` of a project. It lets through this module's controller, the users whose names start with `system:multitenancy-manager:`, `system:sudouser` and the system users and groups of the policy that guards the `heritage: multitenancy-manager` objects. Anyone else may change such an object only with the right to make the same change across the cluster.

The `/is-granted` and `/defaults` webhooks, which check writes of objects that reference cluster-wide resources in project namespaces (see [Managing access to cluster-wide resources](#managing-access-to-cluster-wide-resources)), skip `system:masters` together with the platform components. The `/protect` webhook keeps AvailableClusterResource read-only. It lets through the controller, the platform components and `system:masters`.

An administrator who needs to bypass the Project or ProjectTemplate webhook, the policy that guards the `heritage: multitenancy-manager` objects, the `d8-multitenancy-manager-manifests-objects` policy, or the policy that reserves the users the controller acts as, does it deliberately as `system:sudouser`, for example with `d8 k --as system:sudouser …`, and the bypass is then visible in the audit log. There is no similar option for ProjectRoleBinding, ProjectNamespace and ClusterProjectRoleBinding, for a create of a ProjectTemplate through `deckhouse.io/v1alpha1`, or for the namespace labels and annotations that the `d8-multitenancy-manager-namespace-ownership` policy keeps.

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

The module allows you to manage project access to cluster-wide resources such as StorageClass, ClusterIssuer, ClusterRole, and others.

For a description of the mechanism, the resources it uses, and the cluster-wide resources registered by the platform, refer to the [module description](./#managing-access-to-cluster-wide-resources).

The following sections provide common scenarios for configuring and using the mechanism.

### How a policy selects namespaces

A [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy) (`multitenancy.deckhouse.io/v1alpha2`) has two selectors, [`projectSelector`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-projectselector) and [`namespaceSelector`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-namespaceselector). The policy applies to a namespace of a project when every selector it sets matches:

- `projectSelector` is matched against the labels of the Project object. It selects every namespace of a project at once, including an additional namespace created later, so use it to select a project as a whole. The module puts the `projects.deckhouse.io/project: <PROJECT_NAME>` and `projects.deckhouse.io/project-template: <PROJECT_TEMPLATE_NAME>` labels on every Project, so a project can also be selected by its name or template. The virtual projects carry only the first of these labels. The two labels cannot be set to another project or template. A manifest that replaces a whole Project may leave them out. The request is admitted with a warning, and the module puts the labels back. The template label of a Project changes as soon as `spec.projectTemplateName` changes. The same label on the namespaces shows the template they were rendered from, and the service policies of a template select namespaces by it.
- `namespaceSelector` is matched against the labels of the Namespace. It narrows the choice to some namespaces of the selected projects, or of all projects when `projectSelector` is not set. For example, it can select only the main namespace (the `projects.deckhouse.io/project-namespace` label does not exist) or only the namespaces labeled by a template.
- A policy with **neither** selector matches no project on its own. Such a policy is a library policy, applied to a project through `grantPolicies` of its ProjectTemplate or of the Project itself (`spec.grants.grantPolicies`). An explicit empty selector (`{}`) matches everything.

The namespaces of the [virtual projects](#virtual-projects) `deckhouse` and `default` are never covered, even by `{}`.

An additional namespace is created by a project user and carries only the labels the module sets and the [labels of the project template](#what-applies-to-the-additional-namespaces). A policy that restricts a resource through `namespaceSelector` may therefore not cover a new additional namespace. This happens when the selector relies on a label set on a namespace by hand, on a template label whose value was changed by hand on the main namespace, or on `projects.deckhouse.io/project-namespace`, whose value is the name the user chooses. If the GrantableClusterResourceDefinition of the resource has [`defaultAvailability: All`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultavailability) (the default), such a namespace gets access to every resource of that type.

The policy reports this in its `NewNamespacesCovered` condition. The condition checks `namespaceSelector` against the labels that a new additional namespace of each covered project would get. Denied names count as a restriction for any `defaultAvailability` value. An allow-list or `availabilityDefault: None` counts only when `defaultAvailability` is `All`.

Do not select namespaces by the `projects.deckhouse.io/project-namespace` label without a `projectSelector`. A user of any project can create a ProjectNamespace with the name that such a `namespaceSelector` expects. The policy then applies in that project and makes the resources it allows available there.

The `multitenancy.deckhouse.io/v1alpha1` version is still served. Its `projectSelector` is matched against the labels of the namespace and corresponds to `namespaceSelector` of `v1alpha2`. `v1alpha1` has no field for the `v1alpha2` `projectSelector`, so a `v1alpha1` client sees it as JSON in the `multitenancy.deckhouse.io/v1alpha2-project-selector` annotation. A policy with this annotation selects projects by it, even though its `v1alpha1` `projectSelector` is empty.

To move a policy manifest from `v1alpha1` to `v1alpha2`, rename its `projectSelector` to `namespaceSelector`. Do not change only `apiVersion`. The same `projectSelector` is then matched against the labels of the Project, so a selector by a label that only the namespace has, such as `kubernetes.io/metadata.name` or a label from `namespaceMetadata`, matches nothing, and the policy silently stops applying to those namespaces.

When you change such a policy through `v1alpha1`, keep the annotation. `d8 k apply` and `d8 k edit` keep it, while `d8 k replace` with a manifest that lacks the annotation removes the `projectSelector`. To return to an old `v1alpha1` manifest after you set the `v1alpha2` `projectSelector`, remove the annotation explicitly, because `d8 k apply` of the old manifest leaves it in place.

### For cluster administrators

The following examples show how to configure project access to cluster-wide resources using ClusterResourceGrantPolicy. They select projects with `projectSelector` by a label on the Project, which covers every namespace of the project.

Because the labels of a Project and of its namespaces take part in the match, the right to change these labels decides which policies apply. The labels of a Project are changed with the `d8:system-capability:multitenancy-manager:edit` capability, which the `iam` subsystem manager has and which also allows creating and changing the policies. The labels of a namespace are changed with the `d8:subsystem-capability:cluster:manage_resources` capability, which the `cluster` subsystem manager has. The project roles `d8:project:*` and `d8:namespace:*` only read namespaces.

#### Restricting StorageClass for a project

To allow projects to use only the `fast-ssd` and `standard` StorageClasses and use `fast-ssd` by default, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: production-storage
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: storageclasses
      default: fast-ssd
      allowed:
        - fast-ssd
        - standard
```

When a PersistentVolumeClaim is created without a value in `spec.storageClassName`, `fast-ssd` is automatically assigned to this field. If a StorageClass that is not in the allowed list is specified, it is replaced with `fast-ssd`, because StorageClass uses the [`Coerce`](cr.html#grantableclusterresourcereference-v1alpha1-spec-fieldpaths-defaulting) default assignment mode. This also covers a class that the built-in Kubernetes admission controller has filled in. The replacement is reported as an admission warning in the response to the request, so the `d8 k` output shows both the original value and the substituted one. If the project has no default StorageClass, such a PersistentVolumeClaim is rejected.

To check which StorageClasses are available to the project, run the following command:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

{% alert level="info" %}
While the [`storage-foundation`](/modules/storage-foundation/) module of version 1.0.3 or later is enabled, the `storageclasses` definition and the `storageclasses-pvc` GrantableClusterResourceReference are registered by that module and can differ from the ones described here, see the documentation of that module.
{% endalert %}

#### Restricting ClusterIssuer for a project

To allow projects to use only the `letsencrypt-prod` and `vault-issuer` ClusterIssuers and use `letsencrypt-prod` by default, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: production-issuers
spec:
  projectSelector:
    matchLabels:
      environment: production
  resources:
    - resourceName: clusterissuers
      default: letsencrypt-prod
      allowed:
        - letsencrypt-prod
        - vault-issuer
```

The policy applies to a ClusterIssuer specified in either of the following ways:

- In the `.spec.issuerRef.name` field of a Certificate if `.spec.issuerRef.kind` is set to ClusterIssuer.
- In the `cert-manager.io/cluster-issuer` annotation of an Ingress.

When a Certificate with a ClusterIssuer is created without a value in `.spec.issuerRef.name`, `letsencrypt-prod` is automatically assigned to this field. If a ClusterIssuer that is not in the allowed list is specified, the Certificate is rejected.

No default value is assigned to the `cert-manager.io/cluster-issuer` annotation. If the annotation is specified, its value is checked against the same list of allowed ClusterIssuers.

{% alert level="info" %}
Access management for ClusterIssuer is available only when the [`cert-manager`](/modules/cert-manager/) module is enabled: the `clusterissuers` definition is shipped by that module.
{% endalert %}

#### Restricting ClusterRoles for a project

By default, a RoleBinding in a project namespace can reference every ClusterRole with the `rbac.deckhouse.io/delegatable: "true"` label, and no other ClusterRole. A policy cannot make a ClusterRole without this label available. To use such a ClusterRole in projects, add the label to it (see [Roles available in a RoleBinding inside a project](#roles-available-in-a-rolebinding-inside-a-project)).

To allow the projects of the `payments` team to use only the `my-custom-role` ClusterRole and the ClusterRoles with the `shared: "true"` label, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: extra-roles
spec:
  projectSelector:
    matchLabels:
      team: payments
  resources:
    - resourceName: clusterroles
      allowed:
        - my-custom-role
      allowedSelector:
        matchLabels:
          shared: "true"
```

In RoleBindings of these projects, only these ClusterRoles can be used, and only the ones with the `rbac.deckhouse.io/delegatable: "true"` label. The other delegatable ClusterRoles, such as `d8:project:*` and `d8:namespace:*`, become unavailable in them, including the RoleBindings that the [`manifests`](#describing-other-objects-in-manifests) of the template create. So list in the policy every role that these projects bind. ProjectRoleBinding and ClusterProjectRoleBinding are not affected, because the controller creates their RoleBindings.

The `rbac.deckhouse.io/delegatable: "true"` label is required on every ClusterRole the policy names: the `clusterroles` registration excludes roles without it from all projects, and an exclusion takes precedence over `allowed` and `allowedSelector`. A policy entry that names a role without the label grants nothing. The controller reports this in the `AllowedEffective` condition of the ClusterResourceGrantPolicy status, with the role name in the message:

```shell
d8 k get clusterresourcegrantpolicy extra-roles -o jsonpath='{.status.conditions[?(@.type=="AllowedEffective")]}'
```

When a RoleBinding is created or modified, the ClusterRole specified in it is checked for availability to the project. No ClusterRole value is assigned automatically.

#### Restricting LoadBalancerClass for a Service

To allow projects to use only the `internal-lb` and `edge-lb` LoadBalancerClass values and use `internal-lb` by default, create the following [ClusterResourceGrantPolicy](cr.html#clusterresourcegrantpolicy):

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: lb-classes
spec:
  projectSelector:
    matchLabels:
      environment: staging
  resources:
    - resourceName: loadbalancerclasses
      default: internal-lb
      allowed:
        - internal-lb
        - edge-lb
```

Unlike StorageClass, ClusterIssuer, and ClusterRole, LoadBalancerClass is not a separate Kubernetes resource. The policy defines the allowed values of the `.spec.loadBalancerClass` field for Services of the `LoadBalancer` type.

When a Service of the `LoadBalancer` type is created without a value in `.spec.loadBalancerClass`, `internal-lb` is automatically assigned to this field. If a value that is not in the allowed list is specified, the Service is rejected.

The policy does not apply to Services of other types.

#### Granting access to all resources of a specific type

To allow specific projects to use all resources of a selected type without listing them explicitly, set [`availabilityDefault: All`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-availabilitydefault).

The following policy allows all projects with the `environment: sandbox` label to use any StorageClass:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: open-storage-for-sandbox
spec:
  projectSelector:
    matchLabels:
      environment: sandbox
  resources:
    - resourceName: storageclasses
      availabilityDefault: All
```

The registered resources are available to all projects by default, so `availabilityDefault: All` changes something only where another policy restricts the resource. There it overrides the `allowed` and `allowedSelector` lists of every policy that applies to the project, the policies of its template included. `denied`, `deniedSelector` and the `excluded` filters of the registration still apply. The resources that match the `reserved` filters of the registration stay unavailable, see ["Granting a reserved resource"](#granting-a-reserved-resource).

Usually, explicitly specifying allowed resources using [`allowed`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowed) or [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowedselector) is sufficient for access management. Use `availabilityDefault: All` when the selected projects need access to all resources of the specified type.

#### Granting a reserved resource

A GrantableClusterResourceDefinition can mark some resources as reserved with [`reserved`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-reserved). A reserved resource is unavailable to projects by default, even when the other resources of its type are available. It becomes available to a project only if a policy that applies to the project lists it in [`allowed`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowed) or matches it with [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowedselector). `availabilityDefault: All` and `defaultAvailability` do not make it available. `denied`, `deniedSelector` and the `excluded` filters take precedence over `allowed` and `allowedSelector` here as well.

An `allowedSelector` opens every reserved resource it matches. A broad or negative selector (`Exists`, `NotIn`, `DoesNotExist`) written for ordinary resources therefore also grants the reserved resources that happen to match it. To grant a reserved resource, name it in `allowed` or use a selector that targets it explicitly.

In the following examples, the `myclusterresources` registration reserves the `premium` and `internal` resources.

To grant a reserved resource without narrowing the other resources of the type, list it in `allowed` and set `availabilityDefault: All`:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: grant-premium
spec:
  projectSelector:
    matchLabels:
      tier: critical
  resources:
    - resourceName: myclusterresources
      allowed:
        - premium
      availabilityDefault: All
```

The selected projects get every resource of the type that is not reserved, and `premium`. The `internal` resource stays unavailable, because `availabilityDefault: All` does not open reserved resources. `availabilityDefault: All` also overrides the `allowed` and `allowedSelector` lists of the other policies that apply to the project (see ["Granting access to all resources of a specific type"](#granting-access-to-all-resources-of-a-specific-type)).

`allowed` without `availabilityDefault` makes every resource not on its list unavailable, as in any other policy. If a project must stay restricted to a list, for example by the policy of its template, grant the reserved resource with `allowed` alone: the `allowed` lists of all policies that apply to the project add up. Alternatively, list in one policy every resource the project needs, the reserved one included.

To make a reserved resource the project default, specify it in both `allowed` and [`default`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-default) of the same entry. A default that the project may not use is ignored. This also applies to a reserved resource marked as the default by `defaultFrom`: it becomes the project default only where a policy allows it.

The table shows what a project gets for typical combinations of policies. The registration reserves `premium` and `internal`, excludes `system` with `excluded`, and has `defaultAvailability: All`. The `premium` resource carries the `tier: critical` label. "Policies of the project" lists the entries of all policies that apply to the project.

| Policies of the project | `standard` | `premium` (reserved) | `internal` (reserved) | `system` (excluded) |
| --- | --- | --- | --- | --- |
| None | available | — | — | — |
| `availabilityDefault: All` | available | — | — | — |
| `allowed: [premium]` | — | available | — | — |
| `allowed: [premium]` and `availabilityDefault: All` | available | available | — | — |
| `allowedSelector` matching `tier: critical` | — | available | — | — |
| `allowed: [standard]` in the template policy and `allowed: [premium]` in another policy | available | available | — | — |
| `allowed: [premium]` and, in another policy, `denied: [premium]` | — | — | — | — |
| `allowed: [system]` | — | — | — | — |
| `allowed: [system]` and `availabilityDefault: All` | available | — | — | — |

#### Denying individual resources

To prevent projects from using individual resources while keeping the others available, use [`denied`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-denied) or [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-deniedselector).

The following policy prevents projects with the `environment: dev` label from using the `expensive-nvme` and `archived-hdd` StorageClasses:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: deny-expensive-storage
spec:
  projectSelector:
    matchLabels:
      environment: dev
  resources:
    - resourceName: storageclasses
      denied:
        - expensive-nvme
        - archived-hdd
```

Deny rules take precedence over allow rules. If a resource matches both `denied` or `deniedSelector` and `allowed` or `allowedSelector`, it is considered unavailable.

#### Managing access using label selectors

To manage access to resources without listing their names, use [`allowedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowedselector) and [`deniedSelector`](cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-deniedselector). Selectors allow or deny resources based on their labels.

The following policy allows projects with the `tier: shared` label to use StorageClasses with the `shared: "true"` label, except for StorageClasses with the `deprecated: "true"` label:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: shared-storage-only
spec:
  projectSelector:
    matchLabels:
      tier: shared
  resources:
    - resourceName: storageclasses
      allowedSelector:
        matchLabels:
          shared: "true"
      deniedSelector:
        matchLabels:
          deprecated: "true"
```

#### Granting cluster-wide resources through a project template

The availability rules for cluster-wide resources can be set directly in a [structured template](#structured-templates) — they then automatically apply to all projects created from that template:

- `spec.resources`: Rules inside the template, in the same format as `resources` in a ClusterResourceGrantPolicy (resource name, `allowed`/`allowedSelector`, `default`).
- `spec.grantPolicies`: List of names of **library** ClusterResourceGrantPolicy objects. A library policy describes a reusable set of rules and sets neither `projectSelector` nor `namespaceSelector`, because the referencing template determines which projects it applies to. This way, for example, a "corporate StorageClasses" policy can be maintained by one administrator and used by several templates.

The following template restricts StorageClass for its projects and connects a library policy:

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
    # A library ClusterResourceGrantPolicy without selectors.
    - corporate-issuers
```

For each source, the controller creates a service policy named `template-<template>-<source>` (for `spec.resources` — `template-<template>-inline`). A template that references a policy named `inline` is rejected, and so is a template whose service policy names would match those of another template. A reference to a non-existent policy or to a policy with either selector is rejected when the template is created or updated. A service policy selects the namespaces of the template's projects with `namespaceSelector` by their `projects.deckhouse.io/project-template` label. The label is set when the template renders, so after a project switches to another template, the rules of the old template apply until the new one has rendered.

#### Granting cluster-wide resources to one project

A project can be given cluster-wide resources of its own, on top of the ones its template grants, in [`spec.grants`](cr.html#project-v1alpha3-spec-grants) of the Project. The field has the format of the template fields: `resources` with the rules themselves and `grantPolicies` with the names of library ClusterResourceGrantPolicy objects.

```yaml
apiVersion: deckhouse.io/v1alpha3
kind: Project
metadata:
  name: my-project
spec:
  projectTemplateName: my-template
  grants:
    resources:
      - resourceName: storageclasses
        allowed: ["fast"]
    grantPolicies:
      - corporate-issuers
```

For each source, the controller creates a service policy named `project-<PROJECT_NAME>-<SOURCE>` (for `resources`, `project-<PROJECT_NAME>-inline`), owned by the project and selecting it with `projectSelector` by its `projects.deckhouse.io/project` label, so the policy applies to every namespace of the project, the ones created later included.
Deleting the project deletes its service policies.
The rules of the project and of its template combine as the rules of any two policies do: the allowed resources add up, and a resource that one of them denies stays unavailable.

The field has these rules:

- Changing `spec.grants` needs the right to create ClusterResourceGrantPolicy objects, the same right as creating such a policy directly. A user who may only change a project cannot grant it cluster-wide resources.
- The references and the names are checked like those of a template.
  A reference to a policy named `inline`, to a missing policy or to a policy with a selector is rejected.
  An empty or invalid `allowedSelector` or `deniedSelector` in `resources` is rejected.
  A `spec.grants` whose service policy names would match a policy that is not the project's, or the service policies of another project, is rejected, and so is a `spec.grants` whose service policy name is not a valid object name, which a very long name of a referenced policy causes.
- If a service policy cannot be applied, or a referenced policy is deleted later, the controller records a Warning event on the Project. The other sources stay applied, and a source removed from `spec.grants` is revoked all the same. A policy that is created again is picked up without waiting.
- The names that start with `project-` are reserved for the service policies, and a service policy is changed only through `spec.grants` of its project.
- `deckhouse.io/v1alpha2` has no place for the field, so a change of a project that has `spec.grants` through that version, a write of its status included, is rejected: it would erase the field. Change such a project through `deckhouse.io/v1alpha3`.

### For project users

Project users can view the cluster-wide resources available to them and check the values used by default.

#### Viewing available cluster-wide resources

An [AvailableClusterResource](cr.html#availableclusterresource) is automatically created in each project namespace for every registered resource. These resources show which cluster-wide resources are available to the project and which resource is used by default.

To view all cluster-wide resources available to a project, run the following command:

```shell
d8 k get available -n <PROJECT_NAME>
```

Example output:

```text
NAME                KIND          DEFAULT      AVAILABLE   AGE
storageclasses      StorageClass  fast-ssd     2           5m
clusterissuers      ClusterIssuer letsencrypt  2           5m
```

To view detailed information about resources of a specific type, such as StorageClass, run the following command:

```shell
d8 k get available storageclasses -n <PROJECT_NAME> -o yaml
```

Example output (abridged):

```text
status:
  grantedResourceKind: StorageClass
  default: fast-ssd
  availableCount: 2
  available:
    - name: fast-ssd
      default: true
      fields:
        provisioner: rbd.csi.ceph.com
        reclaimPolicy: Delete
        volumeBindingMode: WaitForFirstConsumer
        allowVolumeExpansion: true
    - name: hdd
      fields:
        provisioner: local.csi.storage.deckhouse.io
        reclaimPolicy: Retain
        volumeBindingMode: WaitForFirstConsumer
```

The `status.available[].fields` field contains the values of some fields of each available resource, so that you can choose a resource without access to the resource itself. The fields to show are selected by the module that registers the resource, in the [`spec.catalogFields`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-catalogfields) parameter of its GrantableClusterResourceDefinition. For StorageClass, these are `provisioner`, `reclaimPolicy`, `volumeBindingMode` and `allowVolumeExpansion`. A value keeps its JSON type (for example, `allowVolumeExpansion` is a boolean). A field whose value is missing in the resource is not shown: in the example above, `hdd` has no `allowVolumeExpansion`. A change in a cluster-wide resource usually appears in AvailableClusterResource within 30 seconds.

An AvailableClusterResource exists as long as the corresponding GrantableClusterResourceDefinition: when the GrantableClusterResourceDefinition is deleted, its AvailableClusterResource is deleted from the namespaces of all projects.

#### Rejection due to an unavailable cluster-wide resource

If the cluster-wide resource is not available to the project when an object is created, or when an update sets a new value, the request is rejected with the following message:

```text
[multitenancy] <KIND> "<OBJECT_NAME>" references "<RESOURCE_NAME>" which is not available to project "<PROJECT_NAME>". Ask the cluster administrator to grant it.
```

In this case, check the available resources using AvailableClusterResource. Use a resource from the list of available resources or ask the cluster administrator to grant access to the required resource.

#### Assigning default values automatically

For some cluster-wide resources, the administrator can configure a default value. If the corresponding value is not specified when an object is created, it is assigned automatically.

For example, if `fast-ssd` is configured as the default StorageClass, the `fast-ssd` value can be automatically assigned to `.spec.storageClassName` when a PersistentVolumeClaim is created without this field.

You can specify a value explicitly by selecting any resource available to the project from the corresponding AvailableClusterResource.

#### Projects without a default value

AvailableClusterResource reports the project default in `status.default` and marks it among the names in `status.available` with `default: true`. Both are absent in the following cases:

- the grant policy sets no `default`, and the registration has no `defaultFrom`;
- the resource is granted as a list of values rather than as objects of the cluster, and `defaultFrom` does not apply to it;
- `defaultFrom` is set, but not exactly one object has the annotation set to `true`;
- a default is set, but the project may not use it, for example the default StorageClass of the cluster is not among the names granted to the project, or the default is a [reserved](#granting-a-reserved-resource) resource that no policy allows for the project.

Then the field stays empty, and the object is created or rejected by the usual rules.

### For module developers

#### Configuring validation of a cluster-wide resource reference

If a module resource contains a field that references a cluster-wide resource already registered using [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition), create a [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference). It defines which resources and fields use the cluster-wide resource and allows you to configure availability checks and automatic default value assignment.

For example, to configure validation of the StorageClass specified in the `.spec.storageClassName` field of a PostgresDatabase resource, add the following object to the module's Helm chart:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: postgresdatabases-storageclasses
  labels:
    heritage: deckhouse
    module: postgres
spec:
  grantableClusterResourceName: storageclasses
  rule:
    apiGroups: ["postgres.example.com"]
    apiVersions: ["*"]
    resources: ["postgresdatabases"]
  fieldPaths:
    - path: $.spec.storageClassName
      defaulting: Coerce
```

In this example, `storageclasses` is the name of an existing GrantableClusterResourceDefinition, while `Coerce` allows the default StorageClass available to the project to be assigned when a PostgresDatabase is created if the value is missing or unavailable to the project.

If the resources listed in `rule` store the reference at different paths, limit `fieldPaths` items to specific resources using the `resources` parameter. For example, a Job and a CronJob store the PriorityClass name at different paths. In this case, when PriorityClass is registered as `priorityclasses`, use the following example:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceReference
metadata:
  name: jobs-priorityclasses
  labels:
    heritage: deckhouse
    module: my-module
spec:
  grantableClusterResourceName: priorityclasses
  rule:
    apiGroups: ["batch"]
    apiVersions: ["v1"]
    resources: ["jobs", "cronjobs"]
  fieldPaths:
    - resources: ["jobs"]
      path: $.spec.template.spec.priorityClassName
    - resources: ["cronjobs"]
      path: $.spec.jobTemplate.spec.template.spec.priorityClassName
```

For each object, the most specific of the matching `fieldPaths` items is used: an item limited by `resources` takes precedence over items limited only by `apiGroups` or `apiVersions` and over an item without limits, which applies to all other resources from `rule`. `*` in `resources` does not limit the item: it has the same effect as omitting the parameter.

For descriptions of GrantableClusterResourceReference parameters, default assignment modes, `match` conditions, and configuration for resources with multiple API versions, refer to the [resource description](cr.html#grantableclusterresourcereference).

#### Registering a new cluster-wide resource

To add access management for a new cluster-wide resource:

1. Create a [GrantableClusterResourceDefinition](cr.html#grantableclusterresourcedefinition) to register the cluster-wide resource with the access management mechanism.
1. Create one or more [GrantableClusterResourceReference](cr.html#grantableclusterresourcereference) objects to define the fields that reference it.

   For example, to register the MyClusterResource resource, add the following object to the module's Helm chart:

   ```yaml
   apiVersion: multitenancy.deckhouse.io/v1alpha1
   kind: GrantableClusterResourceDefinition
   metadata:
     name: myclusterresources
     labels:
       heritage: deckhouse
       module: my-module
   spec:
     grantedResource:
       apiGroup: my.example.com
       kind: MyClusterResource
     enforcement: Managed
     defaultAvailability: All
     excluded:
       - matchExpressions:
           - key: my.example.com/internal
             operator: Exists
   ```

   In this example, the registered resources are available to projects by default. Resources with the `my.example.com/internal` label are excluded from the available resources.

   To keep some resources unavailable by default but let an administrator grant them to individual projects, list them in [`reserved`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-reserved) instead of `excluded`. An administrator grants such a resource as described in ["Granting a reserved resource"](#granting-a-reserved-resource).

   Only a cluster-scoped resource can be specified in `grantedResource`. A definition of a namespaced resource (for example, Secret) is refused when it is created or updated. If such a definition is stored anyway, it is not processed: no project gets its AvailableClusterResource, and the `GrantedResourceValid` condition is `False` (read more in section ["Checking resource registration status"](#checking-resource-registration-status)). A resource whose CRD is not installed yet is accepted: its scope is checked once the resource is served.

   For descriptions of GrantableClusterResourceDefinition parameters and available access management modes, refer to the [resource description](cr.html#grantableclusterresourcedefinition).

1. After registering the cluster-wide resource, configure references to it using GrantableClusterResourceReference as described in ["Configuring validation of a cluster-wide resource reference"](#configuring-validation-of-a-cluster-wide-resource-reference).

#### Showing resource fields to projects

Project users see the available cluster-wide resources in AvailableClusterResource (read more in section ["Viewing available cluster-wide resources"](#viewing-available-cluster-wide-resources)), but usually cannot read the resources themselves. To help them choose, list the fields to show in [`spec.catalogFields`](cr.html#grantableclusterresourcedefinition-v1alpha1-spec-catalogfields) of the GrantableClusterResourceDefinition. The controller copies the values of these fields from every available resource into `status.available[].fields` of AvailableClusterResource.

For example:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha1
kind: GrantableClusterResourceDefinition
metadata:
  name: myclusterresources
spec:
  grantedResource:
    apiGroup: my.example.com
    kind: MyClusterResource
  catalogFields:
    - name: tier
      path: $.spec.tier
    - name: region
      path: $.metadata.labels['my.example.com/region']
```

Rules:

- `name` is the key in `fields`, in lowerCamelCase; `path` is an RFC 9535 JSONPath singular query that selects one value: member names and array indexes only, without wildcards (`*`), descendant segments (`..`), slices or filters.
- Paths that read `metadata.managedFields` or the `kubectl.kubernetes.io/last-applied-configuration` annotation, or a parent that contains them (for example, `$.metadata`), are not allowed: these fields hold a copy of the whole resource.
- At most 10 fields can be listed.
- A value longer than 512 bytes in JSON serialization is not shown; a field whose value is missing in the resource is not shown either.
- If the `status.available` list of an AvailableClusterResource with the fields of all resources would take more than 512 KiB in JSON serialization, no fields are shown in it; the names and the default stay.
- `catalogFields` works only for definitions with `grantedResource`, which must be cluster-scoped.
- The values are visible to all users of every project the resource is available to. List only non-secret data.

Invalid paths and `catalogFields` without `grantedResource` are refused when the GrantableClusterResourceDefinition is created or updated, and are reported in its `CatalogFieldsValid` condition. Values left out for their size depend on the resources and the project, so they are reported by `CatalogFieldsSkipped` warning events on the GrantableClusterResourceDefinition:

```shell
d8 k describe grantableclusterresourcedefinition myclusterresources
```

#### Using x-deckhouse-grantable-resource in Deckhouse Platform application settings

To manage access to cluster-wide resources in Deckhouse Platform application settings, use the `x-deckhouse-grantable-resource` OpenAPI extension. In this case, deckhouse-controller automatically checks the availability of the specified resource and assigns the default value when necessary. You do not need to create GrantableClusterResourceReference manually.

For a description of the extension and usage examples, refer to ["Application settings"](/products/kubernetes-platform/documentation/v1/architecture/marketplace/settings.html#defaulting-a-grantable-cluster-wide-resource-value-x-deckhouse-grantable-resource).

#### Checking resource registration status

You can check the status of GrantableClusterResourceDefinition and its associated GrantableClusterResourceReference resources in the `status` field:

- [`GrantableClusterResourceDefinition.status.references`](cr.html#grantableclusterresourcedefinition-v1alpha1-status-references): Contains a list of associated GrantableClusterResourceReference resources and information about the resources to which they apply.
- [`GrantableClusterResourceReference.status.bound`](cr.html#grantableclusterresourcereference-v1alpha1-status-bound): Indicates whether the corresponding GrantableClusterResourceDefinition was found.
- `GrantableClusterResourceReference.status.conditions[Bound]`: `True` with the reason `Resolved` if the definition was found, `False` with the reason `UnknownResource` if it is missing. `UnknownResource` can indicate an incorrect GrantableClusterResourceDefinition name or a missing registration.
- `GrantableClusterResourceReference.status.conditions[FieldPathsValid]`: Whether `spec.fieldPaths` pass the checks applied when the object is created or updated (reasons `Valid`, `InvalidFieldPaths`; the message lists all problems). `False` means the object was stored bypassing these checks; the invalid paths are skipped during availability checks.
- `GrantableClusterResourceDefinition.status.conditions[GrantedResourceValid]`: Whether `spec.grantedResource` can be processed. `True` with the reason `ClusterScoped` for a cluster-scoped resource or `ValueBacked` for a definition without `grantedResource`; `False` with the reason `Namespaced` for a namespaced resource, which is not processed; `Unknown` with the reason `KindNotServed` while the resource is not served (for example, its CRD is not installed yet) or `MappingFailed` if its scope could not be determined.
- `GrantableClusterResourceDefinition.status.conditions[CatalogFieldsValid]`: Whether `spec.catalogFields` pass the checks applied when the object is created or updated (reasons `Valid`, `InvalidCatalogFields`; the message lists all problems). A definition without `catalogFields` has the condition `True`. Values left out for their size are reported by `CatalogFieldsSkipped` events (read more in section ["Showing resource fields to projects"](#showing-resource-fields-to-projects)).

For example:

```shell
d8 k get grantableclusterresourcedefinition myclusterresources -o jsonpath='{range .status.conditions[*]}{.type}={.status} ({.reason}): {.message}{"\n"}{end}'
```
