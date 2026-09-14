# Patches

One patch per commit on the `d8/v1.20.1` branch of the cilium fork
(`git@github.com:dna787/cilium.git`). The rationale, design notes and behaviour
details live in the commit message and in the code comments the patch adds, not
here. Regenerate with:

    git format-patch --no-signature --no-numbered v1.20.1..d8/v1.20.1 -o <dir>

## 000-go-mod.patch

Bumps a set of Go module dependencies in `go.mod`/`go.sum` above the versions
shipped by upstream Cilium v1.20.1 to remediate known CVEs.

Regenerate by cloning `cilium v1.20.1`, applying this patch, bumping the
required module(s), then running `go mod tidy && go mod vendor && go mod verify`
and diffing `go.mod`/`go.sum`.

## 001-request-ip.patch

Add the opportunity to request specific IP-address using annotation:

    cni.cilium.io/ipAddress: 10.10.10.10

Needed by DVP, where the address is part of a VM's identity: live migration runs
two pods for one VM on two nodes, both holding that address, so a requested
address is accepted even when it falls outside the local node's podCIDR.

Upstream <https://github.com/cilium/cilium/pull/24098>
Test `~/src/kind/d8-1.20-tests/001-request-ip/`

## 002-stable-mac.patch

Predefined MAC addresses for virtualization workloads:

    endpoint-interface-mac: 0a:d8:00:00:00:11
    endpoint-interface-host-mac: 0a:d8:00:00:00:22

Needed by DVP: a VM keeps its MAC across a live migration, and the host side
address is the one the pod ARPs for its gateway, so it has to be identical on
every node. Upstream's per-pod `cni.cilium.io/mac-address` still overrides the
container side, and covers only that side.

Upstream <https://github.com/cilium/cilium/pull/24100>
Test `~/src/kind/d8-1.20-tests/002-stable-mac/`

## 003-mtu.patch

Give endpoint devices `RouteMTU` instead of `DeviceMTU`.

A virtual machine inside a pod takes its MTU from the link, not from the pod's
default route, so a device left at `DeviceMTU` makes the guest emit frames the
overlay cannot carry. Covers creation time (CNI plugin and health endpoint) and
the endpoint MTU updater, which would otherwise reset the devices.

Upstream issue <https://github.com/cilium/cilium/issues/23711>
Test `~/src/kind/d8-1.20-tests/003-mtu/`

## 005-ebpf-dhcp-server.patch

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

Test `~/src/kind/d8-1.20-tests/005-dhcp/`

## 008-hide-error-of-incompatibility-of-egw-with-ces.patch

Upstream makes the agent fatal when the egress gateway and CiliumEndpointSlice
are both enabled (commit `9768f15c9d`, <https://github.com/cilium/cilium/issues/24833>).
Deckhouse runs both, so the agent would not start at all.

Measured on 1.20: policies still resolve for local and remote pods, because the
manager reads `CiliumEndpoint` objects directly and CES does not stop those being
created. That would not hold with `disable-endpoint-crd`, which the module does
not set.

Remove once CES is stable, <https://github.com/cilium/cilium/issues/31904>.

Test `~/src/kind/d8-1.20-tests/008-egw-with-ces/`

## 013-ignore-egress-gateway-inactual-warning.patch

Tolerate a `CiliumEgressGatewayPolicy` whose egress IP is not assigned to any
interface on the gateway node. Deckhouse attaches such addresses out of band, so
upstream's derive failure would log an error on every reconcile.

Note this changes forwarding, not only logging: the policy is programmed with the
configured egress IP, where unpatched the entry stays `0.0.0.0`.

Test `~/src/kind/d8-1.20-tests/013-egw-unassigned-ip/`

## 018-fix-svacer.patch

Nil guard in `ICMPField.UnmarshalJSON`: a missing or null `type` panics in
`IntOrString.IntValue()`, because `String()` tolerates a nil receiver and
returns `"<nil>"` so the preceding check does not short circuit.

Not reachable from a running cluster in 1.20 -- the CRD marks `type` as
required, and `cilium-dbg policy import` no longer exists -- but it clears the
Svace `DEREF_OF_NULL` finding in this build (see `SvaceBuildOptions` in
`werf.inc.yaml`).

Test `~/src/kind/d8-1.20-tests/018-icmp-nil-type/` (unit test; its negative
control runs the same test against a pristine v1.20.1 worktree, where it panics)

## 009-wireguard-port.patch

Move the WireGuard listen port from upstream's `51871` to `4287`, inside the
range Deckhouse reserves for platform components.

One constant in 1.20: the datapath reads it through `CONFIG(wg_port)`, and the
iptables rules and the device derive from it too, so the 1.17 patch's
`bpf/node_config.h` hunk is gone. The `.github/actions/bpftrace` hunk is dropped
as CI-only.

Test `~/src/kind/d8-1.20-tests/009-wireguard-port/`

## 015-cleanup-conntrack-endpoints.patch

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

Test `~/src/kind/d8-1.20-tests/015-conntrack-cleanup/`. A live migration is
imitated on the dev cluster by rewriting the datapath's ipcache entry for the pod
so it carries a tunnel endpoint, which is what the address looks like once
another node owns it, and holding that across the teardown -- the agent
reconciles it back within about a second. Patched, the local client's outbound
entries survive and the inbound ones go; unpatched, all of them go. A table test
in `pkg/maps/ctmap` pins down the per-direction behaviour.

## 016-add-import-export-conntrack-http-endpoints.patch

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

Test `~/src/kind/d8-1.20-tests/016-conntrack-api/`. It installs the way the
module does -- `bpf-lb-sock-hostns-only` -- because only then is pod traffic load
balanced on the tc hooks, which is what produces the service conntrack entries
the RevNAT translation needs.

## Dropped

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

Test kept as the evidence record: `~/src/kind/d8-1.20-tests/011-hostport-lb-algo/`
(it passes on the unpatched image, which is the point).

### 019-ipcache-no-deadlock-on-label-injection.patch

`IPCache.UpdatePolicyMaps()` was removed upstream (`cilium#39970`), so the
deadlock the patch worked around cannot occur and the patch cannot apply.

### 020-policy-nil-safe-selector-policy-detach.patch

`pkg/policy/distillery.go` is gone and the nil check is present upstream.
