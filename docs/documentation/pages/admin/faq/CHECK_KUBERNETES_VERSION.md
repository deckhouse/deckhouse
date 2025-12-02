---
title: How to check the Kubernetes version in use?
permalink: en/faq-common/check-kubernetes-version.html
---

To check the version of Kubernetes version in use, run the command:

```shell
d8 k get nodes
```

Output example:

```console
NAME                   STATUS   ROLES                  AGE    VERSION
frontend-0             Ready    frontend               118d   v1.31.9
master-0               Ready    control-plane,master   118d   v1.31.9
master-1               Ready    control-plane,master   118d   v1.31.9
master-2               Ready    control-plane,master   118d   v1.31.9
system-0               Ready    system                 118d   v1.31.9
system-1               Ready    system                 118d   v1.31.9
worker-0               Ready    worker                 37d    v1.31.9
worker-1               Ready    worker                 19d    v1.31.9
```
