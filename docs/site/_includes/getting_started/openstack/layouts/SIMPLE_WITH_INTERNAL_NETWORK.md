![resources](https://docs.google.com/drawings/d/e/2PACX-1vQOcYZPtHBqMtlNx9PDcMrqI0WEwRssL-oXONnrOoKNaIx1fcEODo9dK2zOoF1wbKeKJlhphFTuefB-/pub?w=960&h=720)
<!--- Исходник: https://docs.google.com/drawings/d/1H9HGOn4abpmZwIhpwwdZSSO9izvyOZakG8HpmmzZZEo/edit --->

The master node and cluster nodes are connected to the existing network. This placement strategy might come in handy if you need to merge a Kubernetes cluster with existing VMs.

**Caution!**

This placement strategy does not involve the management of `SecurityGroups` (it is assumed they were created beforehand).
To configure security policies, you must explicitly specify both `additionalSecurityGroups` in the OpenStackClusterConfiguration
for the masterNodeGroup and other nodeGroups, and `additionalSecurityGroups` when creating `OpenStackInstanceClass` in the cluster.
