1. The code in this directory has been copied from: https://github.com/kubernetes/kubernetes/blob/v1.29.10/staging/src/k8s.io/kubectl/pkg/drain
Tag 0.29.10


!!!Attention!!!
This version is patched to ignore kruise AdvancedDaemonSetPods.
https://github.com/kubernetes/kubernetes/issues/101557
https://github.com/kubernetes/kubernetes/pull/128779
https://github.com/openkruise/kruise/issues/1831

!!!Attention!!!
This version also retries an eviction that failed with a transient error (an unreachable admission
webhook surfaces as an internal error, an API server may time out). Upstream ends the pod's eviction
on any error other than 429, so a single failed second disqualifies the pod for the rest of the
drain while the node is still reported as drained once the global timeout expires.

2. helper.go
Transfer the wrapper to the location where it is used after accepting the PR above.
