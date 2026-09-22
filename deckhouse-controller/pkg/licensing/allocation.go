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

// Billing groups a node can land in. The value is a string enum on purpose:
// splitting the pool into separate vCPU and cores groups later adds values
// instead of changing the shape of the status (specification 14).
const (
	BillingFree       = "Free"
	BillingServer     = "Server"
	BillingPool       = "Pool"
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

// Allocation is the assignment of every licensable node to a billing group,
// together with the totals the status and the Console show next to it.
type Allocation struct {
	// Servers, Pool and Unlicensed are node names, in allocation order.
	Servers    []string
	Pool       []string
	Unlicensed []string

	// ServersLimit is the servers quota the allocation was computed against;
	// nil means the quota is unlimited.
	ServersLimit *int64
	// PoolCapacity is vCPU + 2*cores; nil means the pool is unbounded.
	PoolCapacity *int64
	// PoolUsedVCPU is the vCPU of the nodes that fit into the pool.
	PoolUsedVCPU int64
	// UnlicensedVCPU is the vCPU of the nodes that fit nowhere.
	UnlicensedVCPU int64
}

// WithinLimits reports whether every licensable node is covered.
func (a Allocation) WithinLimits() bool { return len(a.Unlicensed) == 0 }

// BillingByNode indexes the allocation by node name.
func (a Allocation) BillingByNode() map[string]string {
	out := make(map[string]string, len(a.Servers)+len(a.Pool)+len(a.Unlicensed))
	for _, group := range []struct {
		names   []string
		billing string
	}{
		{a.Servers, BillingServer},
		{a.Pool, BillingPool},
		{a.Unlicensed, BillingUnlicensed},
	} {
		for _, name := range group.names {
			out[name] = group.billing
		}
	}
	return out
}

// Allocate lays the licensable nodes out over the metrics of the key
// (specification 7.3). It is a pure function of the node set, the effective
// limits and the previous allocation.
//
// wasServer is the previous allocation, read off status.nodes[]; it only breaks
// ties between equally sized nodes.
func Allocate(nodes []Node, limits Limits, wasServer map[string]bool) Allocation {
	ordered := SortNodes(nodes, wasServer)

	out := Allocation{PoolCapacity: poolCapacity(limits)}

	// An unlimited servers quota covers every node there is, which is the same
	// answer as a limit equal to the node count. The published limit keeps the
	// quota as granted, so that the Console can say "12 of 12".
	servers := int64(len(ordered))
	if granted, finite := limits.Of(MetricServers); finite {
		servers = granted
		out.ServersLimit = &granted
	}

	for i, node := range ordered {
		switch {
		case int64(i) < servers:
			out.Servers = append(out.Servers, node.Name)
		case out.PoolCapacity == nil || out.PoolUsedVCPU+node.VCPU <= *out.PoolCapacity:
			out.Pool = append(out.Pool, node.Name)
			out.PoolUsedVCPU += node.VCPU
		default:
			out.Unlicensed = append(out.Unlicensed, node.Name)
			out.UnlicensedVCPU += node.VCPU
		}
	}

	return out
}

// poolCapacity is vCPU + 2*cores. Either metric being unlimited makes the whole
// pool unbounded (ADR 14.3), which is reported as a nil capacity. The two share
// one pool: a key granting 60 vCPU and 20 cores covers a hundred vCPU of nodes
// however they are sized.
func poolCapacity(limits Limits) *int64 {
	vcpu, vcpuFinite := limits.Of(MetricVCPU)
	cores, coresFinite := limits.Of(MetricCores)
	if !vcpuFinite || !coresFinite {
		return nil
	}
	capacity := vcpu + 2*cores
	return &capacity
}

// SortNodes orders the licensable nodes by (vCPU descending, previously a server
// first, name ascending).
//
// The middle component is what keeps the allocation still: two nodes of the same
// size would otherwise swap places on every recompute, and the Console would
// blink. A node that already holds a server licence keeps it until a larger node
// appears or it goes away itself.
//
// The same order fills the pool, so the largest nodes are covered first and the
// smallest ones are what ends up unlicensed. That minimises the number of
// unlicensed nodes for a given capacity.
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
