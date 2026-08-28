# Patches

## 000-go-mod.patch

Bumps a set of Go module dependencies in `go.mod`/`go.sum` above the versions
shipped by upstream Cilium v1.17.17 to remediate known CVEs, e.g.:

- `github.com/go-jose/go-jose/v4` -> `v4.1.4`
- `github.com/envoyproxy/go-control-plane/envoy` -> `v1.37.0`
- `go.mongodb.org/mongo-driver` -> `v1.17.7`
- `github.com/cilium/ebpf` -> `v0.22.0` (CVE-2026-10722)

Regenerate by cloning `cilium v1.17.17`, applying this patch, bumping the
required module(s), then running `go mod tidy && go mod vendor && go mod verify`
(with `GOTOOLCHAIN=go1.25.11`) and diffing `go.mod`/`go.sum`.

## 001-request-ip.patch

Add the oportunity to request specific IP-address using annotation:

    cni.cilium.io/ipAddress: 10.10.10.10

Upstream <https://github.com/cilium/cilium/pull/24098>
Possible feature for refactoring <https://docs.cilium.io/en/v1.14/network/concepts/ipam/multi-pool/>

## 002-stable-mac.patch

Use predefined MAC-addresses for virtualization workloads

Upstream <https://github.com/cilium/cilium/pull/24100>

It needs to be changed to <https://docs.cilium.io/en/latest/network/pod-mac-address/#pod-mac-address>

## 003-mtu.patch

Set correct MTU value for veth interfaces

Upstream issue <https://github.com/cilium/cilium/issues/23711>

## 005-ebpf-dhcp-server.patch

Added DHCP server for pods (ebpf implementation).

## 006-add-pod-prioroty-management.patch

Added a `network.deckhouse.io/pod-common-ip-priority` label allows you to share a single IP between  several Pods and to switch the actual owner.

## 007-fix-restoring-cep-for-dead-local-endpoint.patch

Fixed bug when agent uses CiliumEndpoint cache for dead local endpoints.

## 008-hide-error-of-incompatibility-of-egw-with-ces.patch

In the PR <https://github.com/cilium/cilium/pull/27984>, an error has been introduced if `CES` and `EGW` are enabled together, as some of the features are not functioning correctly.

While we were previously satisfied with the older behavior, the agent is now unable to start due to this error.

Please remove this change after `CES` becomes Stable. <https://github.com/cilium/cilium/issues/31904#issuecomment-2647858564>

## 009-wireguard-port.patch

Changing the hardcoded wireguard port from `51871` to `4287` (a port within our range).

## 011-bpf-lb-use-random-lb-algo-for-hostport-serives-fixed.patch

For HostPort pseudo-serivces always use random LB algo. When bpf-lb-algorithm-annotation feature activated - use default LB algo if it incorrectly choosed in service map.

## 012-add-least-conn-lb-algorithm.patch

Added an implementation of the Least Connections load balancing algorithm.

## 013-ignore-egress-gateway-inactual-warning.patch

Ignore error when using IPv4 address for egress gateway other than assigned

## 014-kernel-verifier-stat.patch

Added kernel verifer statistics in logs and prometeus metric as max bpf program complexity

## 015-cleanup-conntrack-endpoints.patch

Add new conntrack cleanup logic for vm pods.

## 016-add-import-export-conntrack-http-endpoints.patch

Add import/export conntrack http endpoints. See usage example here modules/021-cni-cilium/docs/internal/DVP_INTEGRATION.md.

## 017-bpf-lb-generate-icmp-reply.patch

An ICMP echo reply feature has been added to reply on LoadBalancer's service IP

## 018-fix-svacer.patch

Fixed svacer DEREF_OF_NULL error in pkg/policy/api/icmp.go

## 019-ipcache-no-deadlock-on-label-injection.patch

Backport for Cilium 1.17.x fixing an agent-wide deadlock in ipcache label injection:
`doInjectLabels()` holds `ipc.mutex` while `UpdatePolicyMaps()` waits for all endpoints,
yet endpoint regeneration acquires the same mutex (`GetDNSRules` → `LookupByIdentity`),
so under identity churn regeneration stops node-wide, CNI requests pile up, the
`endpoint-create` limiter returns `429 putEndpointIdTooManyRequests` and liveness stays
green. The patch moves the delete-path `UpdatePolicyMaps()` call out of the critical
section, as the add path above it already does, and bounds the previously unbounded wait
with `ctx` and a 3-minute timeout.

**Remove this patch when upgrading to Cilium 1.18 or newer**, where upstream fixed the
same deadlock (`cilium#39970`, commit `47ace2de6`) by removing
`IPCache.UpdatePolicyMaps()` altogether, so the patch is both unnecessary and will fail
to apply.

## 020-policy-nil-safe-selector-policy-detach.patch

Fixes an agent crash (`SIGSEGV` in `selectorPolicy.detach`) that shows up once endpoint
regeneration runs at full speed under heavy identity churn. `policyCache.delete()`
dereferences `cip.getPolicy()` unconditionally, but a cache entry created by
`lookupOrCreate()` has a nil policy until `updateSelectorPolicy()` finishes resolving it,
and that resolution does not hold the cache lock — so an endpoint whose identity changes
mid-regeneration can have its old identity removed while its policy is still being
resolved. The patch adds the nil check, matching `pkg/policy/distillery.go` in 1.18.

**Remove this patch when upgrading to Cilium 1.18 or newer**, where the same nil check is
already present (it was never backported to 1.17.x, up to and including v1.17.18).

## 021-ebpf-api-compat.patch

Two fixes Cilium v1.17.17 needs to work with `github.com/cilium/ebpf` v0.22.0,
which `000-go-mod.patch` bumps from v0.17.1 to fix CVE-2026-10722 (fixed only in
ebpf >= v0.22.0).

1. Compile fix (`pkg/bpf/collection.go`): ebpf v0.21.0 removed
   `(*ebpf.VariableSpec).MapName()` in favour of the exported
   `VariableSpec.SectionName` (same value). `applyConstants()` called it to assert
   config variables live in `.rodata.config`; both calls are replaced.

2. Runtime fix (`pkg/datapath/linux/probes/probes.go`), backported from upstream
   `36f2b4a12`: with `net.core.bpf_jit_harden=2` the kernel blinds constants even
   for privileged loaders and then hides xlated instructions from
   `BPF_OBJ_GET_INFO_BY_FD`, unless the caller may read kernel pointers
   (`kallsyms_show_value()`: `kptr_restrict=0` *and* `perf_event_paranoid<=1`, or
   `CAP_SYSLOG` — the agent has neither). ebpf v0.20.0 turned that previously
   silent empty response into `ebpf.ErrRestrictedKernel`, so `HaveDeadCodeElim()`
   started failing and `CheckRequirements()` aborted agent startup with
   `requirements failed: Require support for dead code elimination (Linux 5.1 or
   newer)`. The probe now tolerates the error, as it effectively did before the
   bump. Deckhouse sets `bpf_jit_harden=2` in the CSE edition
   (`candi/bashible/common-steps/all/041_configure_sysctl_tuner.sh.tpl`) and some
   hardened distros set it themselves, hence only part of the clusters broke
   (seen with `kptr_restrict` both 2 and 0). Upstream's two other call sites
   (`HaveBPFJIT`, `verifyUnusedMaps`) do not exist in v1.17.17, and nothing else
   there reads xlated/JIT/BTF info from `ProgramInfo`.

Upstream chain: ebpf adds `ErrRestrictedKernel`
(<https://github.com/cilium/ebpf/pull/1858>, v0.20.0) -> ebpf queries xlated
instructions separately so the rest of the info survives
(<https://github.com/cilium/ebpf/commit/5ac1d5a9f065adef5726b3969da8ef3a1626c603>,
v0.21.0) -> Cilium reverts its own bump over these probe failures on GKE COS
(<https://github.com/cilium/cilium/pull/42327>) -> Cilium forward-fixes the probes
and re-lands the bump (<https://github.com/cilium/cilium/pull/42361>,
<https://github.com/cilium/cilium/commit/36f2b4a127f07985a79b984cd603de3cbd9c1d0f>).

**Remove this patch when upgrading to a Cilium version that already targets
ebpf >= v0.21.0**, where `applyConstants()` no longer uses the removed method
and the probes already tolerate `ErrRestrictedKernel`.
