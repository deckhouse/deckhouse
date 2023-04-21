### pdb-daemonset.patch

Supports DaemonSets in disruption controller by adding /scale subresource to daemonsets API. It allows to control the eviction rate of DaemonSet pods.

> Upstream PR https://github.com/kubernetes/kubernetes/pull/98307.
