## Patches

### 001-certificate_owner_ref.patch

Adds `CertificateOwnerRef` flag to Certificate CRD. `CertificateOwnerRef` flag is whether to set the certificate resource as an owner of a secret where the TLS certificate is stored. When this flag is enabled, the secret will be automatically removed when the certificate resource is deleted.
https://github.com/cert-manager/cert-manager/pull/5158

### 002-FixCVE.patch

Fixes:
CVE-2025-22870
CVE-2025-22872
CVE-2025-22869
CVE-2025-22868
CVE-2025-27144
CVE-2025-30204
CVE-2025-47914
CVE-2025-58181
CVE-2026-33186
CVE-2026-34986
CVE-2026-39883
CVE-2026-39828
CVE-2026-25680
CVE-2026-25681
CVE-2026-33814
CVE-2026-39821
CVE-2026-46600
CVE-2026-39824
CVE-2026-56852
GHSA-hrxh-6v49-42gf
