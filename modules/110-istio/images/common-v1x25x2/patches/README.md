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
=======
## 003-change-to-deckhouse-user.patch

Change runAsUser from 1337 to 64535 in istio templates, changed istio-init.iptables user arg to both 1337 and 64535 UIDs in injection-template.yaml

## 004-mark-interception.patch

Add mark-based outbound interception as an opt-in alternative to --uid-owner, for pods where the sidecar must run with the same UID/GID as the application container (e.g. ingress-nginx under deckhouse, both UID 64535). With the dual-uid approach from 003 the app's own outbound traffic is also excluded from redirect, breaking interception. When `traffic.sidecar.istio.io/outboundSocketMark` is set, Envoy tags its outbound sockets with SO_MARK and iptables excludes marked packets via `-m mark --mark <mark> -j RETURN`, distinguishing Envoy from the app by socket mark. Applies on top of 003.
