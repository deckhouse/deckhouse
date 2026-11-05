# Patches

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

Also includes a backport of upstream commit `72fd7c2b9a` (https://github.com/istio/istio/pull/56139, in Istio 1.25.3): migrates Istio from `go-jose/v3`, which has no fixed releases, to `go-jose/v4`. Drop the code part of the patch when Istio is updated to 1.25.3 or later.

To regenerate the patch, apply it to a clean Istio checkout with the index:

```shell
git apply --index 001-istio-gomod_gosum.patch
# bump dependencies, then go mod tidy
git add -A && git diff --cached > 001-istio-gomod_gosum.patch
```

Do not apply `002-istio-multicluster-regenerate-token-timeout.patch` before taking the diff.

## 001-kiali-go-mod.patch

Fix Kiali CVE vulnerabilities

Also migrates Kiali from `go-jose` v2 to `go-jose/v4` and includes a backport of upstream pr https://github.com/kiali/kiali/pull/8380 (commit `27f192ebe`), adapted to Kiali v2.7.1. The backport adds the new file `jwt/jwt.go`. Drop the code part of the patch when Kiali is updated to v2.14.0 or later, where it is already upstream.

To regenerate the patch, apply it to a clean Kiali checkout with the index, so that the new file is included in the diff:

```shell
git apply --index 001-kiali-go-mod.patch
# bump dependencies, then go mod tidy
git add -A && git diff --cached > 001-kiali-go-mod.patch
```

Do not apply `002-kiali-logout.patch` before taking the diff.

## 002-istio-multicluster-regenerate-token-timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 002-kiali-logout.patch

Enable Logout in Kiali for header auth (DexAuthenticator). The tab that clicks Logout calls `/logout?rd=<app-origin>/` once; other tabs receive a `localStorage` event and only dispatch `sessionExpired` locally (no second sign_out, no reload) to avoid oauth2-proxy CSRF races.
