## Patches

### Healthcheck

After catching SIGTERM, ingress stops responding to the readiness probe. 
The combination of this patch and the `EndpointSliceTerminatingCondition` feature gate for kube-proxy helps us avoid 
traffic loss on rollout updates. 

We also have backported the behavior of the later versions of ingress nginx controller.
The `sleep` is needed to gracefully shut down ingress controllers behind a cloud load balancer.

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

### Always set auth cookie

Without always option toggled, ingress-nginx does not set the cookie in case if backend returns >=400 code, which may lead to dex refresh token invalidation.

https://github.com/kubernetes/ingress-nginx/pull/8213

### Deny locations with the invalid auth URL

There is a problem, that when you set an invalid URL as a value for the nginx.ingress.kubernetes.io/auth-url annotation, the ingress controller allows accessing upstreams without authentication.
Thus, this is considered a security hole. Users can accidentally set an invalid URL by hand or because of an error in helm templates.

https://github.com/kubernetes/ingress-nginx/pull/8256

### Ingress class
If ingress class annotation or spec field is not set, fallback to default class("nginx") and check it
