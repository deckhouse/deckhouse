---
title: Поддержка модуля containerized-data-importer
searchable: false
---

Как обновлять CDI
-----------------

Большинство манифестов, таких как crd и rbac, генерируется автоматически из оригинальных.  
Для того чтобы обновить версию CDI выполните:

```bash
hack/update.sh v1.55.1
```

А так же не забудьте бампнуть версию в `images/artifact/werf.inc.yaml`:

```yaml
{{- $version := "1.55.1" }}
```
