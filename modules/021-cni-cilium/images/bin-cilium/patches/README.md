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
