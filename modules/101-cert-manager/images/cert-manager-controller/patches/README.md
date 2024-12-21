## Patches

### go-mod.patch

Bump libraries versions to fix security errors.

### Certificate owner ref

Adds `CertificateOwnerRef` flag to Certificate CRD. `CertificateOwnerRef` flag is whether to set the certificate resource as an owner of a secret where the TLS certificate is stored. When this flag is enabled, the secret will be automatically removed when the certificate resource is deleted.
https://github.com/cert-manager/cert-manager/pull/5158
