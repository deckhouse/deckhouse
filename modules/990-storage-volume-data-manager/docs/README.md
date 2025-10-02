---
title: "Module storage-volume-data-manager"
description: "Module storage-volume-data-manager: general concepts and provisions."
moduleStatus: preview
---

The storage-volume-data-manager module provides a mechanism for exporting the contents of a user volume via the HTTP protocol. It creates a namespaced resource "DataExport" in the namespace where data export needs to be created. This resource specifies the targetRef - a reference to the resource that needs to be exported. Only PersistentVolumeClaim and VolumeSnapshot are supported. The standard Go file server is used as a base. Export of volumes is supported both in file system mode and block mode.
User authorization is provided by k8s tools, with a mechanism for downloading files/blocks in byte ranges (supports 'Range' headers).

## Key Parameters

- ttl - this is the time after the last server access: downloading a file or listing a directory. After the ttl expires, the exporter pod is deleted, and the user PVC is returned to the user PV.
 In the DataExport resource, the Condition Ready is set to false with the Reason as Expired.
- publish - a value of true in publish means that access to the exporter pod will be opened from outside the cluster. In this case, a public access string will appear in the PublicURL field: publicURL: `https://data-exporter.<public-domain>/<namespace>/<user-pvc-name>/`

## Quick Start

Enabling the module:

```bash
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: storage-volume-data-manager
spec:
  enabled: true
  version: 1
EOF
```

To create and manage DataExport resources, the d8 command is used, and the structure looks as follows:

```bash
d8 data -n <namespace> create <DataExport resource name> <resource type for export>/<resource name for export> --ttl 5m
```

Important!
Working with PVC resources is possible if the PVC is not currently in use by pods.

For example, creating a DataExport resource for a PVC named "data" in the namespace "project" with a ttl of 5m:

```bash
d8 data -n project create my-export pvc/data --ttl 5m
```

Information about the created resource can be obtained with the command:

```bash
d8 k -n project get de my-export
```

Data downloading is performed with the following command:

```bash
d8 data -n <namespace> download <resource type (pvc/vs/dataexport)>/<resource name>/<file path> -o <file name> --publish
```

For example:

```bash
d8 data -n project download dataexport/my-export -o test_file.txt --publish
```
