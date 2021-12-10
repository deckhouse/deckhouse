## Patches

### Reason

Properly show the reason in templates.
It was accepted to the upstream.
https://github.com/kubernetes/ingress-nginx/pull/5213

### Omit helm secrets

The problem is that the ingress nginx controller subscribes to all secrets.
If there are a lot of helm secrets in the cluster, kube-apiserver will consume a significant amount of memory.
It was accepted to the upstream.
https://github.com/kubernetes/ingress-nginx/pull/5702

### Nginx TPL

* Enable our metrics collector instead of the default one.
* Enable pcre_jit.
* Add the health check server to provide the way for an external load balancer to check that the ingress controller will be terminated soon.

We do not intend to make a PR to the upstream with these changes, because there are only our custom features.

### Ingress information

There are two patches to fix the problem with ingress names in logs and metrics.
Unfortunately, the PR was declined in the upstream.
https://github.com/kubernetes/ingress-nginx/pull/4367

### Pod IP

This is a small patch to fix the problem with the ingress controller pod listening on 0.0.0.0 in the host network mode.
Our PR was reverted in the upstream.
https://github.com/kubernetes/ingress-nginx/issues/2262
