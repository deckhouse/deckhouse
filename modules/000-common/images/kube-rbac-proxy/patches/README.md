## Patches

### 001-stale-cache.patch

PR has been passed to the upstream and waits to be tested in the real cluster.
https://github.com/brancz/kube-rbac-proxy/pull/59

### 002-config.patch
- Support of defining ExcludePaths and multiple Upstreams in config.
- Config from environment variable `KUBE_RBAC_PROXY_CONFIG`.

### 003-livez.patch
Adds parameter for liveness probes path `--livez-path`.

### 004-insecure-upstream.patch
Do not check upstream TLS certificate.

### 005-preserve-auth-header.patch
Propagate the `Authorization` header to upstream.

### 006-secure-listen-address.patch
Check if the --secure-listen-address flag is set.

### 007-go-mod.patch
Update Go module dependencies for the image build.

### 999-fix-cve.patch

Fix CVEs:
- CVE-2026-25680
- CVE-2026-25681
- CVE-2026-27136
- CVE-2026-33186
- CVE-2026-33814
- CVE-2026-39821
- CVE-2026-39824
- CVE-2026-39827
- CVE-2026-39828
- CVE-2026-39829
- CVE-2026-39830
- CVE-2026-39831
- CVE-2026-39832
- CVE-2026-39833
- CVE-2026-39834
- CVE-2026-39835
- CVE-2026-42502
- CVE-2026-42506
- CVE-2026-42508
- CVE-2026-46595
- CVE-2026-46597
- CVE-2026-46598
- CVE-2026-46600
- CVE-2026-56852
- CVE-2026-56854
- CVE-2026-56855
- CVE-2026-78662
- CVE-2026-84303
- CVE-2026-84304
- CVE-2026-84445

GHSA:
- GHSA-hrxh-6v49-42gf

<!-- cvefix:begin -->
### Dependency overrides

`cvefix` from modules-ci maintains this section from `999-fix-cve.patch`. Do not edit it by hand.
It lists the `go.mod` requirements that the patch set changes over the upstream release.

| Module | Upstream | Patched |
|---|---|---|
| `github.com/cespare/xxhash/v2` | v2.2.0 | v2.3.0 |
| `github.com/go-logr/logr` | v0.2.0 | v1.4.3 |
| `github.com/golang/protobuf` | v1.5.3 | v1.5.4 |
| `github.com/google/go-cmp` | v0.6.0 | v0.7.0 |
| `github.com/google/uuid` | v1.3.0 | v1.6.0 |
| `golang.org/x/crypto` | v0.45.0 | v0.57.0 |
| `golang.org/x/net` | v0.47.0 | v0.60.0 |
| `golang.org/x/oauth2` | v0.27.0 | v0.36.0 |
| `golang.org/x/sync` | v0.18.0 | v0.23.0 |
| `golang.org/x/sys` | v0.38.0 | v0.48.0 |
| `golang.org/x/term` | v0.37.0 | v0.46.0 |
| `golang.org/x/text` | v0.31.0 | v0.42.0 |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20230711160842-782d3b101e98 | v0.0.0-20260526163538-3dc84a4a5aaa |
| `google.golang.org/grpc` | v1.58.3 | v1.83.2 |
| `google.golang.org/protobuf` | v1.33.0 | v1.36.11 |
| `k8s.io/klog/v2` | v2.3.0 | v2.130.1 |

The `go` directive of `go.mod` is raised from 1.24.0 to 1.26.0: the patched dependencies require it.
<!-- cvefix:end -->
