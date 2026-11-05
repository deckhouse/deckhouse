# Patches

## 001-istio-apply_go.patch

Fix Istio Operator healt status

## 001-kiali-go-mod.patch

Fix CVE

## 002-istio-gomod_gosum.patch

Fix CVE

Also includes an adaptation of upstream commit `72fd7c2b9a` (https://github.com/istio/istio/pull/56139, in Istio 1.25.3): migrates Istio from `go-jose/v3`, which has no fixed releases, to `go-jose/v4`, and bumps `coreos/go-oidc/v3` to v3.12.0 (go-oidc depends on `go-jose/v4` instead of `go-jose/v3` since v3.10.0). Upstream has no such fix for Istio 1.21.

To regenerate the patch, apply `001-istio-apply_go.patch` to a clean Istio checkout and commit it, then apply this patch with the index:

```shell
git apply --index 002-istio-gomod_gosum.patch
# bump dependencies, then go mod tidy
git add -A && git diff --cached HEAD > 002-istio-gomod_gosum.patch
```

Do not apply `003-*` and later patches before taking the diff.

## 002-kiali-logout.patch

Enable Logout in Kiali for header auth (DexAuthenticator). The tab that clicks Logout calls `/logout?rd=<app-origin>/` once; other tabs receive a `localStorage` event and only dispatch `sessionExpired` locally (no second sign_out, no reload) to avoid oauth2-proxy CSRF races.

## 003-istio-server_fmtText.patch

Fix use expfmt library in pilot-agent. This library used for format metrics.

> [!WARNING]
> **After update istio to version 1.22.X and above need delete this patch!**

## 004-istio-multicluster_regenerate_token_timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 005-istio-init-readonly-rootfs.patch

Set `readOnlyRootFilesystem: true` for the `istio-init` container in the sidecar injection template (`InitContainer` mode).
Required for clusters that enforce read-only root filesystem via SecurityPolicy/PSS (e.g. CSE). Safe for the standard Deckhouse `proxyv2` image: `iptables-wrapper` selects nft in the pod network namespace, so `/run/xtables.lock` is not required.
