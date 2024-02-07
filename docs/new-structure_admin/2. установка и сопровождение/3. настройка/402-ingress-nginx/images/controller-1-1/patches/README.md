## Patches

### Healthcheck

After catching SIGTERM, ingress stops responding to the readiness probe.
The combination of this patch and the `EndpointSliceTerminatingCondition` feature gate for kube-proxy helps us avoid
traffic loss on rollout updates.
Update: for external load balancers it's advisable to get 5xx if a SIGTERM was sent to the controller, we control this logic by applying/checking `D8s-External-Check` http header.

Backport of the behavior of the later versions of ingress nginx controller.
The `sleep` is needed to gracefully shut down ingress controllers behind a cloud load balancer.

### Nginx TPL

* Enable our metrics collector instead of the default one.
* Enable pcre_jit.
* Add the health check server to provide the way for an external load balancer to check that the ingress controller will be terminated soon.

We do not intend to make a PR to the upstream with these changes, because there are only our custom features.

### Ingress information

There are two patches to fix the problem with ingress names in logs and metrics.
Unfortunately, the PR was declined in the upstream.
https://github.com/kubernetes/ingress-nginx/pull/4367

### Makefile

Run the build locally, not inside the container.

### Always set auth cookie

Without always option toggled, ingress-nginx does not set the cookie in case if backend returns >=400 code, which may lead to dex refresh token invalidation.

https://github.com/kubernetes/ingress-nginx/pull/8213

### Nginx tmpl backport from 1.2

Without this fix, redirects don't work if using behindL7Proxy controller and a load balancer without X-Forwarded-Proto header. In this case, ingress-nginx redirects to nil://example.com/abc.
Backported from 1.2 version.

https://github.com/kubernetes/ingress-nginx/pull/8468

### metrics SetSSLExpireTime

Fixes namespace which is given by metric nginx_ingress_controller_ssl_expire_time_seconds.

https://github.com/kubernetes/ingress-nginx/pull/10274
