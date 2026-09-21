# Patches

One patch per commit on the `d8/v1.20.1` branch of the cilium fork
(`git@github.com:dna787/cilium.git`). The rationale, design notes and behaviour
details live in the commit message and in the code comments the patch adds, not
here. Regenerate with:

    git format-patch --no-signature --no-numbered --binary v1.20.1..d8/v1.20.1 -o <dir>

`--binary` matters: `013-add-least-conn-lb-algorithm` adds two BPF maps, which
regenerates `pkg/datapath/maps/mapkv.btf`. Without it that hunk exports as
"Binary files differ" and the patch will not apply.

The numbers are the order the commits sit in on the branch, because the build
applies `patches/*.patch` and the shell expands that in filename order -- a patch
whose diff was taken on top of another must be applied after it. They are
therefore **not** the numbers the same patches had in the 1.17 set; where a 1.17
number is meant, it says so.

## 000-go-mod.patch

Bumps a set of Go module dependencies in `go.mod`/`go.sum` above the versions
shipped by upstream Cilium v1.20.1 to remediate known CVEs.

Regenerate by cloning `cilium v1.20.1`, applying this patch, bumping the
required module(s), then running `go mod tidy && go mod vendor && go mod verify`
and diffing `go.mod`/`go.sum`.

## 001-request-ip.patch

Add the opportunity to request a specific IP address using an annotation. Two
carry the request, the first winning:

    network.deckhouse.io/networks-spec: [{"type":"Main","ipAddress":"10.10.10.10"}]
    cni.cilium.io/ipAddress: 10.10.10.10

`networks-spec` describes every network attached to the pod and its `Main` entry
is the one this CNI honours; `cni.cilium.io/ipAddress` is the older
single-valued form and the fallback, including when `networks-spec` names no
primary network. One that cannot be parsed fails the allocation rather than
falling through.

Needed by DVP, where the address is part of a VM's identity: live migration runs
two pods for one VM on two nodes, both holding that address, so a requested
address is accepted even when it falls outside the local node's podCIDR.

Upstream <https://github.com/cilium/cilium/pull/24098>
Test `~/src/kind/d8-1.20-tests/request-ip/`

## 002-stable-mac.patch

Predefined MAC addresses for virtualization workloads:

    endpoint-interface-mac: 0a:d8:00:00:00:11
    endpoint-interface-host-mac: 0a:d8:00:00:00:22

Needed by DVP: a VM keeps its MAC across a live migration, and the host side
address is the one the pod ARPs for its gateway, so it has to be identical on
every node. Upstream's per-pod `cni.cilium.io/mac-address` still overrides the
container side, and covers only that side.

Upstream <https://github.com/cilium/cilium/pull/24100>
Test `~/src/kind/d8-1.20-tests/stable-mac/`

## 003-mtu.patch

Give endpoint devices `RouteMTU` instead of `DeviceMTU`.

A virtual machine inside a pod takes its MTU from the link, not from the pod's
default route, so a device left at `DeviceMTU` makes the guest emit frames the
overlay cannot carry. Covers creation time (CNI plugin and health endpoint) and
the endpoint MTU updater, which would otherwise reset the devices.

Upstream issue <https://github.com/cilium/cilium/issues/23711>
Test `~/src/kind/d8-1.20-tests/mtu/`

## 004-ebpf-dhcp-server.patch

A DHCP server for pods, implemented in the datapath (`bpf/lib/dhcp.h`, hooked
into `cil_from_container`). A VM inside a pod boots by DHCP, and there is no
DHCP server on a cilium network, so the agent answers out of what it already
knows: the endpoint's own address, the node gateway, and the configured DNS,
search domain and MTU.

    dhcpd-enabled: "true"
    dhcpd-cluster-dns: "10.96.0.10"
    dhcpd-cluster-domain: "cluster.local"

Option 26 hands out `RouteMTU`, the same value `003-mtu.patch` puts on the
devices. Only the `veth` datapath is covered.

Test `~/src/kind/d8-1.20-tests/dhcp/`

## 005-hide-error-of-incompatibility-of-egw-with-ces.patch

Upstream makes the agent fatal when the egress gateway and CiliumEndpointSlice
are both enabled (commit `9768f15c9d`, <https://github.com/cilium/cilium/issues/24833>).
Deckhouse runs both, so the agent would not start at all.

Measured on 1.20: policies still resolve for local and remote pods, because the
manager reads `CiliumEndpoint` objects directly and CES does not stop those being
created. That would not hold with `disable-endpoint-crd`, which the module does
not set.

Remove once CES is stable, <https://github.com/cilium/cilium/issues/31904>.

Test `~/src/kind/d8-1.20-tests/egw-with-ces/`

## 006-ignore-egress-gateway-inactual-warning.patch

Tolerate a `CiliumEgressGatewayPolicy` whose egress IP is not assigned to any
interface on the gateway node. Deckhouse attaches such addresses out of band, so
upstream's derive failure would log an error on every reconcile.

Note this changes forwarding, not only logging: the policy is programmed with the
configured egress IP, where unpatched the entry stays `0.0.0.0`.

Test `~/src/kind/d8-1.20-tests/egw-unassigned-ip/`

## 007-fix-svacer.patch

Nil guard in `ICMPField.UnmarshalJSON`: a missing or null `type` panics in
`IntOrString.IntValue()`, because `String()` tolerates a nil receiver and
returns `"<nil>"` so the preceding check does not short circuit.

Not reachable from a running cluster in 1.20 -- the CRD marks `type` as
required, and `cilium-dbg policy import` no longer exists -- but it clears the
Svace `DEREF_OF_NULL` finding in this build (see `SvaceBuildOptions` in
`werf.inc.yaml`).

Test `~/src/kind/d8-1.20-tests/icmp-nil-type/` (unit test; its negative
control runs the same test against a pristine v1.20.1 worktree, where it panics)

## 008-wireguard-port.patch

Move the WireGuard listen port from upstream's `51871` to `4287`, inside the
range Deckhouse reserves for platform components.

One constant in 1.20: the datapath reads it through `CONFIG(wg_port)`, and the
iptables rules and the device derive from it too, so the 1.17 patch's
`bpf/node_config.h` hunk is gone. The `.github/actions/bpftrace` hunk is dropped
as CI-only.

Test `~/src/kind/d8-1.20-tests/wireguard-port/`

## 009-cleanup-conntrack-endpoints.patch

Keep local clients' conntrack entries when an address migrates to another node.

Tearing down an endpoint normally scrubs every conntrack entry whose source or
destination is its address. For a VM that has live migrated away that resets the
connections migration exists to preserve: clients still running on this node
hold flows towards the address, now forwarded over the tunnel.
`GCFilter.MigrationSafeCleanup` removes only the flows the departing address
owned and keeps outbound entries where it is the destination.

The endpoint detects the case from the datapath's own ipcache map: a tunnel
endpoint is set only for an address owned by another node. The lookup happens
once per teardown, not inside the GC filter, which runs for every conntrack
entry.

Test `~/src/kind/d8-1.20-tests/conntrack-cleanup/`. A live migration is
imitated on the dev cluster by rewriting the datapath's ipcache entry for the pod
so it carries a tunnel endpoint, which is what the address looks like once
another node owns it, and holding that across the teardown -- the agent
reconciles it back within about a second. Patched, the local client's outbound
entries survive and the inbound ones go; unpatched, all of them go. A table test
in `pkg/maps/ctmap` pins down the per-direction behaviour.

## 010-add-import-export-conntrack-http-endpoints.patch

`GET /conntrack/export` streams one IPv4 endpoint's conntrack entries as a binary
stream; `POST /conntrack/import` ingests them on another node. Used to carry a
VM's established connections across a live migration. See
`modules/021-cni-cilium/docs/internal/DVP_INTEGRATION.md`.

Each imported entry's RevNAT is translated from the source node's service id to
the local one, resolved through the service map; a reference that cannot be
resolved is dropped rather than written with the wrong id.

The wire format is unchanged from 1.17 -- 68 bytes per entry -- so the
`Cilium-Conntrack-Export-Version` header stays at `"1"` and existing consumers
keep working. `daemon/cmd/status.go` and `api_handlers.go` are gone in 1.20, so
the handlers live in `pkg/maps` and are provided by its cell. The generated API
files come from `make generate-api`, never hand edits.

Test `~/src/kind/d8-1.20-tests/conntrack-api/`. It installs the way the
module does -- `bpf-lb-sock-hostns-only` -- because only then is pod traffic load
balanced on the tc hooks, which is what produces the service conntrack entries
the RevNAT translation needs.

## 011-kernel-verifier-stat.patch

Export `cilium_bpf_progs_complexity_max_verified_insts`: the highest instruction
count the verifier walked for any loaded `cil_`/`tail_` program. The verifier
refuses a program past 1,000,000 instructions, and without the gauge that ceiling
is invisible until a datapath change makes the agent fail to load on some nodes.
`ebpf.LogLevelStats` also puts the verifier's per-program statistics in the agent
debug log.

Much smaller than on 1.17: that version also had to replace a
`bpftool -j prog show` subprocess with a walk over the ebpf API, which upstream
has since done itself (`bpfVisitor`). Only the metric is left.

Test `~/src/kind/d8-1.20-tests/verifier-stat/`

## 012-bpf-lb-generate-icmp-reply.patch

Make a LoadBalancer service IP answer `ping`. A VIP is on no interface anywhere,
so nothing in the stack replies to an echo request for it and monitoring reads
the silence as the service being down.

    enable-loadbalancer-icmp-reply: "true"

The control plane marks each LoadBalancer frontend with a (VIP, port 0, proto
ICMP) service entry, refcounted over the service's ports; the datapath stops
dropping ICMP in `lb4_extract_tuple()` and, on a match, rewrites the request
into a reply and sends it back out, rate limited per ingress interface.

Smaller than on 1.17 because 1.20 grew the same refcount-per-VIP machinery
upstream for its wildcard entries, so the entry rides the StateDB reconciler
instead of `pkg/service`, and the restore pass is no longer needed. The ICMP
lookup asks for an exact match (`lb4_lookup_service(&key, true)`) so it can never
hit upstream's wildcard entry, whose meaning is "drop", and an echo we do not
answer takes the same path an unpatched build takes.

The rate limiter reuses the existing `icmpv6` member of `struct ratelimit_key`
rather than adding one: the buckets are separated by `usage`, and a new member
would change the generated binary `pkg/datapath/maps/mapkv.btf`, which the
Deckhouse build cannot regenerate and which would break this patch on the next
upstream tag.

Deckhouse sets `default-lb-service-ipam: none`, which is exactly when upstream
writes no wildcard entry for a classless LoadBalancer service -- so this patch is
still required. Needs `kubeProxyReplacement`.

Test `~/src/kind/d8-1.20-tests/lb-icmp-reply/`

## 013-add-least-conn-lb-algorithm.patch

A `least-conn` load balancing algorithm, selected per service:

    service.cilium.io/lb-algorithm: least-conn

`cilium_lb4_leastconn_backend` counts open connections per backend and
`cilium_lb4_leastconn_service` remembers which backend to hand out next. Picking
the minimum is too expensive per packet, so a BPF timer re-scans off the packet
path and the datapath reads the remembered choice; the conntrack GC recounts the
live entries on its first pass, because the pinned counters outlive an agent
restart while the agent's view of them does not.

Much smaller than on 1.17 because upstream added a hook for exactly this
(`004dd6d4e4`): `lb4_select_backend_id_custom()` plus
`RegisterSVCLoadBalancingAlgorithm`, so registering the algorithm is four lines
and the datapath hookup is one include. The annotation, and the `bpf_lxc`
ClusterIP override the 1.17 patch carried, are both upstream now.

The maps are declared outside the feature `#ifdef`, as upstream does for
`cilium_lb_act`: `tools/dpgen` builds the map registry from objects compiled with
the standard define set, so a map behind a feature guard would never be
registered. That registration is what regenerates `mapkv.btf`.

The scan runs at once when it has been idle and is otherwise rate limited to
`LEAST_CONN_TIMEOUT`, set to 3ms: a burst arriving inside one interval all lands
on the backend selected when it started, so the interval sets how coarsely load
is spread. Measured over three 200-connection bursts across three backends, as
max:min skew of what each backend received -- 10ms gives 2.15x/1.72x/2.15x, 3ms
gives 1.20x/1.05x/1.03x, and 1ms only reaches 1.02x. The table is in
`bpf/lib/least_conn.h`. Not measured: the scan cost on a service with many
backends, which is what to check before shortening it further.

Reachable only through the annotation -- `bpf-lb-algorithm` is validated against
`random`/`maglev`, so least-conn cannot be a node-wide default. Needs
`kubeProxyReplacement` and `bpf-lb-sock-hostns-only`.

Test `~/src/kind/d8-1.20-tests/least-conn/`

## 014-add-pod-prioroty-management.patch

One shared IPv4, two pods, a single owner -- the window a DVP live migration
passes through.

    network.deckhouse.io/pod-common-ip-priority: <number>      (0 is the highest)

The operator publishes the address on the winning CiliumEndpoint only, and every
other node withholds its `cilium_lxc` entry until the address comes back. Only
the IPv4 is withheld: the two pods of one VM have distinct IPv6 addresses, which
are never in conflict. Unlike the 1.17 version this needs no change to the lxcmap
value struct, so the alignchecker and `mapkv.btf` are untouched.

Folds in `007-fix-restoring-cep-for-dead-local-endpoint` (a 1.17 number; there
is no separate patch for it here).

Test `~/src/kind/d8-1.20-tests/pod-priority/`: fourteen single-case scripts,
one transition each, driven by `run-cases.sh`, which installs once and stops at
the first failure. Patched: 70 case runs, no failures. On the same stack one
commit earlier it stops at the second case, with both nodes holding an entry and
both CiliumEndpoints publishing the address. The 007 half has no cluster test --
the leftover CiliumEndpoint it catches cannot be staged from outside, because
CiliumEndpoints are garbage collected with their pod -- and is carried on the
strength of having been seen in production on 1.17.

## Dropped

Numbered as they were in the 1.17 set.

Patches from the 1.17 stack that are not carried on 1.20, with the evidence:

### 011-bpf-lb-use-random-lb-algo-for-hostport-serives-fixed.patch

Both halves are obsolete.

The `default:` arm of `lb{4,6}_select_backend_id()` used to `return 0` for an
algorithm value it did not recognise, leaving the service with no backend.
Upstream now falls back to `lb_default_algorithm()` there, which is exactly what
the patch did.

Forcing random selection for HostPort was needed because HostPort
pseudo-services had no Maglev table. In 1.20 they do: `SVCTypeHostPort` is in the
list of service types `useMaglev()` provisions a LUT for
(`pkg/loadbalancer/reconciler/bpf_reconciler.go`). Verified on the kind cluster against
**unpatched** v1.20.1 across the whole matrix -- `bpf-lb-algorithm` random and
maglev, each with `bpf-lb-algorithm-annotation` on and off, 24 checks, all
passing: a hostPort answers from its own node and from another node, and a
NodePort service keeps working. The annotation state matters because it switches
`LB_SELECTION_PER_SERVICE`, which changes how `lb4_algorithm()` derives the
algorithm; Deckhouse enables it via `extraLoadBalancerAlgorithmsEnabled`.

Test kept as the evidence record: `~/src/kind/d8-1.20-tests/hostport-lb-algo/`
(it passes on the unpatched image, which is the point).

### 019-ipcache-no-deadlock-on-label-injection.patch

The deadlock was ipcache holding `ipc.mutex` while waiting for every endpoint's
policy map update. Both halves of that are gone from `pkg/ipcache`:
`IPCache.UpdatePolicyMaps()` and `ipc.DatapathHandler` were removed by
`47ace2de6e`, and the wait now lives in another component entirely,
`pkg/policy/cell/identity_updater.go` calling `epmanager.UpdatePolicyMaps()`,
which never runs under that lock. Note 1.20 still has a function of that name at
`pkg/endpointmanager/manager.go` -- different owner, different signature.

Upstream also adopted the patch's ordering on its own: `doInjectLabels` now
deletes no-longer-referenced prefixes before releasing identities.

### 020-policy-nil-safe-selector-policy-detach.patch

`policyCache`, `cachedSelectorPolicy` and `pkg/policy/distillery.go` are all
gone, replaced by the StateDB-based `pkg/policy/compute`. The same defect class
-- releasing a policy while another goroutine may still be resolving it -- is
handled there: the row is deleted in a write transaction, and only after the
commit are the selectors released, behind a nil guard
(`pkg/policy/compute/compute.go`, `if obj.NewPolicy != nil`).
