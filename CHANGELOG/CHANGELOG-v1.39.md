# Changelog v1.39

## Know before update


 - Cillium agent Pods will be restarted.
 - Istio version `1.10` has been removed. Use any other supported version (recommended version is `1.13`).
 - The `openvpn` module will be restarted.

## Features


 - **[cni-cilium]** Fix validation for InitContainers. [#2512](https://github.com/deckhouse/deckhouse/pull/2512)
    The `cni-cilium` module will be restarted.
 - **[control-plane-manager]** Save the latest 5 backups of control-plane configs. [#2783](https://github.com/deckhouse/deckhouse/pull/2783)
 - **[istio]** Got rid of 1.10.1 version. Metric `d8_istio_pod_revision` renamed to `d8_istio_dataplane_metadata` and enriched with `version`, `full_version`, `desired_version` and `desired_full_version` labels. Alerts were refactored using the new metric. Also, a great refactoring for using term `version` instead of `revision`. [#2707](https://github.com/deckhouse/deckhouse/pull/2707)
    Istio version `1.10` has been removed. Use any other supported version (recommended version is `1.13`).
 - **[kube-proxy]** Fix validation for InitContainers. [#2512](https://github.com/deckhouse/deckhouse/pull/2512)
    The `kube-proxy` module will be restarted.
 - **[linstor]** Fix validation for InitContainers. [#2512](https://github.com/deckhouse/deckhouse/pull/2512)
    The `linstor` module will be restarted.
 - **[log-shipper]** Add Kafka destination. [#2871](https://github.com/deckhouse/deckhouse/pull/2871)
 - **[node-manager]** Cluster-autoscaler: smart balancing between zones in a NodeGroup.
    CA will try to align the number of nodes between zones during scaling.
    New priority field in the NodeGroup OpenAPI spec, which can set the order of scaling up nodes between different NodeGroups. [#2735](https://github.com/deckhouse/deckhouse/pull/2735)
 - **[openvpn]** Fix validation for InitContainers. [#2512](https://github.com/deckhouse/deckhouse/pull/2512)
    The `openvpn` module will be restarted.
 - **[upmeter]** Fix validation for InitContainers. [#2512](https://github.com/deckhouse/deckhouse/pull/2512)
    The `upmeter` module will be restarted.

## Fixes


 - **[cni-cilium]** Fix panic in the cilium-agent code. [#2781](https://github.com/deckhouse/deckhouse/pull/2781)
    Cillium agent Pods will be restarted.
 - **[deckhouse-controller]** Update shell-operator and addon-operator dependencies to reduce memory usage. [#2864](https://github.com/deckhouse/deckhouse/pull/2864)
 - **[dhctl]** Wait for the control plane manager Pod readiness while creating a new control-plane node. Fix no control new nodes in the internal state. [#2764](https://github.com/deckhouse/deckhouse/pull/2764)
 - **[kube-proxy]** Fix insufficient privileges for the init container. [#2923](https://github.com/deckhouse/deckhouse/pull/2923)
    The `kube-proxy` DaemonSet will be restarted.
 - **[log-shipper]** Fix Elasticsearch 8.X and Opensearch. [#2798](https://github.com/deckhouse/deckhouse/pull/2798)
 - **[log-shipper]** Expire metrics more frequently. [#2795](https://github.com/deckhouse/deckhouse/pull/2795)
 - **[log-shipper]** Add `FlowSchema` and `PriorityLevelConfiguration` to limit concurrent requests to Kubernetes API for the log-shipper ServiceAccount. [#2794](https://github.com/deckhouse/deckhouse/pull/2794)
 - **[log-shipper]** Check namespace before creating config. [#2793](https://github.com/deckhouse/deckhouse/pull/2793)
 - **[monitoring-kubernetes]** Fix render for the `K8SKubeletTooManyPods` alert. [#2843](https://github.com/deckhouse/deckhouse/pull/2843)
 - **[prometheus]** Fixed calculation of PVC size and retention size. [#2934](https://github.com/deckhouse/deckhouse/pull/2934)
 - **[prometheus]** Improve Prometheus Retain alerts. [#2841](https://github.com/deckhouse/deckhouse/pull/2841)
 - **[upmeter]** Get rid of shell-operator dependency. [#2736](https://github.com/deckhouse/deckhouse/pull/2736)

## Chore


 - **[linstor]** Fix the description for the `D8LinstorNodeIsNotOnline` alert. [#2734](https://github.com/deckhouse/deckhouse/pull/2734)
 - **[metallb]** Bump metallb version to `0.13.7` and migrate configuration to metallb CRDs.
    Add alerts and update documentation. [#2595](https://github.com/deckhouse/deckhouse/pull/2595)
    metallb Pods will be restarted.
 - **[node-local-dns]** Remove Prometheus rule `D8NodeLocalDnsNotScheduledOnNode`. [#2778](https://github.com/deckhouse/deckhouse/pull/2778)
 - **[node-manager]** Clarify further actions for the `EarlyOOMPodIsNotReady` alert. [#2761](https://github.com/deckhouse/deckhouse/pull/2761)

