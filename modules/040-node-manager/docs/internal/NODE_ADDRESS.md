---
title: "Node address on Deckhouse Engine"
description: How an immutable node chooses the address it joins the cluster with, who decides at each step, and how the choice is pinned and reported.
---

The address a node presents to the cluster is a property of one of its physical interfaces. Which
interface, a layered rule decides once at boot; the result is pinned on the node and the reason is
reported in the NodeConfig status. This document is the contract between node-controller, dhctl and
nodelet. **If something is not in this document, it is not in the contract.**

## Principle

Every layer of the rule uses whoever knows best, and none of them guesses. The operator may name the
interface. The cluster knows its own networks and hands them to the node. The node's routing table says
which interface it reaches the control plane through. When none of that answers and the node has one
physical NIC, that NIC is the answer; with several, the first one the document describes. Only a node
whose document describes no interface at all refuses, and says why. The default route means nothing.

There is no address literal in the document. An address cannot be known ahead of time for a DHCP
interface, and nothing could check it against the interface list. The interface mark replaces it.

## The rule

Each layer either answers or passes to the next. Layers go from the most precise knowledge to the
weakest.

1. **Candidates**: physical interfaces (`/sys/class/net/<name>/device` exists) with a global IPv4
   address. Virtual ones (CNI, tunnels, bridges) are never candidates.
2. **Operator or dhctl**: an interface marked `cluster: true` answers. The explicit choice beats
   everything, the pin included.
3. **Cluster**: if `spec.internalNetworkCIDRs` is set, the first CIDR in list order that has an
   address on the machine answers, with that address. The list order decides, as it does in bashible,
   so both node types in one cluster pick alike. Two addresses inside one CIDR on different NICs: the
   lower ifindex wins. No address in any listed CIDR: refusal, `NoneInCIDR`, and the rule stops.
4. **Topology**: an unambiguous route to `apiServerEndpoints` answers with the interface it leaves
   through. Endpoints are URLs; a host that is not an IPv4 literal is skipped. The route is found by
   longest-prefix match over `/proc/net/route` (the main table only) and is unambiguous when the
   winning entry is more specific than default, or is the only default route in the table. Two equal
   defaults answer nothing. All endpoints that gave an unambiguous route must agree on the interface.
5. **Machine**: a single candidate answers, `SingleInterface`.
6. **Document**: the first candidate described in `spec.network.interfaces`, in document order,
   answers, `FirstListed`. This is the last resort: the document's order is a statement by the
   operator or the cluster, not by the routing table. In a cloud, the cluster describes `eth0` first.
7. **Otherwise**: refusal, `Ambiguous`, with the candidates and the route table in the condition.
   Only a node with several physical NICs and none of them described reaches this.

The address follows the interface: the current lease on a DHCP interface, the first address inside
the deciding CIDR on a static one, otherwise `addresses[0]`. One result feeds every consumer:
kubelet's `--node-ip`, `$MY_IP` in `apiServerEndpoints`, the control-plane manifests (etcd peer and
client URLs, `--advertise-address`, certificate SANs) and the kubeconfig handed to the installer.
Cloud-controller-manager is not a consumer: it writes `Node.status.addresses` from the cloud API on
its own.

## What is in the document

```yaml
spec:
  apiServerEndpoints: [...]
  internalNetworkCIDRs: [10.12.0.0/16]   # cluster-owned, next to apiServerEndpoints
  network:                               # machine-owned, as before
    interfaces:
    - name: eth0
      dhcp: true
      cluster: true                      # the mark, optional
  statusToken: <hex>                     # machine-owned, bearer for GET /status on :50000
status:
  network: {clusterInterface: eth0, address: 10.12.1.92}
  conditions:
  - type: AddressResolved
    reason: Marked        # Marked | ClusterCIDR | APIServerRoute | SingleInterface | FirstListed
                          # | NoneInCIDR | Ambiguous | AddressDrift
```

- `spec.internalNetworkCIDRs`: IPv4 CIDRs only, the same pattern `StaticClusterConfiguration`
  enforces, at most 64. It lives next to `apiServerEndpoints` and not inside `spec.network`,
  because `spec.network` belongs to the machine: the webhook refuses cluster writes into it and the
  carry-over keeps it as the node published it.
- `spec.network.interfaces[].cluster`: at most one `true` in the list.
- `spec.statusToken`: minted by whoever writes the document (dhctl, or the NodeConfigTemplate render
  for a hand-installed machine) and carried over by the cluster untouched. It is deliberately not
  marked sensitive: the apiserver would answer `<omitted>` and the carry-over would write that back.
- `spec.kubelet.nodeIP` no longer exists. The operator's document describes the machine only:
  `network` and `storage`.

## Who does what

**dhctl, static cluster.** Before the push it reads `/inventory.json` from the machine and walks the
same ladder over the inventory and the operator's document: the mark, the cluster CIDRs, the only
NIC, the first NIC the operator described, and, when the operator described none, the NIC the push
address lives on, printed to the log. The choice is written into the document as `cluster: true`; an
interface entry is synthesized from the inventory when the operator wrote none. Several NICs, no
CIDR, no mark: dhctl refuses before the machine is touched, naming its interfaces, the way bashible
refuses. The address dhctl then waits on is computed by the same rule. Converge takes the same path.

**dhctl, cloud, first master.** The document carries the provider's CIDR where the provider declares
one, and the rendered `eth0` description, so the node answers by the cluster layer or by
`FirstListed`. The master's address from the terraform outputs does not reach the document: the
payload is rendered before `terraform apply`, and the address is that apply's result. Making the
payload lazy over the outputs is a separate change in `pkg/infrastructure`.

**node-controller.** Renders `internalNetworkCIDRs` into every NodeConfig and NodeConfigTemplate from
`StaticClusterConfiguration` (`d8-static-cluster-configuration`) or from the provider configuration
(`d8-provider-cluster-configuration`), read unstructured through one table of provider kind to field.
A cluster has one of the two secrets; a missing one is an empty list, not a failed pass. It never
sets the mark: in a cloud that would turn the `eth0` guess into an order. It carries the node's
`network` and `statusToken` over untouched.

**nodelet.** Walks the ladder once, in `config.LoadNode`, before the API proxy and every controller,
and threads the result to them as an argument. Pins the result on disk. Publishes the interface,
the address and the reason in the status. Serves `GET /status` on `:50000` with the bearer from the
document, for both the API and the file source, so a refusal is visible before the node has joined
anything; on the file source `PUT /config` answers 503. A refusal does not fail the load: it fails
the kubelet and control-plane passes and is published as `AddressResolved=False`, so the maintenance
port stays up to read it.

## Pinning

The first successful choice is written to `/var/lib/nodelet/node-address`, next to the node's other
machine facts. `/var` survives reboots and A/B updates and is lost only by a reinstall with `wipe`.

```yaml
# /var/lib/nodelet/node-address
interface: eth0
address: 10.12.1.92
reason: Marked          # the layer that answered at the first choice
chosenAt: "2026-09-04T10:21:07Z"
```

Written once, atomically (temporary file and rename), mode 0644. Rewritten only when a `cluster: true`
mark points at a different interface. On reading, only `address` is compared; `interface` and
`reason` are for the message to the operator, since kernel interface names shift when a NIC is added.
A broken or unreadable file is a refusal with a condition, not "no pin".

After the pin the ladder is not rerun; only the presence of the pinned address is checked. A changed
DHCP lease or route table leaves the node on the pinned address and raises `AddressDrift` with both
addresses. A pinned address that is gone from the machine stops the node with the same condition:
kubelet cannot start with an absent `--node-ip`. An explicit mark from the operator overrides the pin;
for a master that is a deliberate address change and means a re-join.

Today nothing pins the address, on Engine or in bashible: it is recomputed on every run, and drift
breaks etcd silently.

## By scenario

| Scenario | Who decides | Reason in status |
|---|---|---|
| Static, any node through dhctl | dhctl over the inventory, mark in the document | `Marked` |
| Static, day-2 worker by hand | the operator by the mark, else the cluster CIDRs, else the route to the apiserver | `Marked`, `ClusterCIDR` or `APIServerRoute` |
| Cloud, first master | the provider's CIDR, else the only NIC, else the described `eth0` | `ClusterCIDR`, `SingleInterface` or `FirstListed` |
| Cloud, every other node | the provider's CIDR, else the route to the apiserver, else the only NIC, else the described `eth0` | `ClusterCIDR`, `APIServerRoute`, `SingleInterface` or `FirstListed` |
| Nothing to choose by | refusal, candidates and routes in the status | `NoneInCIDR`, or `Ambiguous` for a document with no interfaces |

Refusal remains in two cases: the node has no address in any listed cluster network (bashible
refuses there too), or the document describes no interface and the machine has several physical
ones. Everywhere else there is a choice, and its reason is readable in the status.

## What this gives

- One rule chooses the address on static and cloud nodes alike.
- A cluster network on DHCP works: the address follows the interface instead of being written ahead.
- A wrong choice is loud and visible before the node joins, not through a broken etcd.
- Address drift does not silently replay the choice.
- "Why this address" is read from the NodeConfig status, not from the code.

## Not in this contract

- The first cloud master's address from the terraform outputs (see above).
- A cluster CIDR for providers that declare none (DVP, zVirt, OpenStack `simple*`, Dynamix
  `Standard`): there the cluster layer is empty and the route or the described NIC answers.
- Whether the document owns the network entirely: the stock `20-dhcp.network` in the image still
  brings up every NIC the document does not name. With physical-only candidates and the pin this no
  longer breaks the choice, but the node still gets addresses and routes nobody asked for.
- IPv6.
