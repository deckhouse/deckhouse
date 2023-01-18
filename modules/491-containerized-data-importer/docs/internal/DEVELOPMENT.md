---
title: Maintenance of the containerized-data-importer module 
searchable: false
---

How to update CDI
-----------------

Most manifests, such as crd and rbac, are generated automatically from the original ones.  
To update the version of CDI run:

```bash
hack/update.sh v1.55.1
```

Also don't forget to bump the version in `images/artifact/werf.inc.yaml`:

```yaml
{{- $version := "1.55.1" }}
```
