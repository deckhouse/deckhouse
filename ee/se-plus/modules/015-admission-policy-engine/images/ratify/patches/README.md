## Patches

### 001-add-custom-ca.patch

This patch provides the way of setting default secure http transport for the Oras client to support multiple custom CA when accessing private registries.

### 002-fix-cve.patch

Fixes CVEs, including:

- GHSA-pmwq-pjrm-6p5r (`github.com/in-toto/in-toto-golang` -> `v0.11.0`)
- CVE-2026-49478 (`github.com/sigstore/fulcio` -> `v1.8.6`)
- CVE-2026-48702 (`github.com/sigstore/rekor` -> `v1.5.2`)
- CVE-2026-49834, CVE-2026-54787 (`github.com/sigstore/sigstore-go` -> `v1.2.1`)
- CVE-2026-49835 (`github.com/sigstore/timestamp-authority/v2` -> `v2.1.2`)
- CVE-2026-39828 (`golang.org/x/crypto` -> `v0.53.0`)
- CVE-2026-25680, CVE-2026-25681, CVE-2026-33814, CVE-2026-39821, CVE-2026-46600 (`golang.org/x/net` -> `v0.56.0`)
- CVE-2026-39824 (`golang.org/x/sys` -> `v0.46.0`)
- CVE-2026-56852 (`golang.org/x/text` -> `v0.39.0`)
- GHSA-hrxh-6v49-42gf (`google.golang.org/grpc` -> `v1.82.1`)
- CVE-2026-50151, CVE-2026-50162, CVE-2026-50163 (`oras.land/oras-go/v2` -> `v2.6.2`)
