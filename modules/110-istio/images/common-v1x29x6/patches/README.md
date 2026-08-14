# Patches

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

## 001-kiali-gomod_gosum.patch

Fix Kiali CVE vulnerabilities.

Bumps `k8s.io/*` to v0.36.3 and `sigs.k8s.io/controller-runtime` to v0.24.1
(controller-runtime v0.22.x is incompatible with client-go v0.36 — missing
`HasSyncedChecker` on `ResourceEventHandlerRegistration`).

## 002-istio-multicluster-regenerate-token-timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 002-kiali-logout.patch

Enable Logout in Kiali for header auth (DexAuthenticator). The tab that clicks Logout calls `/logout?rd=<app-origin>/` once; other tabs receive a `localStorage` event and only dispatch `sessionExpired` locally (no second sign_out, no reload) to avoid oauth2-proxy CSRF races.

## 003-istio-crl-only-verify-leaf.patch

Wire `D8_CRL_ONLY_VERIFY_LEAF=true` into Envoy `only_verify_leaf_cert_crl` on
pilot `default_validation_context` (server SDS, ISTIO_MUTUAL clusters, and
credential/file upstream TLS). Combined with agent SDS CRL via boolean OR merge.

Mapped from ModuleConfig `settings.crl.enableFullCrlChainCheck=false`.
