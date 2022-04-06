### pdb-daemonset.patch

Supports DaemonSets in disruption controller allowing to control the eviction rate of DaemonSet pods.

[PR#98307](https://github.com/kubernetes/kubernetes/pull/98307)


### Ingress validation patch

Remove ingress class validation on ingress creation. Ingress can work with both annotation and ingressClassName field specified
but validation prevents this like: `Invalid value: \"nginx\": can not be set when the class field is also set"`
