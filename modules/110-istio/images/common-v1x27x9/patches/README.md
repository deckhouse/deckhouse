# Patches

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

## 001-kiali-gomod_gosum.patch

Fix Kiali CVE vulnerabilities

## 002-istio-multicluster-regenerate-token-timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 002-kiali-logout.patch

Enable Logout in Kiali for header auth (DexAuthenticator) and redirect to `/logout?rd=<app-origin>/` so sign-out returns to the app and avoids stale Dex login CSRF.
