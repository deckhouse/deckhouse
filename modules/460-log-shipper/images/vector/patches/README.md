## Patches

### Kubernetes Owner Reference

Expand the owner reference if the pod controller is a ReplicaSets or Job, and it also has the owner reference.

ReplicaSets is an internal controller which should not be used directly, so it is not that informative.
Way better is to know which Deployment is the owner of the pod.

Now we are waiting vector to migrate to the [kube-rs](https://github.com/kube-rs/kube-rs) client to adopt our patch and open a PR.

https://github.com/vectordotdev/vector/issues/9550

### Loki Labels

Add the ability to extract objets to Loki labels, e.g., `{"pod_labels":{"app":"server","name":"web"}}` -> `{"pod_labels_app": "server", "pod_labels_name": "web"}`. 

https://github.com/vectordotdev/vector/issues/9549
