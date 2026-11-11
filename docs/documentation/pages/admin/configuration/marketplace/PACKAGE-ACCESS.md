---
title: Configuring package access
permalink: en/admin/configuration/marketplace/package-access.html
description: "Choose the application packages that project users can install from Deckhouse Platform Marketplace, using ClusterResourceGrantPolicy."
---

By default, a user who may create Application objects in a namespace can install applications from any package. If the [`multitenancy-manager`](/modules/multitenancy-manager/) module is enabled, the cluster administrator can restrict the packages available for installation in each [project](../../../glossary.html#project) and each namespace of a project. A request to create an Application with unavailable package will be rejected.

To configure access to packages, Deckhouse Platform registers an ApplicationPackage as a cluster-wide resource named `applicationpackages`. The `multitenancy-manager` module checks the [`spec.packageName`](../../../reference/api/cr.html#application-v1alpha1-spec-packagename) field value of Application objects in project namespaces. Package access in configured using the [ClusterResourceGrantPolicy](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy) resource. A policy does not grant RBAC permissions: to install an application, a user still needs a role that allows creating Application objects in the corresponding namespace.

For details on the access management mechanism, refer to the [`multitenancy-manager` module documentation](/modules/multitenancy-manager/#managing-access-to-cluster-wide-resources).

## Allowing packages in a project

To allow installing only the `postgres` and `redis` packages in the `my-project` project, create the following ClusterResourceGrantPolicy:

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

The names in `allowed` field are the names of ApplicationPackage objects corresponding to the respective [`spec.packageName`](../../../reference/api/cr.html#application-v1alpha1-spec-packagename) field values of an Application. To list the available packages, run `d8 k get ap`.

The `multitenancy-manager` module automatically assigns the `projects.deckhouse.io/project: <PROJECT_NAME>` label to every Project, which makes the policy applied to every namespace in the respective namespace, including the additional namespaces created after the policy is created.

To apply a policy to several projects, use one of the labels:

- `projects.deckhouse.io/project-template: <PROJECT_TEMPLATE_NAME>` label, which selects all projects based on the template
- your own label on the Project objects, for example `team: payments`, which selects a custom group of projects.

Once a policy with `allowed` field applies to a project, the packages that are not listed in it become unavailable for installation. If several policies apply to a project, their `allowed` lists are combined.

## Allowing packages in one namespace of a project

Using the [`namespaceSelector`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-namespaceselector) parameter, you can restrict a policy to selected namespaces of a project. The policy is applied only to namespaces whose labels match the selector. It does not restrict the other namespaces of the project. If no other policy restricts them, every package is available there.

For example, to allow installation of the `postgres` package only in the `my-project-db` namespace of the `my-project` project and deny installation of any packages in other namespaces of the project, create two policies:

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

The first policy (`my-project-no-packages`) makes every package unavailable for installation in all namespaces of the project. The second one (`my-project-db-packages`) allows the `postgres` packages in the `my-project-db` namespace. If a package is allowed at least by a single policy, it's available for installation, even if another policy sets [`availabilityDefault: None`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-availabilitydefault).

The `NewNamespacesCovered` condition in the status of the second policy is `False`, because a new additional namespace of the project does not match its `namespaceSelector`. However the first policy covers such namespaces, so no packages are available for installation in them.

## Denying packages

To deny installation of specific packages while keeping the others available, list them in [`denied`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-resources-denied).

The following is an example policy that makes the `experimental-db` package unavailable in every project:

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

The empty [`projectSelector`](/modules/multitenancy-manager/cr.html#clusterresourcegrantpolicy-v1alpha2-spec-projectselector) parameter selects every project. A package listed under `denied` is unavailable for installation even if another policy allows it.

## Packages in a project template or in a project

Package access rules can be set in [`spec.resources`](/modules/multitenancy-manager/cr.html#projecttemplate-v1alpha2-spec-resources) of a project template, so that they apply to every project created from the template, or in [`spec.grants.resources`](/modules/multitenancy-manager/cr.html#project-v1alpha3-spec-grants) of a single Project.

For example, to allow installation of only `postgres` and `redis` packages in the projects of a template, add the following entry to the template:

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

The packages available for installation in a namespace are listed in the `applicationpackages` [AvailableClusterResource](/modules/multitenancy-manager/cr.html#availableclusterresource) of the corresponding namespace. That same list can be read by the project users.

To view the list, run the following command:

```shell
d8 k get available applicationpackages -n <NAMESPACE> -o yaml
```

Requests of the platform components and of members of the `system:masters` group are not checked against policies. To check on restrictions, use a project user account.

For example, to perform a server-side dry run of an Application manifest without creating the object, run the following:

```shell
d8 k apply -f application.yaml --dry-run=server --as <USER_NAME>
```

## Applications installed before a restriction

Introducing policy restrictions does not remove installed applications. When an existing Application object is changed, the availability of its package is checked only if the package is modified. Therefore an application installed before its package became unavailable to the project keeps working.

Such applications are counted by the `d8_cluster_objects_grant_violated` metric, and the [`ClusterResourceGrantPolicyViolation`](../../../reference/alerts.html#multitenancy-manager-clusterresourcegrantpolicyviolation) alert fires. The alert stops when the package becomes available to the project again or the corresponding Application object is deleted.

## Limitations

When configuring access to packages, consider the following limitations:

- Policies apply only in project namespaces. They do not apply to the namespaces of the virtual projects `default` and `deckhouse`, so any package can be installed there.
- A policy selects available packages by name. It does not restrict the repository an available package is installed from (`spec.packageRepositoryName`) or its version.
- A policy does not hide unavailable packages. Any authenticated user can read the ApplicationPackageVersion objects of all packages, including the ones unavailable to the project. The `status.usedBy` field of a version lists the applications installed from it and their namespaces in all projects.
- While the controller of the `multitenancy-manager` module is unavailable, Application objects cannot be created or changed in project namespaces. To keep the controller available, enable [`highAvailability`](/modules/multitenancy-manager/configuration.html#parameters-highavailability) in the `multitenancy-manager` settings.
