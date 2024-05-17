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
* Set default values for upstream_retries and total_upstream_response_time to avoid incorrect logs when it is a raw tcp request.
* Replace the status field with formatted status field which is explicitly converted to number to avoid incorrect logs when response status is 009.

We do not intend to make a PR to the upstream with these changes, because there are only our custom features.

### Ingress information

There are two patches to fix the problem with ingress names in logs and metrics.
Unfortunately, the PR was declined in the upstream.
https://github.com/kubernetes/ingress-nginx/pull/4367

### Makefile

Run the build locally, not inside the container.

### metrics SetSSLExpireTime

Fixes namespace which is given by metric nginx_ingress_controller_ssl_expire_time_seconds.

https://github.com/kubernetes/ingress-nginx/pull/10274

### Always set auth cookie

Without always option toggled, ingress-nginx does not set the cookie in case if backend returns >=400 code, which may lead to dex refresh token invalidation.
Annotation `nginx.ingress.kubernetes.io/auth-always-set-cookie` does not work. Anyway, we can't use it, because we need this behavior for all ingresses.

https://github.com/kubernetes/ingress-nginx/pull/8213

### Endpointslice missing ready conditions

Ingress controller panics if there is an ingress, pointing to a service with the endpointslice lacking 'condtitions.ready' field. Fixed in 1.7.

https://github.com/kubernetes/ingress-nginx/pull/9550/files


### Fix cleanup

Fix tmpDir path for the cleanup procedure.

https://github.com/kubernetes/ingress-nginx/pull/10797

### nginx-build

Build nginx for controller on ALT Linux.
