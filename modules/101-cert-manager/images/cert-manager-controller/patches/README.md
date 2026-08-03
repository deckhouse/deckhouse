## Patches

### 001-certificate_owner_ref.patch

Adds `CertificateOwnerRef` flag to Certificate CRD. `CertificateOwnerRef` flag is whether to set the certificate resource as an owner of a secret where the TLS certificate is stored. When this flag is enabled, the secret will be automatically removed when the certificate resource is deleted.
https://github.com/cert-manager/cert-manager/pull/5158

### 999-fix-cve.patch

Fix CVEs:
- CVE-2026-46600
- CVE-2026-56852

GHSA:
- GHSA-hrxh-6v49-42gf

Note: stdlib CVEs requiring Go >= 1.25.12 (CVE-2026-27145, CVE-2026-39822, CVE-2026-42504, CVE-2026-42505, CVE-2026-42507) remain until builder/golang is bumped past 1.25.10 (GOTOOLCHAIN=local).
