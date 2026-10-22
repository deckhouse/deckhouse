---
title: "Module multitenancy-manager"
search: multitenancy
description: Creating isolated projects for development teams from project templates.
---
The module enables the creation of projects in a Kubernetes cluster. **Project** is an isolated environment where applications can be deployed.

## Main features

- Creating projects from project templates. A template sets the pod security profile, network isolation, log collection, node placement and other policies of the project namespaces.
- Several namespaces in one project. The template settings and the access granted to the project apply to all of them.
- Access to the whole project through `.spec.administrators`, ProjectRoleBinding and ClusterProjectRoleBinding.
- A resource quota for each project.
- A project for every namespace: a namespace created directly becomes a project automatically.
- Control over which cluster-wide resources, such as StorageClass or ClusterIssuer, projects can use.

{% alert level="warning" %}
The [secret-copier](/modules/secret-copier/) module cannot be used together with the `multitenancy-manager` module: it copies Secrets into every namespace, project namespaces included.
{% endalert %}

## Project composition

A project includes:

* **The main namespace** — created automatically, its name matches the project name.
* **Additional namespaces** (optional) — created with the [ProjectNamespace](./cr.html#projectnamespace) resource and named `<project name>-<name>`. The project settings (security policies, network policies, log collection, etc.) and the access granted to the project automatically apply to all of its namespaces.
* **Standard fields** — the list of administrators ([`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators)) and resource quotas ([`.spec.quota`](./cr.html#project-v1alpha3-spec-quota)), managed by the [Project](./cr.html#project) resource itself independently of the template.
* **Resources from the template** (optional) — policies and settings described in a [ProjectTemplate](./cr.html#projecttemplate) and created by the controller in the project namespaces, and the objects of its `manifests`, applied with the rights of a project administrator.

## How the module works

This section describes how the module creates projects, adds namespaces to them, grants access and turns existing namespaces into projects.

### Creating a project

To create projects, the following [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) are used:
* [ProjectTemplate](./cr.html#projecttemplate) — a resource that describes the project template: which settings and policies its namespaces get and which parameters can be passed when creating a project;
* [Project](./cr.html#project) — a resource that describes a specific project.

When creating a [Project](./cr.html#project) resource from a specific [ProjectTemplate](./cr.html#projecttemplate), the following happens:
1. The [parameters](./cr.html#project-v1alpha3-spec-parameters) passed are validated against the OpenAPI specification (the [`parametersSchema.openAPIV3Schema`](./cr.html#projecttemplate-v1alpha2-spec-parametersschema-openapiv3schema) field of [ProjectTemplate](./cr.html#projecttemplate));
1. A namespace is created with the name of the [Project](./cr.html#project);
1. The project resources are created from the template:
   * in modern templates (`deckhouse.io/v1alpha2`) the settings, such as the pod security profile, network isolation, log collection and node placement, are described by [structured fields](usage.html#structured-templates). The controller itself creates the corresponding objects in every namespace of the project. Other objects go to [`manifests`](usage.html#describing-other-objects-in-manifests) and are applied with the rights of a project administrator;
   * a template with a Helm `resourcesTemplate` is rendered through Helm. This form is deprecated, and so is the `deckhouse.io/v1alpha1` version of ProjectTemplate, which has no other form ([see details](usage.html#template-checks)). No template can get a new `resourcesTemplate`, a template is created through `deckhouse.io/v1alpha2` only, and the field goes away in a future release, so [move such a template](usage.html#moving-a-template-from-a-helm-text) to structured fields and `manifests`;
1. The standard fields of the project are applied independently of the template: [`.spec.quota`](./cr.html#project-v1alpha3-spec-quota) is reconciled into a `ResourceQuota`, and [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators) into an auto-managed [ProjectRoleBinding](./cr.html#projectrolebinding).

The Project API is served as `deckhouse.io/v1alpha3`. `v1alpha2` manifests keep working through a conversion webhook, and `v1alpha1` is no longer served (see [Project API versions](usage.html#project-api-versions)). The `projectTemplateName` field is optional: when omitted, the `simple` template is used, which only creates the project namespace.

Project names are validated on creation. Names longer than 61 characters, names with the system prefixes `d8-` and `kube-`, names of existing namespaces or of namespaces that a ProjectNamespace has claimed, and names of namespaces whose ServiceAccounts a cluster-wide binding or a binding of another project still names are not allowed. A project `foo-bar` beside a project `foo` is allowed, but it is a separate project and not an additional namespace of `foo`. The webhook warns about it, and an alert lists such pairs ([see details](usage.html#project-naming-rules)).

{% alert level="warning" %}
A change of a project template applies to every project created from it.
{% endalert %}

### Multiple namespaces in a project

A project can span several namespaces. Additional namespaces are created with the [ProjectNamespace](./cr.html#projectnamespace) resource in the main namespace of the project and are named `<project name>-<name>` (for example, `backend-cache` for the `backend` project).

The following automatically applies to the additional namespaces:

* the project template settings — the pod security profile, network policies, log collection, and other policies are created in every namespace of the project;
* the access granted via [ProjectRoleBinding](./cr.html#projectrolebinding) and [ClusterProjectRoleBinding](./cr.html#clusterprojectrolebinding), including the automatic access of the project administrators.

The project quota ([`.spec.quota`](./cr.html#project-v1alpha3-spec-quota)) applies in the main namespace.

Deleting a project deletes all of its namespaces — both the main and the additional ones. See [the usage examples](usage.html#additional-project-namespaces) for details.

### Access to a project

User access to a project is managed at the level of the whole project rather than individual namespaces:

* The subjects listed in [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators) automatically get the project administrator role (`d8:project:admin`).
* The [ProjectRoleBinding](./cr.html#projectrolebinding) resource grants a role in all namespaces of one project.
* A [ClusterProjectRoleBinding](./cr.html#clusterprojectrolebinding) grants a role in all projects of the cluster at once, except the virtual ones.

A ProjectRoleBinding or ClusterProjectRoleBinding can grant only ClusterRoles named `d8:project:*`, `d8:namespace:*`, `d8:project-capability:*`, `d8:namespace-capability:*` or `d8:custom:*`. For details on the roles, see [the user-authz module documentation](/modules/user-authz/). See [the usage examples](usage.html#granting-access-within-a-project) for details on the bindings.

### Automatic project creation for namespaces

Users can still create namespaces directly (`d8 k create namespace`), unless the [`allowNamespacesWithoutProjects`](configuration.html#parameters-allownamespaceswithoutprojects) parameter is `false`: then users create environments as Project objects, and only cluster administrators (`system:masters`, `system:sudouser`), Deckhouse, the module controller and system components create namespaces directly. Every namespace that belongs to no project becomes a project of its own, with the same name and a full-fledged template:

* the template is picked from what the namespace already carries — `secure` if it has vulnerability scanning enabled, `default` if it has a Pod Security Standard or extended monitoring, and `simple` (namespace only) otherwise;
* the project parameters are filled in from the current state of the namespace, so adopting it changes nothing inside;
* from then on the project is the source of truth and is edited like any other. The namespace cannot be deleted directly: to remove the environment, delete the Project, and its namespace is deleted with it.

System namespaces (`d8-*`, `kube-*`, `upmeter-*`, `default`, and anything labeled `heritage: deckhouse` or `heritage: upmeter`) are never adopted: they are listed on the virtual `deckhouse` project (except `default`). There is no label that leaves a user namespace without a project. A namespace whose name is longer than 61 characters is also skipped: that is the Project name limit.

### Isolating a project

A namespace alone does not isolate a project. The project adds the following:

- Access to the whole project through [`.spec.administrators`](./cr.html#project-v1alpha3-spec-administrators), ProjectRoleBinding and ClusterProjectRoleBinding. A plain RoleBinding in one namespace is allowed for delegatable roles.
- A resource limit through [`.spec.quota`](./cr.html#project-v1alpha3-spec-quota), applied as the `d8-project-quota` ResourceQuota in the main namespace.
- Network isolation through the [`networkPolicy.mode: Isolated`](./cr.html#projecttemplate-v1alpha2-spec-networkpolicy-mode) field of the template.
- The pod security profile through the [`podSecurityStandard`](./cr.html#projecttemplate-v1alpha2-spec-podsecuritystandard) field of the template.

## Managing access to cluster-wide resources

Objects in project namespaces can reference cluster-wide resources. For example, a PersistentVolumeClaim can use a StorageClass, a Certificate can use a ClusterIssuer, and a RoleBinding can use a ClusterRole. The module allows cluster administrators to define which cluster-wide resources can be used from project namespaces and which values are used by default.

This mechanism works independently of RBAC. RBAC determines *who can create and modify* objects, while the cluster-wide resource access mechanism determines *which resources* those objects can use.

### How access management works

The following resources are used to manage access to cluster-wide resources:

* [GrantableClusterResourceDefinition](./cr.html#grantableclusterresourcedefinition) registers a type of cluster-wide resource whose access can be managed. These resources are provided by Deckhouse Platform or by module developers.
* [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference) defines where a registered cluster-wide resource is used, for example, which resource field contains a reference to it. These resources are provided by modules.
* [ClusterResourceGrantPolicy](./cr.html#clusterresourcegrantpolicy) defines access rules. Using labels, a cluster administrator selects the projects to which the policy applies and defines the allowed and denied resources, as well as the resource used by default.
* In every namespace of every project, the controller keeps an [AvailableClusterResource](./cr.html#availableclusterresource) for each GrantableClusterResourceDefinition. It lists the cluster-wide resources available to the project under the policies that apply and is read-only.
* When an object is created or modified, webhooks check whether it is allowed to use the specified cluster-wide resource. When an object is created, the resource configured as the project default can also be inserted automatically.

The webhooks reject requests while the module controller is unavailable. Until it is back, objects of the registered kinds (for example, PersistentVolumeClaim, Service or RoleBinding) cannot be created or changed in project namespaces, except by the platform components and members of the `system:masters` group. To keep the controller available, enable [`highAvailability`](configuration.html#parameters-highavailability).

<script src="/assets/js/mermaid.min.js"></script>
<script>mermaid.initialize({ startOnLoad: true });</script>

<pre class="mermaid">
flowchart LR
    A["Module developer or Deckhouse Platform<br/>provides GrantableClusterResourceDefinition<br/>and GrantableClusterResourceReference"] --> C
    B["Cluster administrator<br/>creates<br/>ClusterResourceGrantPolicy"] --> C["Controller"]
    C --> D["Creates AvailableClusterResource<br/>in each project namespace"]
    E["User creates an object<br/>(for example,<br/>PersistentVolumeClaim)"] --> F["Mutating webhook<br/>/defaults"]
    F --> G["Validating webhook<br/>/is-granted"]
    D -. Available names .-> G
    G --> H["Object created<br/>or rejected"]
</pre>

Until the administrator creates a ClusterResourceGrantPolicy, resource availability is determined by their registration: resources are available to all projects if `defaultAvailability: All` (the default value) is set in GrantableClusterResourceDefinition and the resource does not match the `excluded` filters.

Resource quotas are not part of this mechanism. They are managed using the standard Kubernetes ResourceQuota resource.

Access checks are performed only for objects in project namespaces. If an access policy changes, cluster-wide resources already used by existing objects remain available to those objects.

### Access checks and default value assignment

The mechanism determines cluster-wide resource availability and default values based on registered resources and policies that apply to the project.

#### Determining cluster-wide resource availability

If multiple rules apply to a cluster-wide resource, its availability is determined in the following order:

1. The [`excluded`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-excluded) value in GrantableClusterResourceDefinition: the resource is unavailable regardless of policy settings.
1. The [`denied`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-denied) and [`deniedSelector`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-deniedselector) values in the corresponding ClusterResourceGrantPolicy entry.
1. The [`allowed`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowed) and [`allowedSelector`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-allowedselector) values in the corresponding ClusterResourceGrantPolicy entry.
1. The [`availabilityDefault`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-availabilitydefault) value in the ClusterResourceGrantPolicy entries that apply to the project. If an entry sets `allowed` or `allowedSelector` without `availabilityDefault`, every resource not on its lists is unavailable. If one applicable entry sets `All`, it wins over `None` in the others.
1. The [`defaultAvailability`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultavailability) value in GrantableClusterResourceDefinition.

The first matching step decides. Steps 2–4 combine the entries of all ClusterResourceGrantPolicy objects that apply to the project, including the service policies of its template.

#### Assigning default values

The behavior when an object is created depends on the mode configured in [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference-v1alpha1-spec-fieldpaths-defaulting):

* `None`: The value is checked for availability but is not assigned automatically.
* `FillEmpty`: If no value is specified, the project default is assigned.
* `Coerce`: If no value is specified or the specified cluster-wide resource is unavailable to the project, the project default is assigned. Replacing a specified value is reported as an admission warning in the response to the request.

The project default is determined in the following order:

1. The [`default`](./cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-default) value from the corresponding ClusterResourceGrantPolicy entry.
2. The value determined using [`defaultFrom`](./cr.html#grantableclusterresourcedefinition-v1alpha1-spec-defaultfrom) in GrantableClusterResourceDefinition.
3. If no value is found, no default is assigned.

#### Checking existing objects

When an access policy changes, values already used by existing objects are preserved. When an object is updated, only new values are checked. If the value of a field has not changed, it remains valid even after access to the resource has been restricted.

This allows existing objects to continue operating after access to cluster-wide resources is restricted.

#### System requests

Requests from platform components and from members of the `system:masters` group are not checked. The platform components are the API server, the ServiceAccounts of Deckhouse, of this module and of the `user-authz` module, all ServiceAccounts of the `d8-system`, `kube-system` and `d8-user-authz` namespaces, and kubelets. To test a policy, send the request as a project user, for example with `d8 k --as <USER_NAME>`.

### Monitoring access policy violations

If an existing object uses a cluster-wide resource that becomes unavailable to the project after a policy change, the object continues to operate, but the discrepancy is detected by the monitoring system.

When such objects are detected, the [`ClusterResourceGrantPolicyViolation`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation) alert is triggered. Information about violations is available on the Grafana dashboard under "Security" → "Cluster Resource Grant Violations".

The `d8_cluster_objects_grant_violated` metric is used for monitoring. It is exported by the module controller, which recounts the violations of a project namespace on every reconcile and at least every two minutes, using the same availability rules as the admission webhook.

### Resources registered by Deckhouse Platform

Deckhouse Platform registers the following cluster-wide resources:

| Definition name | Cluster-wide resource | Where it is used | Default assignment mode |
| --- | --- | --- | --- |
| `storageclasses` | StorageClass (storage.k8s.io) | PersistentVolumeClaim `.spec.storageClassName` | `Coerce` |
| `loadbalancerclasses` | `loadbalancerclass` value | Service `.spec.loadBalancerClass` (for services of the `LoadBalancer` type) | `FillEmpty` |
| `clusterissuers` | ClusterIssuer (cert-manager.io) | Certificate `.spec.issuerRef.name`; Ingress: `cert-manager.io/cluster-issuer` annotation | `FillEmpty` or `None` |
| `clusterroles` | ClusterRole (rbac.authorization.k8s.io) | RoleBinding `.roleRef.name` | `None` |

The `clusterroles` registration excludes every ClusterRole without the `rbac.deckhouse.io/delegatable: "true"` label. By default, a RoleBinding in a project can reference the `d8:namespace:*` and `d8:project:*` roles, the deprecated `d8:use:role:*` aliases, and the `user-authz:user`, `user-authz:privileged-user`, `user-authz:editor` and `user-authz:admin` roles of the basic model.

The `clusterissuers` definition is registered only when the `cert-manager` module is enabled.

### Access management resources

| Resource | Scope | Created by | Manual creation | Purpose |
| --- | --- | --- | --- | --- |
| [GrantableClusterResourceDefinition](./cr.html#grantableclusterresourcedefinition) | Cluster | Module developer or Deckhouse Platform | Allowed for custom resources | Registers a type of cluster-wide resource whose access can be managed |
| [GrantableClusterResourceReference](./cr.html#grantableclusterresourcereference) | Cluster | Module developer | Allowed for fields of custom resources | Defines where a registered cluster-wide resource is used |
| [ClusterResourceGrantPolicy](./cr.html#clusterresourcegrantpolicy) | Cluster | Cluster administrator | Required | Defines allowed and denied resources, as well as the resource used by the project by default |
| [AvailableClusterResource](./cr.html#availableclusterresource) | Namespace | Controller (automatically) | Prohibited (protected by a webhook) | Read-only catalog of resources available to the project |

For detailed access configuration scenarios, viewing resources available to projects, access checks and default assignment rules, and recommendations for module developers, refer to ["Usage examples"](usage.html#managing-access-to-cluster-wide-resources).
