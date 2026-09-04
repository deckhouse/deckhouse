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

GHSA:
- GHSA-hrxh-6v49-42gf
