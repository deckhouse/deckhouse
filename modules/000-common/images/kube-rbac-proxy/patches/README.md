# stale-cache.patch
PR has been passed to the upstream and waits to be tested in the real cluster.
https://github.com/brancz/kube-rbac-proxy/pull/59

# config.patch
- Support of defining ExcludePaths and multiple Upstreams in config.
- Config from environment variable `KUBE_RBAC_PROXY_CONFIG`.

# livez.patch
Adds parameter for liveness probes path `--livez-path`.
> Going to send PR to the upstream after testing.
