### kube-scheduler-fix-int-divide-by-zero.patch

Fixed a bug in the scheduler where it would crash when prefilter returns a non-existent node.
https://github.com/kubernetes/kubernetes/issues/124930

TODO: Delete this patch after version k8s 1.27.15
