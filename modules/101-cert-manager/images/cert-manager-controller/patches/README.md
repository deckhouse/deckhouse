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

