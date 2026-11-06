---
title: Package access
permalink: en/admin/configuration/marketplace/package-access.html
description: "Choose the application packages that project users can install from Deckhouse Platform Marketplace, using ClusterResourceGrantPolicy."
---

By default, a user who may create Application objects in a namespace can install an application from any package. If the [`multitenancy-manager`](/modules/multitenancy-manager/) module is enabled, the cluster administrator can choose the packages available in each [project](../../../glossary.html#project) and in each namespace of a project. An Application of a package that is not available is rejected.

Deckhouse Platform registers ApplicationPackage as a cluster-wide resource of the module named `applicationpackages`, and the module checks the `spec.packageName` field of Application objects in project namespaces. To choose the available packages, create a [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) for the `applicationpackages` resource. A policy does not grant RBAC permissions: to install an application, a user still needs a role that allows creating Application objects in the namespace. For how the mechanism works, refer to the [`multitenancy-manager` module documentation](/modules/multitenancy-manager/#managing-access-to-cluster-wide-resources).

## Allowing packages in a project

To allow only the `postgres` and `redis` packages in the `my-project` project, create the following ClusterResourceGrantPolicy:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: my-project-packages
spec:
  projectSelector:
    matchLabels:
      projects.deckhouse.io/project: my-project
  resources:
    - resourceName: applicationpackages
      allowed:
        - postgres
        - redis
```

The names in `allowed` are the names of ApplicationPackage objects, the same as in the `spec.packageName` field of an Application. To list the packages, run `d8 k get ap`.

The module puts the `projects.deckhouse.io/project: <PROJECT_NAME>` label on every Project, so the policy applies to every namespace of `my-project`, including the additional namespaces created later. To select several projects, use the `projects.deckhouse.io/project-template: <PROJECT_TEMPLATE_NAME>` label, which selects the projects of a template, or your own label on the Project objects, for example `team: payments`.

Once a policy with `allowed` applies to a project, the packages that are not on the list are unavailable in it. If several policies apply to a project, their `allowed` lists add up.

## Allowing packages in one namespace of a project

A policy with [`namespaceSelector`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-namespaceselector) applies only to the namespaces whose labels match the selector. It does not restrict the other namespaces of the project: if no other policy restricts them, every package is available there.

To allow the `postgres` package only in the `my-project-db` namespace of the `my-project` project, and no packages in its other namespaces, create two policies:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: my-project-no-packages
spec:
  projectSelector:
    matchLabels:
      projects.deckhouse.io/project: my-project
  resources:
    - resourceName: applicationpackages
      availabilityDefault: None
---
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: my-project-db-packages
spec:
  projectSelector:
    matchLabels:
      projects.deckhouse.io/project: my-project
  namespaceSelector:
    matchLabels:
      kubernetes.io/metadata.name: my-project-db
  resources:
    - resourceName: applicationpackages
      allowed:
        - postgres
```

The first policy makes every package unavailable in all namespaces of the project. The second one allows `postgres` in `my-project-db`: a package that one policy allows is available even if another policy sets `availabilityDefault: None`.

The `NewNamespacesCovered` condition in the status of the second policy is `False`, because a new additional namespace of the project does not match its `namespaceSelector`. The first policy covers such namespaces, so no packages are available in them.

## Denying packages

To make some packages unavailable and keep the others, list them in [`denied`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-denied). The following policy makes the `experimental-db` package unavailable in every project:

```yaml
apiVersion: multitenancy.deckhouse.io/v1alpha2
kind: ClusterResourceGrantPolicy
metadata:
  name: deny-experimental-db
spec:
  projectSelector: {}
  resources:
    - resourceName: applicationpackages
      denied:
        - experimental-db
```

The empty `projectSelector` selects every project. A denied package is unavailable even if another policy allows it.

## Packages in a project template or in a project

The same rules can be set in `spec.resources` of a project template, so that they apply to every project created from the template, or in [`spec.grants.resources`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-grants) of a single Project. For example, to allow only `postgres` and `redis` in the projects of a template, add the following entry to the template:

```yaml
spec:
  resources:
    - resourceName: applicationpackages
      allowed:
        - postgres
        - redis
```

For details, refer to ["Granting cluster-wide resources through a project template"](/modules/multitenancy-manager/usage.html#granting-cluster-wide-resources-through-a-project-template) and ["Granting cluster-wide resources to one project"](/modules/multitenancy-manager/usage.html#granting-cluster-wide-resources-to-one-project).

## Checking the available packages

The packages available in a namespace are listed in the `applicationpackages` [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) of that namespace, the same list that project users see:

```shell
d8 k get available applicationpackages -n <NAMESPACE> -o yaml
```

Requests of the platform components and of members of the `system:masters` group are not checked. To check a policy, send a request on behalf of a project user, for example a server-side dry run, which creates nothing:

```shell
d8 k apply -f application.yaml --dry-run=server --as <USER_NAME>
```

## Applications installed before a restriction

A policy does not remove installed applications. When an Application is changed, its package is checked only if the package changes, so an application installed before its package became unavailable to the project keeps working.

Such applications are counted by the `d8_cluster_objects_grant_violated` metric, and the [ClusterResourceGrantPolicyViolation](../../../reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation) alert fires. The alert stops when the package becomes available to the project again or the Application is deleted.

## Limitations

- Policies apply only in project namespaces. They do not apply to the namespaces of the virtual projects `default` and `deckhouse`, such as the `default` namespace, so any package can be installed there.
- A policy selects packages by name. It does not restrict the repository an available package is installed from (`spec.packageRepositoryName`) or its version.
- A policy does not hide packages: any authenticated user can read the ApplicationPackageVersion objects of all packages, including the ones unavailable to the project. The `status.usedBy` field of a version names the applications installed from it and their namespaces in all projects.
- While the controller of the `multitenancy-manager` module is unavailable, Application objects cannot be created or changed in project namespaces. To keep the controller available, enable [`highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability).
