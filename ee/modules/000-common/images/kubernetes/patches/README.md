## Patches

### kubelet-graceful-shutdown-wait-for-external-inhibitors

The patch adds a delay before all pods are terminated. This could be a file that kubelet is waiting for to be deleted.
