# Patches

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

## 001-kiali-go-mod.patch

Fix Kiali CVE vulnerabilities

## 002-istio-multicluster-regenerate-token-timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 002-kiali-logout.patch

Enable Logout in Kiali for header auth (DexAuthenticator). The tab that clicks Logout calls `/logout?rd=<app-origin>/` once; other tabs receive a `localStorage` event and only dispatch `sessionExpired` locally (no second sign_out, no reload) to avoid oauth2-proxy CSRF races.

## 003-change-to-deckhouse-user.patch

Change runAsUser from 1337 to 64535 in istio templates, changed istio-init.iptables user arg to both 1337 and 64535 UIDs in injection-template.yaml

## 004-mark-interception.patch

Add mark-based outbound interception for pods where the sidecar must run with the same UID/GID as the application container (e.g. ingress-nginx under deckhouse, both UID 64535). With the dual-uid approach from 003 the app's own outbound traffic is also excluded from redirect, breaking interception.

How it works: when annotation `traffic.sidecar.istio.io/outboundSocketMark` (e.g. "1338") is set on a pod:
1. The init container gets `--outbound-mark 1338` and `ISTIO_META_OUTBOUND_MARK=1338` env.
2. istio-iptables installs a `mark match 0x53a -j RETURN` rule + port exclusions (53, 15012, 15017) instead of owner-based UID/GID rules.
3. istiod adds `SO_MARK=1338` socket option (Level=SOL_SOCKET, Name=SO_MARK) to all upstream clusters, so Envoy's own outbound connections are tagged with fwmark 0x53a.
4. The sidecar container gets `CAP_NET_ADMIN` (required for `setsockopt(SO_MARK)`).
5. Marked packets bypass the redirect, so no infinite loop even when sidecar and app share the same UID/GID.

Applies on top of 003. Do not mix with TPROXY (mark-mode is REDIRECT-only).
