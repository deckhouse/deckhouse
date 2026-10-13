## Patches

### 001-certificate_owner_ref.patch

Adds `CertificateOwnerRef` flag to Certificate CRD. `CertificateOwnerRef` flag is whether to set the certificate resource as an owner of a secret where the TLS certificate is stored. When this flag is enabled, the secret will be automatically removed when the certificate resource is deleted.
https://github.com/cert-manager/cert-manager/pull/5158

The patch also adds the `gen.SetCertificateOwnerRef` test helper
and unit tests that check how `CertificateOwnerRef` overrides the `--enable-certificate-owner-ref` flag.

The post-issuance policy checks `SecretOwnerReferenceMismatch` and `SecretOwnerReferenceManagedFieldMismatch`
take the same effective value: `CertificateOwnerRef` when it is set, otherwise the flag.
Without this, a certificate whose `CertificateOwnerRef` differs from the flag triggers a Secret update on every reconcile,
and an owner reference requested in `CertificateOwnerRef` reaches an existing Secret only on the next issuance.

### 002-solver_test_ipv6_host.patch

Encloses the IPv6 host in square brackets in the request URL of the `TestSolver` HTTP-01 solver test.
Starting with the `go 1.26` directive that `999-fix-cve.patch` sets, `net/url` rejects an IPv6 host without brackets.
The change is a backport of upstream commit
[127488f](https://github.com/cert-manager/cert-manager/commit/127488f59fb627c0653f0cb1f94b213021629ea2)
and affects the test only.

### 003-gateway_discovery_retry.patch

Retries the Gateway API discovery at controller startup instead of exiting on the first error.
Upstream exits when the API server does not answer the discovery request, so a controller that starts
during an API server outage goes into `CrashLoopBackOff` and, once the API server is back, waits for
the kubelet back-off of up to five minutes.
The patch retries transient errors with a back-off of up to 10 seconds for at most five minutes.
The retries stop as soon as the controller receives a shutdown signal, so a pod deleted while it waits
exits at once instead of waiting for `SIGKILL`.
A `NotFound` answer is retried for one minute: a kube-apiserver that has just started answers `NotFound`
for a served CRD group until its discovery controller catches up, while a lasting `NotFound` means the
Gateway API CRDs are absent and still fails.
Unit tests cover the transient errors, a transient `NotFound` and a lasting one.

The check runs only when the module passes `--enable-gateway-api=true`, that is, when the cluster
serves the Gateway API CRDs; without them the patch changes nothing.
The health endpoint of the controller starts after this check, so the Deployment carries a startup
probe long enough for the whole retry period.

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

