## Patches

### 001-certificate_owner_ref.patch

Adds `CertificateOwnerRef` flag to Certificate CRD. `CertificateOwnerRef` flag is whether to set the certificate resource as an owner of a secret where the TLS certificate is stored. When this flag is enabled, the secret will be automatically removed when the certificate resource is deleted.
https://github.com/cert-manager/cert-manager/pull/5158

### 999-fix-cve.patch

Fix CVEs:
- CVE-2026-46600
- CVE-2026-56852
- CVE-2026-56854
- CVE-2026-56855
- CVE-2026-78662
- CVE-2026-81870
- CVE-2026-84303
- CVE-2026-84304
- CVE-2026-84445

GHSA:
- GHSA-gcjh-h69q-9w9g
- GHSA-hrxh-6v49-42gf
- GHSA-mpwr-8vm7-h73f

