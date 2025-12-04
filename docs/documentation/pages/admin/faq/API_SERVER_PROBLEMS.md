---
title: What to do when the API server is overloaded?
permalink: en/faq-common/api-server-overloaded.html
---

The following signs may indicate problems with API server load and memory consumption:

- `kubectl (d8)` responds slowly or does not respond at all (commands are executed slowly or not at all).
- Pods are recreated in the cluster for no apparent reason.

If these signs are present, perform the following actions:

1. Check the resource consumption of API server pods. To do this, use the command:

   ```shell
   d8 k -n kube-system top po -l component=kube-apiserver
   ```

   Pay attention to `MEMORY` consumption and `CPU`.

   Example output:

   ```console
   NAME                               CPU(cores)   MEMORY(bytes)
   kube-apiserver-sandbox1-master-0   251m         1476Mi
   ```

1. Check the metrics in Grafana.

   To view the metrics, open the dashboard "Home" → "Dashboards" → "Kubernetes Cluster" → "Control Plane Status". Review the graphs related to the API server ("Kube-apiserver CPU Usage", "Kube-apiserver Memory Usage", "Kube-apiserver latency", etc.).

1. Review the API server [audit logs](/modules/control-plane-manager/#auditing) to identify the source of high memory consumption. One common cause of high memory consumption is a large number of requests.
