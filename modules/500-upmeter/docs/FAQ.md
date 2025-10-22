---
title: "Cluster SLA Monitoring: FAQ"
type:
  - instruction
---

## Why can't `upmeter-probe-*` pods be placed in the cluster, why are some pods constantly deleted?

The module implements tests for the availability of the functionality of some Kubernetes controllers.

The tests are performed by creating and deleting temporary pods.

`upmeter-probe-scheduler` objects implement a test for the functionality of placing pods on nodes.
As part of the test, a Pod is created and placed on a node. Then this pod is deleted.

`upmeter-probe-controller-manager` objects implement a test for the health of `kube-controller-manager`.

`StatefulSet` is created for the test, and it is checked that this object has spawned a Pod (since the actual placement of the pod is not required and is checked as part of another test, a Pod is created that is guaranteed not to be placed, i.e. is in the `Pending` state). Deletes the StatefulSet, checks that the Pod it spawned has been deleted.

`smoke-mini` objects implement network connectivity testing between nodes.
Five `StatefulSet` with one replica are deployed for testing. The test checks connectivity between `smoke-mini` Pods as well as network connectivity with `upmeter-agent` Pods running on master nodes.  
Once per minute, one of the `smoke-mini` Pods is migrated to another node.
