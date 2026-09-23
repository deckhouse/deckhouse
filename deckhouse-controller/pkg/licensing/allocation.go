/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package licensing

import "sort"

// The three consumption metrics of specification 7.2. Aggregation walks the keys
// of resource_limits generically and knows none of these names; Allocate is the
// single place in the code base that knows what they mean.
const (
	MetricServers = "servers"
	MetricVCPU    = "vCPU"
	MetricCores   = "cores"
)

// Billing groups a node can land in: one per metric of the key, plus the two
// groups no metric pays for. Every licensable node is attributed to exactly one
// of them, so the Console can name the licence that covers a given node.
const (
	BillingFree       = "Free"
	BillingServer     = "Server"
	BillingVCPU       = "VCPU"
	BillingCores      = "Cores"
	BillingUnlicensed = "Unlicensed"
)

// Node is one licensable node as the allocation sees it: a name and a whole
// number of vCPU. Nothing else about a node can affect the allocation.
type Node struct {
	Name string
	VCPU int64
}

// Limits is the aggregated quota of the active records.
type Limits struct {
	// Values maps a metric to its limit. A nil value means the metric is
	// granted without a limit; an absent key means no active record named it.
	Values map[string]*int64

	// Speaking reports whether any active record carries resource_limits at
	// all. It is what tells "there is no key" from "the key does not name this
	// metric": a record that carries resource_limits and omits a metric grants
	// that metric without a limit (ADR 14.3, vectors P16 and S10), while a
	// cluster with no speaking record at all is granted nothing (vector S13).
	Speaking bool
}

// Of returns the limit of one metric, resolving an absent key the way ADR 14.3
// requires. The second result is false when the metric is unlimited.
func (l Limits) Of(metric string) (int64, bool) {
	value, named := l.Values[metric]
	switch {
	case named && value == nil:
		return 0, false
	case named:
		return *value, true
	case l.Speaking:
		return 0, false
	default:
		return 0, true
	}
}

// MetricAllocation is what one metric of the key ended up covering.
type MetricAllocation struct {
	// Nodes are the node names attributed to this metric, in allocation order.
	Nodes []string
	// Limit is the granted quota; nil means the metric is unlimited.
	Limit *int64
	// Used is what those nodes cost the metric: one per node for servers, the
	// node vCPU for vCPU, ceil(vCPU/2) for cores.
	Used int64
}

// fits reports whether one more cost still stays inside the quota.
func (m MetricAllocation) fits(cost int64) bool {
	return m.Limit == nil || m.Used+cost <= *m.Limit
}

func (m *MetricAllocation) take(name string, cost int64) {
	m.Nodes = append(m.Nodes, name)
	m.Used += cost
}

// Allocation is the assignment of every licensable node to a billing group,
// together with the totals the status and the Console show next to it.
type Allocation struct {
	Servers MetricAllocation
	VCPU    MetricAllocation
	Cores   MetricAllocation

	// Unlicensed are the node names that fit into no metric, in allocation order.
	Unlicensed []string
	// UnlicensedVCPU is the vCPU of those nodes.
	UnlicensedVCPU int64
}

// WithinLimits reports whether every licensable node is covered.
func (a Allocation) WithinLimits() bool { return len(a.Unlicensed) == 0 }

// Allocate lays the licensable nodes out over the metrics of the key
// (specification 7.3). It is a pure function of the node set, the effective
// limits and the previous allocation.
//
// The capacity is still the single pool vCPU + 2*cores; what the walk adds is
// attribution: every covered node names the metric that pays for it, because
// the Console has to say which licence covers which node. The order is fixed:
// servers go to the largest nodes, then cores, and vCPU comes last because it is
// the finest-grained metric and fills whatever is left. A node is never split
// between two metrics, so a node that does not fit into cores takes vCPU.
//
// Cores before vCPU also keeps a node where it is when a reissue raises the vCPU
// quota: a node covered by cores stays on cores.
//
// wasServer is the previous allocation, read off status.nodes[]; it only breaks
// ties between equally sized nodes.
func Allocate(nodes []Node, limits Limits, wasServer map[string]bool) Allocation {
	ordered := SortNodes(nodes, wasServer)

	out := Allocation{
		// An unlimited quota covers everything there is; the published limit
		// keeps the quota as granted, so the Console can say "12 of 12".
		Servers: MetricAllocation{Limit: grantedLimit(limits, MetricServers)},
		VCPU:    MetricAllocation{Limit: grantedLimit(limits, MetricVCPU)},
		Cores:   MetricAllocation{Limit: grantedLimit(limits, MetricCores)},
	}

	for _, node := range ordered {
		switch {
		case out.Servers.fits(1):
			out.Servers.take(node.Name, 1)
		case out.Cores.fits(coresOf(node.VCPU)):
			out.Cores.take(node.Name, coresOf(node.VCPU))
		case out.VCPU.fits(node.VCPU):
			out.VCPU.take(node.Name, node.VCPU)
		default:
			out.Unlicensed = append(out.Unlicensed, node.Name)
			out.UnlicensedVCPU += node.VCPU
		}
	}

	return out
}

// coresOf is what one node costs the cores metric under the temporary
// "2 vCPU = 1 core" rule of specification 7.2. It rounds up per node because a
// node is never split between two metrics.
func coresOf(vcpu int64) int64 { return (vcpu + 1) / 2 }

// grantedLimit resolves one metric into a published quota: nil for unlimited.
func grantedLimit(limits Limits, metric string) *int64 {
	granted, finite := limits.Of(metric)
	if !finite {
		return nil
	}
	return &granted
}

// SortNodes orders the licensable nodes by (vCPU descending, previously a server
// first, name ascending).
//
// The middle component is what keeps the allocation still: two nodes of the same
// size would otherwise swap places on every recompute, and the Console would
// blink. A node that already holds a server licence keeps it until a larger node
// appears or it goes away itself.
//
// The same order drives the attribution, so the largest nodes are covered first
// and the smallest ones are what ends up unlicensed. That minimises the number
// of unlicensed nodes for a given capacity.
func SortNodes(nodes []Node, wasServer map[string]bool) []Node {
	ordered := make([]Node, len(nodes))
	copy(ordered, nodes)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if a.VCPU != b.VCPU {
			return a.VCPU > b.VCPU
		}
		if wasServer[a.Name] != wasServer[b.Name] {
			return wasServer[a.Name]
		}
		return a.Name < b.Name
	})
	return ordered
}

// Consumption returns the three metrics of the licensable node set
// (specification 7.2). cores is computed on the sum, never per node: 24 nodes of
// 3 vCPU are 72 vCPU and 36 cores, not 48.
func Consumption(nodes []Node) map[string]int64 {
	var vcpu int64
	for _, node := range nodes {
		vcpu += node.VCPU
	}
	return map[string]int64{
		MetricServers: int64(len(nodes)),
		MetricVCPU:    vcpu,
		MetricCores:   (vcpu + 1) / 2,
	}
}
