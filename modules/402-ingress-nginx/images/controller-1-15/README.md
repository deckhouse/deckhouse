This version of the controller uses nginx `1.29.5`.

We add HTTP/3 support with a local patch, but the protocol is still experimental and is not suitable for production use.
For example, SSL passthrough will not work, and there may be issues with authorization and proxy features.

Upgrade notes:
- Kubernetes version: must be >= 1.30. (github.com)
- Annotations nginx.ingress.kubernetes.io/auth-method: ensure values do not rely on partial matches; anchoring with ^$ changes matching behavior. (github.com)
- Custom server-snippet / location-snippet: escaping/quoting may change behavior. (github.com)
- Large Ingress/ConfigMap: admission limit is 9 MB. (github.com)
- If you use mp4 in NGINX, the bundled nginx `1.29.5` includes the fix for CVE-2024-7347. (openwall.com)
