## Patches

### Kubernetes Logs Lib

Expand the owner reference if the pod controller is a ReplicaSets or Job, and it also has the owner reference.

ReplicaSets is an internal controller which should not be used directly, so it is not that informative.
Way better is to know which Deployment is the owner of the pod.

### Loki Labels

Makes the extraLabels option works for Loki.
