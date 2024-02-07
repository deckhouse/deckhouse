# Patches

## Daemonset eviction

Disable eviction for daemonset pods in d8-* namespaces. If you need to change this behavior add
`"cluster-autoscaler.kubernetes.io/enable-ds-eviction": "true"` annotation for daemonset pod (not DaemonSet object!)


## Scale from zero

We want to scale a node group from zero but our MCM revision does not support generic MachineClass CRs. 
With this patch we adds an ability to calculate node-group capacity from MachineDeployment annotations.
It makes sense only for calculation node-group capacity from zero, when we have no nodes presented.
