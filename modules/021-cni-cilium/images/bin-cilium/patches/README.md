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
