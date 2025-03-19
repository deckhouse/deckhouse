---
title: "The operator-trivy module"
description: operator-trivy is a Deckhouse module for periodic scanning for vulnerabilities in a Kubernetes cluster.
---

The module allows you to run a regular vulnerability scans of user images in runtime on known CVEs. The module uses the [Trivy](https://github.com/aquasecurity/trivy) project. [Public databases](https://github.com/aquasecurity/travy-db/tree/main/pkg/vulnsrc) are used for scanning vulnerabilities.

Scanning is performed every 24 hours in namespaces that contain the label `security-scanning.deckhouse.io/enabled=""`.
If there are no namespaces with this label in the cluster, the `default` namespace is scanned.

Once a namespace with the label `security-scanning.deckhouse.io/enabled=""` is detected in the cluster, scanning of the `default` namespace stops.
To re-enable scanning for the `default` namespace, use the following command to set the label to the namespace:

```shell
kubectl label namespace default security-scanning.deckhouse.io/enabled=""
```
