This version of the controller has nginx 1.27.1,
which supports HTTP3, but it has not been added yet.

We add HTTP3, but the protocol is in experimental state, it is not suitable for use in production.
For example, the passthrough mechanism will not work, and there may be issues with authorization and proxy functions.

Upgrade notes:
- Kubernetes version: must be >= 1.30. (github.com)
- Annotations nginx.ingress.kubernetes.io/auth-method: ensure values do not rely on partial matches; anchoring with ^$ changes matching behavior. (github.com)
- Custom server-snippet / location-snippet: escaping/quoting may change behavior. (github.com)
- Large Ingress/ConfigMap: admission limit is 9 MB. (github.com)
- If you use mp4 in NGINX, 1.27.1 fixes CVE-2024-7347. (openwall.com)
