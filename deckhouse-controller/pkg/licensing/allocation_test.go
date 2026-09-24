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

import (
	"fmt"
	"reflect"
	"testing"
)

// key builds the limits of a key the way the license server issues them: all
// three metrics are always present, the ones the customer did not buy as an
// explicit zero (specification 9.1).
func key(servers, vcpu, cores int64) Limits {
	return Limits{Speaking: true, Values: map[string]*int64{
		MetricServers: i64(servers), MetricVCPU: i64(vcpu), MetricCores: i64(cores),
	}}
}

// partial builds the limits of a key that omits a metric altogether, which by
// ADR 14.3 grants that metric without a limit.
func partial(values map[string]*int64) Limits {
	return Limits{Speaking: true, Values: values}
}

// S1 to S13 of specification 15.2.
func TestAllocate(t *testing.T) {
	cases := []struct {
		name       string
		limits     Limits
		nodes      []Node
		wasServer  map[string]bool
		servers    []string
		vcpu       []string
		cores      []string
		coresVCPU  []string
		unlicensed []string
	}{
		{
			name:       "S1 servers only, the pool is zero",
			limits:     key(2, 0, 0),
			nodes:      []Node{nd("a", 32), nd("b", 16), nd("c", 8)},
			servers:    []string{"a", "b"},
			unlicensed: []string{"c"},
		},
		{
			name:    "S2 the rest is paid for out of vCPU",
			limits:  key(2, 8, 0),
			nodes:   []Node{nd("a", 32), nd("b", 16), nd("c", 8)},
			servers: []string{"a", "b"},
			vcpu:    []string{"c"},
		},
		{
			name:       "S3 vCPU only",
			limits:     key(0, 100, 0),
			nodes:      []Node{nd("a", 32), nd("b", 32), nd("c", 32), nd("d", 8)},
			vcpu:       []string{"a", "b", "c"},
			unlicensed: []string{"d"},
		},
		{
			name:       "S4 cores count double",
			limits:     key(0, 0, 50),
			nodes:      []Node{nd("a", 32), nd("b", 32), nd("c", 32), nd("d", 8)},
			cores:      []string{"a", "b", "c"},
			unlicensed: []string{"d"},
		},
		{
			// 40 vCPU of cores pay for a and 8 of b, 24 vCPU of the vCPU pool the
			// rest of b; c takes 32 of the remaining 36 and d does not fit into 4.
			name:       "S5 the node that empties the cores pool takes the rest from vCPU",
			limits:     key(0, 60, 20),
			nodes:      []Node{nd("a", 32), nd("b", 32), nd("c", 32), nd("d", 8)},
			cores:      []string{"a"},
			coresVCPU:  []string{"b"},
			vcpu:       []string{"c"},
			unlicensed: []string{"d"},
		},
		{
			name:      "one core and 2 vCPU pay for a node of 4 together",
			limits:    key(0, 2, 1),
			nodes:     []Node{nd("a", 4)},
			coresVCPU: []string{"a"},
		},
		{
			name:      "the cores pool is drained before vCPU is touched",
			limits:    key(0, 2, 3),
			nodes:     []Node{nd("a", 4), nd("b", 4)},
			cores:     []string{"a"},
			coresVCPU: []string{"b"},
		},
		{
			name:       "a node both pools together cannot pay for is unlicensed",
			limits:     key(0, 1, 1),
			nodes:      []Node{nd("a", 4)},
			unlicensed: []string{"a"},
		},
		{
			name:       "an unlicensed node takes nothing, a smaller one after it still fits",
			limits:     key(0, 1, 1),
			nodes:      []Node{nd("a", 4), nd("b", 3)},
			coresVCPU:  []string{"b"},
			unlicensed: []string{"a"},
		},
		{
			name:   "unlimited cores pay for everything, vCPU is never touched",
			limits: partial(map[string]*int64{MetricServers: i64(0), MetricVCPU: i64(4), MetricCores: nil}),
			nodes:  []Node{nd("a", 1<<20), nd("b", 8)},
			cores:  []string{"a", "b"},
		},
		{
			name:      "unlimited vCPU pays for whatever the cores pool leaves",
			limits:    partial(map[string]*int64{MetricServers: i64(0), MetricVCPU: nil, MetricCores: i64(3)}),
			nodes:     []Node{nd("a", 4), nd("b", 4), nd("c", 4)},
			cores:     []string{"a"},
			coresVCPU: []string{"b"},
			vcpu:      []string{"c"},
		},
		{
			name:      "servers are never split",
			limits:    key(1, 2, 1),
			nodes:     []Node{nd("a", 8), nd("b", 4)},
			servers:   []string{"a"},
			coresVCPU: []string{"b"},
		},
		{
			name:       "S16 every metric is attributed in turn",
			limits:     key(1, 8, 4),
			nodes:      []Node{nd("a", 32), nd("b", 8), nd("c", 8), nd("d", 4)},
			servers:    []string{"a"},
			vcpu:       []string{"c"},
			cores:      []string{"b"},
			unlicensed: []string{"d"},
		},
		{
			name:       "S6 equal nodes, neither was a server before",
			limits:     key(1, 0, 0),
			nodes:      []Node{nd("b", 16), nd("a", 16)},
			servers:    []string{"a"},
			unlicensed: []string{"b"},
		},
		{
			name:       "S7 the previous server keeps the licence",
			limits:     key(1, 0, 0),
			nodes:      []Node{nd("a", 16), nd("b", 16)},
			wasServer:  map[string]bool{"b": true},
			servers:    []string{"b"},
			unlicensed: []string{"a"},
		},
		{
			name:       "S8 a larger node takes the server licence over",
			limits:     key(1, 0, 0),
			nodes:      []Node{nd("a", 16), nd("b", 16), nd("c", 32)},
			wasServer:  map[string]bool{"b": true},
			servers:    []string{"c"},
			unlicensed: []string{"b", "a"},
		},
		{
			name:      "S9 the previous server is gone",
			limits:    key(1, 0, 0),
			nodes:     []Node{nd("a", 16)},
			wasServer: map[string]bool{"b": true},
			servers:   []string{"a"},
		},
		{
			name:   "S10 a key omitting vCPU grants it without a limit",
			limits: partial(map[string]*int64{MetricServers: i64(0), MetricCores: i64(0)}),
			nodes:  []Node{nd("a", 32), nd("b", 1000)},
			vcpu:   []string{"b", "a"},
		},
		{
			name:    "S11 more server licences than nodes",
			limits:  key(10, 0, 0),
			nodes:   []Node{nd("a", 8), nd("b", 8), nd("c", 8), nd("d", 8)},
			servers: []string{"a", "b", "c", "d"},
		},
		{
			name:       "S13 no key at all",
			limits:     Limits{},
			nodes:      []Node{nd("a", 8), nd("b", 4)},
			unlicensed: []string{"a", "b"},
		},
		{
			name:   "an explicitly unlimited vCPU pays for a node of any size",
			limits: partial(map[string]*int64{MetricServers: i64(0), MetricVCPU: nil, MetricCores: i64(0)}),
			nodes:  []Node{nd("a", 1<<20)},
			vcpu:   []string{"a"},
		},
		{
			name:    "an unlimited vCPU takes everything the servers quota leaves",
			limits:  partial(map[string]*int64{MetricServers: i64(1), MetricVCPU: nil, MetricCores: i64(0)}),
			nodes:   []Node{nd("a", 32), nd("b", 32), nd("c", 8)},
			servers: []string{"a"},
			vcpu:    []string{"b", "c"},
		},
		{
			name:    "an unlimited servers quota covers every node",
			limits:  partial(map[string]*int64{MetricServers: nil, MetricVCPU: i64(0), MetricCores: i64(0)}),
			nodes:   []Node{nd("a", 8), nd("b", 4)},
			servers: []string{"a", "b"},
		},
		{
			name:       "vCPU is filled largest first, so the node left out is not the smallest one",
			limits:     key(0, 40, 0),
			nodes:      []Node{nd("small", 8), nd("big", 32), nd("mid", 16)},
			vcpu:       []string{"big", "small"},
			unlicensed: []string{"mid"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Allocate(tc.nodes, tc.limits, tc.wasServer)
			assertGroup(t, "Server", got.Servers.Nodes, tc.servers)
			assertGroup(t, "VCPU", got.VCPU.Nodes, tc.vcpu)
			assertGroup(t, "Cores", got.Cores.Nodes, tc.cores)
			assertGroup(t, "CoresVCPU", got.CoresVCPU, tc.coresVCPU)
			assertGroup(t, "Unlicensed", got.Unlicensed, tc.unlicensed)
			if len(got.CoresVCPU) > 1 {
				t.Fatalf("coresVCPU = %v, at most one node straddles", got.CoresVCPU)
			}
			if got.WithinLimits() != (len(tc.unlicensed) == 0) {
				t.Fatalf("withinLimits = %v with %d unlicensed nodes", got.WithinLimits(), len(got.Unlicensed))
			}
		})
	}
}

func assertGroup(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// S11: the published limits stay the quota that was granted, not the amount the
// nodes happened to use.
func TestAllocationPublishesTheGrantedLimits(t *testing.T) {
	got := Allocate([]Node{nd("a", 8), nd("b", 8), nd("c", 8), nd("d", 8)}, key(10, 6, 0), nil)
	if got.Servers.Limit == nil || *got.Servers.Limit != 10 {
		t.Fatalf("servers limit = %v, want 10", got.Servers.Limit)
	}
	if got.VCPU.Limit == nil || *got.VCPU.Limit != 6 {
		t.Fatalf("vCPU limit = %v, want 6", got.VCPU.Limit)
	}
	if got.Servers.Used != 4 || len(got.Servers.Nodes) != 4 {
		t.Fatalf("servers used = %d over %v, want 4", got.Servers.Used, got.Servers.Nodes)
	}

	// An unlimited metric publishes no limit at all, never a number standing in
	// for "no limit".
	unlimited := Allocate(nil, partial(map[string]*int64{MetricServers: nil}), nil)
	if unlimited.Servers.Limit != nil {
		t.Fatalf("servers limit = %d, want none", *unlimited.Servers.Limit)
	}
}

// A reissue that adds vCPU must not move a node covered by cores onto vCPU:
// cores come before vCPU, so the node stays where it was.
func TestAllocateKeepsCoresWhenVCPUGrows(t *testing.T) {
	nodes := []Node{nd("big", 8), nd("small", 4)}
	for _, limits := range []Limits{key(1, 0, 2), key(1, 4, 5)} {
		got := Allocate(nodes, limits, nil)
		assertGroup(t, "Servers", got.Servers.Nodes, []string{"big"})
		assertGroup(t, "Cores", got.Cores.Nodes, []string{"small"})
		assertGroup(t, "VCPU", got.VCPU.Nodes, nil)
	}
}

// Cores are a pool of 2*cores vCPU with no per-node rounding, so the cores
// metric (ceil(vCPU/2) on the sum) is exactly enough on its own.
func TestAllocateCoresArePooled(t *testing.T) {
	odd := func(n int, vcpu int64) []Node {
		out := make([]Node, 0, n)
		for i := range n {
			out = append(out, nd(fmt.Sprintf("n-%02d", i), vcpu))
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		nodes []Node
		cores int64
		used  int64
	}{
		{"three nodes of 3 vCPU fit into 5 cores", odd(3, 3), 5, 5},
		{"24 nodes of 3 vCPU fit into 36 cores", odd(24, 3), 36, 36},
		{"four nodes of 1 vCPU fit into 2 cores", odd(4, 1), 2, 2},
		{"7 vCPU on the sum cost 4 cores", []Node{nd("a", 3), nd("b", 3), nd("c", 1)}, 4, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Allocate(tc.nodes, key(0, 0, tc.cores), nil)
			if len(got.Unlicensed) != 0 || len(got.Cores.Nodes) != len(tc.nodes) {
				t.Fatalf("cores %v, unlicensed %v", got.Cores.Nodes, got.Unlicensed)
			}
			if got.Cores.Used != tc.used {
				t.Fatalf("cores used = %d, want %d", got.Cores.Used, tc.used)
			}
		})
	}

	// One core short of the sum leaves the last node out: without vCPU there is
	// nothing to pay the rest of it with.
	got := Allocate(odd(3, 3), key(0, 0, 4), nil)
	assertGroup(t, "Unlicensed", got.Unlicensed, []string{"n-02"})
}

// What the pools report as used when a node straddles them, and that a node
// left unlicensed draws on neither.
func TestAllocateUsedAcrossBothPools(t *testing.T) {
	for _, tc := range []struct {
		name        string
		limits      Limits
		nodes       []Node
		cores, vcpu int64
	}{
		{"one core and 2 vCPU for a node of 4", key(0, 2, 1), []Node{nd("a", 4)}, 1, 2},
		{"C=3, P=2 for two nodes of 4", key(0, 2, 3), []Node{nd("a", 4), nd("b", 4)}, 3, 2},
		{"an odd draw on cores rounds up", key(0, 10, 3), []Node{nd("a", 5)}, 3, 0},
		{"nothing is drawn for an unlicensed node", key(0, 1, 1), []Node{nd("a", 4)}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Allocate(tc.nodes, tc.limits, nil)
			if got.Cores.Used != tc.cores || got.VCPU.Used != tc.vcpu {
				t.Fatalf("used cores/vCPU = %d/%d, want %d/%d", got.Cores.Used, got.VCPU.Used, tc.cores, tc.vcpu)
			}
		})
	}
}

// Every non-server node is covered exactly when they sum to no more than
// 2*cores + vCPU: splitting one node across the pools wastes no capacity.
func TestAllocateCoversUpToTheWholePool(t *testing.T) {
	sizes := [][]int64{{4}, {4, 4}, {3, 3, 1}, {8, 5, 2}, {6, 6, 6, 1}}
	for _, set := range sizes {
		var nodes []Node
		var sum int64
		for i, v := range set {
			nodes = append(nodes, nd(fmt.Sprintf("n-%d", i), v))
			sum += v
		}
		for cores := int64(0); cores <= 10; cores++ {
			for vcpu := int64(0); vcpu <= 20; vcpu++ {
				got := Allocate(nodes, key(0, vcpu, cores), nil)
				if want := sum <= 2*cores+vcpu; got.WithinLimits() != want {
					t.Fatalf("nodes %v, cores %d, vCPU %d: withinLimits = %v, want %v (unlicensed %v)",
						set, cores, vcpu, got.WithinLimits(), want, got.Unlicensed)
				}
				if got.Cores.Used > cores || got.VCPU.Used > vcpu || len(got.CoresVCPU) > 1 {
					t.Fatalf("nodes %v, cores %d, vCPU %d: used %d/%d, coresVCPU %v",
						set, cores, vcpu, got.Cores.Used, got.VCPU.Used, got.CoresVCPU)
				}
			}
		}
	}
}

// S14: three hundred nodes lay out without special casing.
func TestAllocateManyNodes(t *testing.T) {
	nodes := make([]Node, 0, 300)
	for i := range 300 {
		nodes = append(nodes, nd(fmt.Sprintf("worker-%03d", i), 8))
	}

	// 12 whole-node licences, then 40 cores cover 10 nodes at ceil(8/2) each,
	// then 200 vCPU cover 25 more.
	got := Allocate(nodes, key(12, 200, 40), nil)
	if len(got.Servers.Nodes) != 12 || len(got.VCPU.Nodes) != 25 ||
		len(got.Cores.Nodes) != 10 || len(got.Unlicensed) != 253 {
		t.Fatalf("servers/vCPU/cores/unlicensed = %d/%d/%d/%d", len(got.Servers.Nodes),
			len(got.VCPU.Nodes), len(got.Cores.Nodes), len(got.Unlicensed))
	}
	if got.Servers.Used != 12 || got.VCPU.Used != 200 || got.Cores.Used != 40 {
		t.Fatalf("used servers/vCPU/cores = %d/%d/%d", got.Servers.Used, got.VCPU.Used, got.Cores.Used)
	}
	if got.UnlicensedVCPU != 253*8 {
		t.Fatalf("unlicensedVCPU = %d", got.UnlicensedVCPU)
	}
}

// The allocation is a pure function: the same input gives the same answer, and
// the input order of the nodes does not leak into it.
func TestAllocateIsStableUnderInputOrder(t *testing.T) {
	forward := []Node{nd("a", 32), nd("b", 32), nd("c", 16), nd("d", 8), nd("e", 8)}
	backward := []Node{nd("e", 8), nd("d", 8), nd("c", 16), nd("b", 32), nd("a", 32)}

	// The second set of limits makes one of the two equal 8 vCPU nodes straddle
	// the pools: the name decides which, whatever the input order.
	for _, limits := range []Limits{key(2, 16, 0), key(2, 6, 9)} {
		got := Allocate(forward, limits, nil)
		if !reflect.DeepEqual(got, Allocate(backward, limits, nil)) {
			t.Fatalf("allocation depends on the order the nodes arrived in")
		}
		if limits.Values[MetricCores] != nil && *limits.Values[MetricCores] == 9 {
			assertGroup(t, "Cores", got.Cores.Nodes, []string{"c"})
			assertGroup(t, "CoresVCPU", got.CoresVCPU, []string{"d"})
			assertGroup(t, "VCPU", got.VCPU.Nodes, nil)
			assertGroup(t, "Unlicensed", got.Unlicensed, []string{"e"})
		}
	}
}

// N9, N10, N11: the three metrics, with cores taken on the sum.
func TestConsumption(t *testing.T) {
	cases := []struct {
		name                 string
		nodes                []Node
		servers, vcpu, cores int64
	}{
		{name: "N10 24 nodes of 3 vCPU", nodes: sameSize(24, 3), servers: 24, vcpu: 72, cores: 36},
		{name: "N11 no licensable nodes", servers: 0, vcpu: 0, cores: 0},
		{name: "an odd total rounds the cores up", nodes: []Node{nd("a", 3)}, servers: 1, vcpu: 3, cores: 2},
		{name: "the example of the specification", nodes: sameSize(24, 16), servers: 24, vcpu: 384, cores: 192},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Consumption(tc.nodes)
			want := map[string]int64{MetricServers: tc.servers, MetricVCPU: tc.vcpu, MetricCores: tc.cores}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("consumption = %v, want %v", got, want)
			}
		})
	}
}

func sameSize(count int, vcpu int64) []Node {
	nodes := make([]Node, 0, count)
	for i := range count {
		nodes = append(nodes, nd(fmt.Sprintf("n%02d", i), vcpu))
	}
	return nodes
}

// The ordering rule of specification 7.4 in isolation.
func TestSortNodes(t *testing.T) {
	nodes := []Node{nd("b", 16), nd("a", 16), nd("c", 32), nd("d", 8)}

	plain := nodeNames(SortNodes(nodes, nil))
	if !reflect.DeepEqual(plain, []string{"c", "a", "b", "d"}) {
		t.Fatalf("order = %v", plain)
	}

	sticky := nodeNames(SortNodes(nodes, map[string]bool{"b": true}))
	if !reflect.DeepEqual(sticky, []string{"c", "b", "a", "d"}) {
		t.Fatalf("order with a previous server = %v", sticky)
	}
}

// Limits.Of is what tells "no key" from "the key does not name this metric".
func TestLimitsOf(t *testing.T) {
	cases := []struct {
		name   string
		limits Limits
		metric string
		value  int64
		finite bool
	}{
		{name: "a granted quota", limits: key(10, 0, 0), metric: MetricServers, value: 10, finite: true},
		{name: "an explicit zero is a real quota of nothing", limits: key(10, 0, 0), metric: MetricVCPU, finite: true},
		{name: "an explicit null is unlimited", limits: partial(map[string]*int64{MetricVCPU: nil}), metric: MetricVCPU},
		{name: "a metric a speaking key omits is unlimited", limits: key(10, 0, 0), metric: "gpu"},
		{name: "without a key nothing is granted", limits: Limits{}, metric: MetricVCPU, finite: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, finite := tc.limits.Of(tc.metric)
			if value != tc.value || finite != tc.finite {
				t.Fatalf("Of(%q) = %d, %v; want %d, %v", tc.metric, value, finite, tc.value, tc.finite)
			}
		})
	}
}
