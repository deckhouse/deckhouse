## Patches

### kubelet-graceful-shutdown-wait-for-external-inhibitors

This patch supports postponing all pods termination until Node status contains 
a condition with type "GracefulShutdownPostpone" and status "True".

This condition may be set by an external component to prevent Node shutdown
for some scenarios. For example, d8-shutdown-inhibitor.service monitors
if there are Pods with the label "pod.deckhouse.io/inhibit-node-shutdown"
on the Node and prevent shutdown until user migrates these Pods from the Node.

### kubelet-graceful-shutdown-cleanup-memory-manager-state

This patch ensures that the Memory Manager state file is removed during a graceful node shutdown.

The Memory Manager stores the node memory state in a file. After a reboot, the amount of used memory may slightly differ from the previous state, which can make the stored state invalid and prevent the kubelet from starting. Removing the state file before shutdown ensures that the Memory Manager starts with a clean state after the reboot.
See issue: https://github.com/kubernetes/kubernetes/issues/131253
