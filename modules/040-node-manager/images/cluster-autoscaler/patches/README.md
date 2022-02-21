# Patches

## Min size

Forbid the cluster autoscaler to delete nodes if the minimal size of a NodeGroup is reached.


## Daemonset eviction

Disable eviction for daemonset pods in d8-* namespaces. If you need to change this behavior add
`"cluster-autoscaler.kubernetes.io/enable-ds-eviction": "true"` annotation for daemonset pod (not DaemonSet object!)
