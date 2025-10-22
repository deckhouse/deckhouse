## Patches

### kubelet-graceful-shutdown-wait-for-external-inhibitors

This patch supports postponing all pods termination until Node status contains 
a condition with type "GracefulShutdownPostpone" and status "True".

This condition may be set by an external component to prevent Node shutdown
for some scenarios. For example, d8-shutdown-inhibitor.service monitors
if there are Pods with the label "pod.deckhouse.io/inhibit-node-shutdown"
on the Node and prevent shutdown until user migrates these Pods from the Node.
