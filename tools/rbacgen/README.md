## RBAC v2 Generator

The RBACv2 Generator is a tool designed to create RBACv2 cluster roles (manage/use) for modules based on their CRDs.

This tool reads and analyzes CRDs, and if they pass filters, it generates cluster roles in the ```templates/rbacv2``` directory. 
The type of generated role (use or manage) is determined by the resource’s scope:

• *Namespaced resources*: Use roles are generated.

• *Cluster-scoped resources*: Manage roles are generated.

### Adding a Module

To add a module, create a file named rbac.yaml in the module’s directory.

This file contains the information required by the generator to create the roles.

### Examples

Below is an example for the ```deckhouse``` module. 
It includes the module name, module namespace, scopes, and the path to the CRDs:
```yaml
module: deckhouse
namespace: d8-system
scopes:
  - deckhouse
crds:
  - deckhouse-controller/crds/*.yaml
```

Some modules do not have a namespace. For these modules, you need to explicitly set the namespace field to ```none```:
```yaml
module: priority-class
namespace: none
scopes:
  - kubernetes
```

Even though this module does not have CRDs, manage roles will still be generated, 
as these roles are responsible for managing the module’s configuration.

In many cases, specifying the namespace is not necessary. If the namespace field is omitted, 
the tool assumes the namespace is ```d8-MODULE_NAME```:

```yaml
module: ceph-csi
scopes:
  - storage
  - infrastructure
crds:
  - modules/031-ceph-csi/crds/*.yaml
```

By default, the tool generates roles only for resources in the ```deckhouse.io``` group. 
However, if a module provides additional resources in other groups, 
you can include them by specifying them in the configuration:
```yaml
module: operator-trivy
scopes:
  - security
crds:
  - ee/modules/500-operator-trivy/crds/native/*.yaml
allowedResources:
  - group: aquasecurity.github.io
    resources:
      - all
```
