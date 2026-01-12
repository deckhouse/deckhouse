---
title: "Role model"
permalink: en/stronghold/documentation/admin/platform-management/access-control/role-model.html
---

## Description

DVP provides a standard set of roles for managing access to project and cluster resources.
There are two types of roles:

- [Use roles](#use-roles): Assigned to project users, allowing them to manage resources **within a specified project**.
- [Manage roles](#manage-roles): Intended for DVP administrators, granting them permissions to manage resources at the platform-wide level.

Platform permissions are configured using the standard Kubernetes RBAC approach.
This involves creating `RoleBinding` or `ClusterRoleBinding` resources to assign the appropriate role.

### Use roles

Use roles grant permissions to a user **within a specified project** and define access to project resources.
These roles can only be used with a `RoleBinding` resource.

DVP provides the following use roles:

- `d8:use:role:viewer`: Allows viewing project resources and authenticate to the cluster.
- `d8:use:role:user`: Includes all permissions from the `d8:use:role:viewer` role
    and also allows viewing RBAC secrets and resources, connect to virtual machines, and run the `d8 k proxy` command.
- `d8:use:role:manager`: Includes all permissions from the `d8:use:role:user` role
    and also allows managing project resources.
- `d8:use:role:admin`: Includes all permissions from the `d8:use:role:manager` role
    and also allows managing the following resources:
    `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy`, and `VirtualImage`.

Example of administrator permissions granted to the `joe` user in the `vms` project:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: project-admin-joe
  namespace: vms
subjects:
- kind: User
  name: joe@example.com # For users.deckhouse.io, the parameter is .spec.email
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
```

Example of administrator permissions granted to the `vms-admins` user group in the `vms` project:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: project-admin-joe
  namespace: vms
subjects:
- kind: Group
  name: vms-admins # For groups.deckhouse.io, the parameter is .spec.name
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:use:role:admin
  apiGroup: rbac.authorization.k8s.io
```

### Manage roles

Manage roles grant permissions to manage the following:

- DVP cluster resources
- DVP module settings
- Module components in projects with the `d8-*` and `kube-*` prefixes

DVP provides the following manage roles, allowing to manage all subsystems of the `all` cluster:

- `d8:manage:all:viewer`: Grants permissions to view module configurations (`moduleConfig` resources)
    and access cluster-wide resources of these modules.
- `d8:manage:all:manager`: Includes all permissions from the `viewer` role
    and also allows managing module configurations and cluster-wide resources of these modules.

Example of cluster administrator permissions granted to the `jane` user:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com # For users.deckhouse.io, the parameter is .spec.email
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:all:admin # Manage role name
  apiGroup: rbac.authorization.k8s.io
```

DVP can grant restricted permissions to administrators for managing resources and modules associated with specific subsystems.

To assign network subsystem administrator permissions to the `jane` user, use the following configuration:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: network-admin-jane
subjects:
- kind: User
  name: jane.doe@example.com # For users.deckhouse.io, the parameter is .spec.email
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: d8:manage:networking:admin # Manage role name
  apiGroup: rbac.authorization.k8s.io
```

Subsystem management role names follow the `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>` format, where:

- `<SUBSYSTEM>` indicates the subsystem name.
- `<ACCESS_LEVEL>` indicates the access level, similar to the roles for the `all` subsystem.

The subsystems available for manage roles are listed in the following table:

{% include rbac/rbac-subsystems-list.liquid %}
