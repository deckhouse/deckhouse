## Patches

### 001-lua-info.patch

There are two patches to fix the problem with ingress names in logs and metrics.
Unfortunately, the PR was declined in the upstream.
<https://github.com/kubernetes/ingress-nginx/pull/4367>

### 002-makefile.patch

Run the build locally, not inside the container.

### 003-healthcheck.patch

After catching SIGTERM, ingress stops responding to the readiness probe.
The combination of this patch and the `EndpointSliceTerminatingCondition` feature gate for kube-proxy helps us avoid
traffic loss on rollout updates.
Update: for external load balancers it's advisable to get 5xx if a SIGTERM was sent to the controller, we control this logic by applying/checking `D8s-External-Check` http header.

Backport of the behavior of the later versions of ingress nginx controller.
The `sleep` is needed to gracefully shut down ingress controllers behind a cloud load balancer.

### 004-util.patch

Adds "-e /dev/null" flags to the "nginx -t" invocations so that "nginx -t" logs aren't got saved to /var/log/nginx/error.log file, preventing fs bloating.

### 005-add-http3.patch

Add HTTP/3 support.

We have made two PRs in upstream to bump ingress-nginx image and to enable http3 module.
But we did not add full support for http3 in upstream, because at that time OpenSSL did not fully support quic.

Bump the image and enable http3 module: <https://github.com/kubernetes/ingress-nginx/pull/11470>
README about next steps for upstream: <https://github.com/kubernetes/ingress-nginx/pull/11513>

README: <https://github.com/kubernetes/ingress-nginx/blob/main/images/nginx/README.md>

When OpenSSL fully supports quic, the work can be continued.
To add fully support - steps from the readme should be accomplished and after this the patch can be deleted.

### 006-new-metrics.patch

This patch adds worker max connections, worker processes and worker max open files metrics.

### 007-default-backend-fix.patch

Fixes the problem with the controller when Ingress specifies `Service` with the `ExternalName` type as the main backend, and the default backend (using an annotation `nginx.ingress.kubernetes.io/default-backend`) - with the ClusterIP type. You can see the detailed cases here:
<https://github.com/kubernetes/ingress-nginx/issues/12158>
<https://github.com/kubernetes/ingress-nginx/issues/12173>
<https://github.com/deckhouse/deckhouse/issues/9933>

### 008-balancer-lua.patch

The patch enhances `balancer.lua` to store ingress and service metadata, enabling Nginx to use their names and ports for logging and request routing.

### 009-nginx-tmpl.patch

- Enable our metrics collector instead of the default one.
- Enable pcre_jit.
- Add the health check server to provide the way for an external load balancer to check that the ingress controller will be terminated soon.
- Set default values for upstream_retries and total_upstream_response_time to avoid incorrect logs when it is a raw tcp request.
- Set proxy connect timeout for auth locations.
- Replace the status field with formatted status field which is explicitly converted to number to avoid incorrect logs when response status is 009.

We do not intend to make a PR to the upstream with these changes, because there are only our custom features.

### 010-auth-cookie-always.patch

Without always option toggled, ingress-nginx does not set the cookie in case if backend returns >=400 code, which may lead to dex refresh token invalidation.
Annotation `nginx.ingress.kubernetes.io/auth-always-set-cookie` does not work. Anyway, we can't use it, because we need this behavior for all ingresses.

<https://github.com/kubernetes/ingress-nginx/pull/8213>

### 011-restore-validation.patch

Re-enables configuration validation for the ingress-nginx controller, which was previously disabled as a mitigation for the security vulnerabilities described in CVE-2025-1097, CVE-2025-1098, CVE-2025-1974, CVE-2025-24513, and CVE-2025-24514.

### 012-validation-mode.patch

Slightly tunes some logic related to validating ingress objects.

### 013-verbose-maxmind-logs.patch

Added additional logging when downloading GeoIP databases from the MaxMind service.

### 014-go-mod.patch

Go mod patches for ingress-nginx-controller.

### 015-fix-success-reload-metric.patch

This patch ensures that when an invalid Ingress configuration is deleted, metric `nginx_ingress_controller_config_last_reload_successful` is set to 1.

https://github.com/kubernetes/ingress-nginx/pull/13830

### 016-disable-error-logs.patch

Disabling log messages such as "Error obtaining Endpoints for Service...".

### 017-fix-sorting.patch

There is a sorting issue in a couple of files that causes unnecessary config reloads.

https://github.com/kubernetes/ingress-nginx/pull/14005

### 018-geoip-ver-metric.patch

This patch adds a metric that reflects current GeoIP version in use (0 - no geoip, 1 - geoip, 2 - geoip2)

### 019-skip-tls-verification-maxmind.patch
Adds a flag to disable TLS verification when downloading from the MaxMind mirror.

### 020-ingress-update-status.patch

This patch adds the --status-service-label flag, allowing the controller to define a label selector that is used to match related k8s services. Then, the ip addresses/hostnames from the services' statuses are used to fill in Ingress objects statuses with correct values.

### 021-use-proxy-real-ip-cidr-v2.patch

This patch updates lua ingress script to take into account the `proxy-real-ip-cidr` value when deciding if it's ok to accept x-forwarded headers values or not.

### 022-cve-03022026.patch

Fixes the following CVEs:
CVE-2026-1580
CVE-2026-24512
CVE-2026-24513
CVE-2026-24514
https://groups.google.com/a/kubernetes.io/g/dev/c/9RYJrB8e8ts

### 023-lua_ingress-use-request-host-for-https-redirect.patch

This patch fixes HTTP-to-HTTPS redirect host selection in `lua_ingress`: when `Host` differs from the actual request host (for example with absolute-form requests), redirect now uses the request host (`$host`) instead of raw `Host` header to prevent redirects to arbitrary domains.