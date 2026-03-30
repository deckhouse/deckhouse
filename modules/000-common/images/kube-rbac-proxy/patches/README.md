## Patches

### 001-stale-cache

PR has been passed to the upstream and waits to be tested in the real cluster.
https://github.com/brancz/kube-rbac-proxy/pull/59

### 002-config
- Support of defining ExcludePaths and multiple Upstreams in config.
- Config from environment variable `KUBE_RBAC_PROXY_CONFIG`.

#### 003-livez
Adds parameter for liveness probes path `--livez-path`.

#### 004-insecure-upstream
Do not check upstream TLS certificate.

#### 005-preserve-auth-header
Propagate the `Authorization` header to upstream.

### 006-secure-listen-address
Check if the --secure-listen-address flag is set.

### 007-go-mod

Fix CVEs
