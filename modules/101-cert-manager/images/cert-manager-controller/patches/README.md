## Patches

### 001-certificate_owner_ref.patch

Adds `CertificateOwnerRef` flag to Certificate CRD. `CertificateOwnerRef` flag is whether to set the certificate resource as an owner of a secret where the TLS certificate is stored. When this flag is enabled, the secret will be automatically removed when the certificate resource is deleted.
https://github.com/cert-manager/cert-manager/pull/5158

The patch also adds the `gen.SetCertificateOwnerRef` test helper
and unit tests that check how `CertificateOwnerRef` overrides the `--enable-certificate-owner-ref` flag.

### 002-solver_test_ipv6_host.patch

Encloses the IPv6 host in square brackets in the request URL of the `TestSolver` HTTP-01 solver test.
Starting with the `go 1.26` directive that `999-fix-cve.patch` sets, `net/url` rejects an IPv6 host without brackets.
The change is a backport of upstream commit
[127488f](https://github.com/cert-manager/cert-manager/commit/127488f59fb627c0653f0cb1f94b213021629ea2)
and affects the test only.

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

