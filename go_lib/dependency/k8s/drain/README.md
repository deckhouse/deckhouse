1. The code in this directory has been copied from: https://github.com/kubernetes/kubernetes/blob/v1.29.10/staging/src/k8s.io/kubectl/pkg/drain
Tag 0.29.10


!!!Attention!!!
This version is patched to ignore kruise AdvancedDaemonSetPods.
https://github.com/kubernetes/kubernetes/issues/101557
https://github.com/kubernetes/kubernetes/pull/128779
https://github.com/openkruise/kruise/issues/1831

2. helper.go
Transfer the wrapper to the location where it is used after accepting the PR above.
