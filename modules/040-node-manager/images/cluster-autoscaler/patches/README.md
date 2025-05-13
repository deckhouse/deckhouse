# Patches

## Scale from zero

We want to scale a node group from zero but our MCM revision does not support generic MachineClass CRs. 
With this patch we adds an ability to calculate node-group capacity from MachineDeployment annotations.
It makes sense only for calculation node-group capacity from zero, when we have no nodes presented.

## Kruise advanced daemonsets

Cluster autoscaler can't tell the difference between pods created by apps/v1 and apps.kruise.io/v1alpha1 
daemonsets when simulating if a node can be terminated. This patch makes cluster autoscaler check PDB 
instead of checking if an apps/v1 daemonset exists, when it bumps into a pod created by an advanced daemonset.

# Remove additional cordon by mcm cloud provider
Gardner cluster autoscaler cordon node of main flow of autoscaler.
It can be keep nodes in cordon status without deleting them.
