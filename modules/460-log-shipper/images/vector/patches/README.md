## Patches

### Kubernetes Owner Reference

Expand the owner reference if the pod controller is a ReplicaSets or Job, and it also has the owner reference.

ReplicaSets is an internal controller which should not be used directly, so it is not that informative.
Way better is to know which Deployment is the owner of the pod.

Now we are waiting vector to migrate to the [kube-rs](https://github.com/kube-rs/kube-rs) client to adopt our patch and open a PR.

https://github.com/vectordotdev/vector/issues/9550

### Remove high cardinality labels

Remove the `file` label to avoid metrics cardinality explosion.

https://github.com/vectordotdev/vector/issues/11995#issuecomment-1189387421
