# Patches

## Scale from zero

We want to scale a node group from zero but our MCM revision does not support generic MachineClass CRs. 
With this patch we adds an ability to calculate node-group capacity from MachineDeployment annotations.
It makes sense only for calculation node-group capacity from zero, when we have no nodes presented.

## Kruise advanced daemonsets

Cluster autoscaler can't tell the difference between pods created by apps/v1 and apps.kruise.io/v1alpha1 
daemonsets when simulating if a node can be terminated. This patch makes cluster autoscaler check PDB 
instead of checking if an apps/v1 daemonset exists, when it bumps into a pod created by an advanced daemonset.

# Set priorities for to de deleted machines and clean annotation node.machine.sapcloud.io/trigger-deletion-by-mcm
Remove additional cordoning nodes from mcm cloud provider.

New autoscaler works with new version MCM witch select nodes for deleting from annotation `node.machine.sapcloud.io/trigger-deletion-by-mcm`
This annotation does not support by our MCM, and we should set deleting priority with annotation `machinepriority.machine.sapcloud.io`.
We set priority for machines and keep `node.machine.sapcloud.io/trigger-deletion-by-mcm` annotation for calculation replicas,
but we need to clean deleted machines from annotation in refresh function for keeping up to date annotation value to avoid
drizzling replicas count in machine deployment.
