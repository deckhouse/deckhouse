---
title: Maintenance of the virtualization module 
searchable: false
---

How to update KubeVirt
----------------------

Most manifests, such as crd and rbac, are generated automatically from the original ones.  
To update the version of KubeVirt run:

```bash
hack/update.sh v1.58.0
```

Also don't forget to bump the version in `images/artifact/werf.inc.yaml`:

```yaml
{{- $version := "0.58.0" }}
```

How to update Custom Resources
------------------------------

Custom resource definitions are generated out of API types.  
To add new fields edit specific object type in `./hooks/internal/...` and run:

```bash
make generate
make crds
```

to generate dependent resources.

> **Note**: you can edit `doc-ru-*` files manually. After editing run `make crds` again to format the changes.
