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

## 003-istio-fix-invalid-mtls-port-assignment-in-hbone-gateway.patch

Backport of https://github.com/istio/istio/pull/61619

## 004-istio-multicluster-remote-trust-domain-fallback.patch

Use the local trust domain for a remote multicluster peer that has no mesh ConfigMap of this revision.

Since Istio 1.29 istiod reads the trust domain of a remote cluster from that cluster's `istio-<revision>` ConfigMap
(https://github.com/istio/istio/pull/58455). The ConfigMap exists only if the remote cluster has the same revision installed,
so during a staged upgrade the remote cluster usually has none. Then istiod used the default mesh config with trust domain
`cluster.local`, and the fallback to the local mesh config (https://github.com/istio/istio/pull/59476) never fired.
Envoy expected `spiffe://cluster.local/...` in the certificates of Services that exist only in the remote cluster,
and requests to them failed with 503 `CERTIFICATE_VERIFY_FAILED`.

With the patch, while the remote ConfigMap is missing or unreadable, the remote mesh watcher has no value
and istiod uses the local trust domain. Not fixed upstream as of 1.30.3 and master.

> [!WARNING]
> The patch has no unit test. Carry it to every Istio version from 1.29 on (1.30 included) until upstream fixes it.
> On a version bump:
>
> - Check whether upstream already fixed it: the remote mesh watcher built in `NewMulticluster`
>   (`pilot/pkg/serviceregistry/kube/controller/multicluster.go`) has no value for a missing remote ConfigMap,
>   or `restrictedConfigAdapter.TrustDomain` (`pkg/config/mesh/watchers.go`) falls back to the local trust domain
>   in another way. If so, drop the patch.
> - If the patch still applies, verify it by hand or by the alliance e2e checks that call a Service existing only
>   in the remote peer and require `DataplaneConnectionReady=True` on `IstioMulticluster`. They catch the bug only
>   with mixed versions: an Istio 1.29+ client and a remote cluster without the same revision (no `istio-<revision>`
>   ConfigMap). A Service that exists in both peers with the same namespace and service account does not show the bug,
>   because istiod merges the SAN lists of both peers.
