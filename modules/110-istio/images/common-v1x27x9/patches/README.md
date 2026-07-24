# Patches

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

## 001-kiali-gomod_gosum.patch

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
4. The sidecar container gets `CAP_NET_RAW` and `allowPrivilegeEscalation: true`, and BOTH the `envoy` AND `pilot-agent` binaries are baked with `setcap cap_net_raw=ep` in the dedicated `proxyv2-v1x27x9-mark` image selected by the injection template when the annotation is present (see `proxyv2-v1x27x9-mark/werf.inc.yaml`). The default `proxyv2-v1x27x9` image ships without file capabilities. Envoy needs the capability for its upstream `SO_MARK` socket option; pilot-agent needs it for the DNS forward socket `SO_MARK` (see `newUpstreamDNSClient` in `pkg/dns/client/proxy.go`) — without it, DNS resolution of istiod fails with `EPERM` on `setsockopt(SO_MARK)` and the sidecar cannot bootstrap. Both binaries pick up the capability at exec via the normal file-capability rule (`F(permitted) & P(bounding)`) — no Go code changes needed in pilot-agent itself. Since kernel 5.17, `CAP_NET_RAW` is sufficient for `setsockopt(SO_MARK)` (a much narrower capability than `CAP_NET_ADMIN`); older kernels would require `CAP_NET_ADMIN`.
5. Marked packets bypass the redirect, so no infinite loop even when sidecar and app share the same UID/GID.

Applies on top of 003. Do not mix with TPROXY (mark-mode is REDIRECT-only). DNS capture works in mark mode: the istio-agent's DNS forward socket is also tagged with the same `SO_MARK` (see `pkg/dns/client/proxy.go`), so port 53 is captured normally and ServiceEntry/auto-allocate names resolve. Only the istiod bootstrap ports (15012, 15017) are excluded from redirection. The `SO_MARK` on the DNS forward socket requires `CAP_NET_RAW`, which pilot-agent gets via the same `setcap cap_net_raw=ep /pilot-agent` baked into the mark image (see point 4 above).

Note on the capability mechanism: an ambient-capabilities approach (raising `CAP_NET_RAW` into `pilot-agent`'s ambient set in Go before exec'ing Envoy, with no image/template changes) was tried first, since it doesn't require `allowPrivilegeEscalation: true`. It failed in practice: `prctl(PR_CAP_AMBIENT_RAISE)` requires the capability to already be in both the permitted *and* inheritable sets of the calling process, and this cluster's container runtime only populates the bounding set from `securityContext.capabilities.add`, leaving inheritable empty — the raise fails with EPERM every time (confirmed on a live pod via `/proc/<pid>/status`). File capabilities avoid this entirely since they only depend on the bounding set, at the cost of requiring `allowPrivilegeEscalation: true` (file capabilities are otherwise ignored at exec under `no_new_privs`).

Build gotcha: werf's `import` DOES preserve the `security.capability` xattr, but specifying `owner:`/`group:` on the import triggers a chown that strips it (same gotcha as Docker `COPY --chown`). Baking `setcap` in the `-envoy-marked-artifact` and then importing the binaries with `owner: 64535` dropped the capability — confirmed on a live pod as `CapEff=0`, with the sidecar failing to resolve istiod (`transport: Error while dialing: ... i/o timeout`) once port 53 was captured. Fix: keep the setcap'd binaries `root:root 0755` in the artifact and import them WITHOUT `owner:`/`group:` (uid 64535 still executes a world-executable root-owned binary). This mirrors the working `caps-deckhouse-controller` import in `.werf/werf-deckhouse-controller.yaml`.
