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

// Package layout turns the resolved configuration into the two things the data
// plane actually reads: how the storage operates, and what each node's agent
// should do.
//
// The computation is a pure function of its inputs, with no client and no
// cluster access, because this is where the only conditional transition in the
// whole design lives — dropping the upstream to go air-gap. That transition is
// the one place a mistake cuts every node off from images, so it has to be
// exhaustively testable without a cluster.
package layout

import (
	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// Access is how a client reaches the in-cluster storage: where it is, the
// certificate authority that signs its serving certificate, and the read
// credentials.
//
// Sourced by the caller from the storage PKI. Kept as an explicit input so this
// package stays free of any opinion about where PKI comes from.
type Access struct {
	// Addresses are the node addresses the replicas are reachable at, as
	// "host:port".
	//
	// Node addresses rather than the Service name, and this is the one thing about
	// the storage that is easy to get wrong. A node agent runs in the host network
	// and resolves through the host's resolver, where `registry.d8-system.svc` does
	// not exist. That name is a key that identifies the image set — it is what every
	// image reference in the cluster is built from and what the agent matches a
	// request against — but nothing ever dials it. The serving certificate covers
	// these addresses for exactly this reason.
	Addresses []string

	CA   string
	Auth *registryv1alpha1.Auth
}

// Inputs is everything the layout depends on.
type Inputs struct {
	// Config is the resolved configuration, already validated.
	Config registryv1alpha1.RegistryConfigSpec

	// AppliedUpstream is the upstream currently in effect, as recorded in
	// RegistryConfig.status.effectiveUpstream.
	//
	// This is the last-known-good holder, and it does double duty. When the user
	// removes the upstream to go air-gap, the configuration says "none" while this
	// still says "the old one", which keeps nodes served until the cache is
	// complete. When a changed upstream fails its preflight probe, this is what the
	// cluster stays on.
	AppliedUpstream *registryv1alpha1.Upstream

	// UpstreamProbeFailed reports that the configured upstream did not pass its
	// preflight probe, so the cluster must stay on AppliedUpstream.
	//
	// Separated from the probe itself so that this package stays a pure function:
	// the decision is testable without a network, and the probe is testable without
	// a layout.
	UpstreamProbeFailed bool

	// LeaderFull reports whether the storage leader holds the whole expected set.
	// It gates the air-gap transition.
	LeaderFull bool

	// Nodes are the names of the cluster nodes that need an agent layout.
	Nodes []string

	// Routes are the accepted RegistryUpstream objects, already compiled and
	// arbitrated. Always transit, never cached.
	Routes []registryv1alpha1.Route

	// StorageAccess is how agents reach the storage. Required when the cache is
	// enabled, including at least one address.
	StorageAccess Access

	// MaintenanceWindows are the windows of the master node group, used to place the
	// garbage collection when nothing else says where it goes. An operator has already
	// declared those hours safe for disruption, and collecting is a small disruption of
	// exactly that kind.
	MaintenanceWindows []MaintenanceWindow
}

// Desired is the layout to converge on.
type Desired struct {
	// Storage is the spec of RegistryStorage, or nil when no storage is needed
	// and the object should not exist.
	Storage *registryv1alpha1.RegistryStorageSpec

	// Nodes maps a node name to the layout its agent must apply.
	Nodes map[string]registryv1alpha1.RegistryNodeSpec

	// DropUpstream reports that this reconciliation is the moment the cluster
	// becomes air-gapped: the cache is complete, so the upstream is removed from
	// both the storage and the node layouts at once.
	DropUpstream bool

	// HeldUpstream is the upstream still in effect, which is not necessarily the
	// configured one. Reported so the caller can explain to an operator why an
	// upstream they removed is still being used.
	HeldUpstream *registryv1alpha1.Upstream
}

// Compute derives the desired layout.
//
// The two configuration axes are `Config.Storage.Cache` and whether
// `Config.Primary.Upstream` is set. Three of their four combinations are valid
// and are handled here; the fourth (no cache, no upstream) leaves nodes with no
// source of images and is rejected before it reaches this point.
func Compute(in Inputs) Desired {
	if in.Config.Mode == registryv1alpha1.ModeUnmanaged {
		// The module owns nothing, so there is nothing to lay out. Note this is not
		// the same as "delete everything": the caller decides what to do with
		// objects that already exist.
		return Desired{}
	}

	desiredUpstream := in.Config.Primary.Upstream

	// The upstream that is actually in effect, which is not necessarily the
	// configured one. Two things can hold the cluster back on the previous
	// upstream, and both matter more than honouring the request promptly:
	//
	//   - the new one failed its preflight probe, so switching would break pulls;
	//   - the configuration asks for air-gap, but the cache cannot stand alone yet.
	effectiveUpstream := desiredUpstream
	if in.UpstreamProbeFailed || effectiveUpstream == nil {
		effectiveUpstream = in.AppliedUpstream
	}

	if !in.Config.Storage.Cache {
		// The agent forwards straight to the upstream. There is no storage and thus
		// no air-gap to reach, but a failed probe still has to leave the nodes on the
		// upstream that was working: a broken license must not stop them pulling.
		return Desired{
			Nodes:        nodeLayouts(in.Nodes, false, upstreamBackends(effectiveUpstream), in.Routes),
			HeldUpstream: effectiveUpstream,
		}
	}

	if len(in.StorageAccess.Addresses) == 0 {
		// The cache is configured but there is nowhere to send anyone. Rather than lay
		// out a cache nobody can reach, the nodes are pointed at the upstream — which is
		// what they would fall back to anyway, only without a round of failures first.
		//
		// Not reachable in practice: the caller refuses to build a layout from an
		// incomplete storage access secret, precisely because this is the input that is
		// expensive to get wrong. Kept because the alternative to a guard here is a
		// controller that panics, and a panicking controller stops reconciling
		// everything else too.
		return Desired{
			Nodes:        nodeLayouts(in.Nodes, false, upstreamBackends(effectiveUpstream), in.Routes),
			HeldUpstream: effectiveUpstream,
		}
	}

	// The single conditional transition in the design. Everything else is an
	// idempotent reconfiguration; this one is gated on the LEADER being complete,
	// deliberately not on every replica: followers are filled ahead of time and in
	// parallel, so waiting for all of them would delay the transition without
	// making it safer.
	wantAirGap := desiredUpstream == nil
	dropUpstream := wantAirGap && in.LeaderFull

	heldUpstream := effectiveUpstream
	if dropUpstream {
		heldUpstream = nil
	}

	storage := &registryv1alpha1.RegistryStorageSpec{
		Upstream: heldUpstream,
		Source:   in.Config.Storage.Source,
		Store: registryv1alpha1.StorageStore{
			Path: constant.StorePath,
			Size: in.Config.Storage.Size,
		},
		// The authenticated write endpoint exists exactly when there is no upstream
		// to pull from, because that is when `d8 mirror push` is the only way in.
		// Publishing it unconditionally would add an internet-facing write surface
		// to every cluster that merely caches.
		Publish: wantAirGap,
		// Filling the leader is a task only when there is something to fill it
		// from. Replication to the followers is driven by the leader itself, not by
		// this flag.
		NeedSync: heldUpstream != nil && !in.LeaderFull,
		// Resolved here rather than left to each replica, so that every replica agrees on
		// when its turn comes round: three replicas reading the same instruction queue
		// behind one lease, and three replicas each picking their own hour, are different
		// arrangements.
		GarbageCollection: garbageCollection(in),
	}

	upstreams := upstreamBackends(heldUpstream)
	backends := make([]registryv1alpha1.Backend, 0, 1+len(upstreams))
	backends = append(backends, storageBackend(in.StorageAccess))
	backends = append(backends, upstreams...)

	return Desired{
		Storage:      storage,
		Nodes:        nodeLayouts(in.Nodes, true, backends, in.Routes),
		DropUpstream: dropUpstream,
		HeldUpstream: heldUpstream,
	}
}

// storageBackend describes the in-cluster cache as one backend with one endpoint
// per replica.
//
// Every replica holds the same content, so which one answers does not matter and
// the rest are mirrors the agent fails over to. That is also why they are mirrors
// of a single backend rather than several backends: a backend is a distinct source
// of images, and these are one source reachable in several places.
// garbageCollection compiles the collection instruction the replicas act on.
func garbageCollection(in Inputs) *registryv1alpha1.GarbageCollection {
	configured := in.Config.Storage.GarbageCollection

	// Enabled unless it was explicitly turned off. A store nothing ever reclaims fills and
	// then stops being able to pull, which is not a state to arrive at by omission.
	enabled := true
	schedule := ""
	if configured != nil {
		enabled = configured.Enabled
		schedule = configured.Schedule
	}

	return &registryv1alpha1.GarbageCollection{
		Enabled:  enabled,
		Schedule: GarbageCollectionSchedule(schedule, in.MaintenanceWindows),
	}
}

func storageBackend(access Access) registryv1alpha1.Backend {
	endpoint := func(address string) registryv1alpha1.Endpoint {
		return registryv1alpha1.Endpoint{
			Scheme: registryv1alpha1.SchemeHTTPS,
			Host:   address,
			Path:   constant.Path,
			CA:     access.CA,
			Auth:   access.Auth,
		}
	}

	backend := registryv1alpha1.Backend{
		Name:     registryv1alpha1.BackendStorage,
		Endpoint: endpoint(access.Addresses[0]),
	}
	for _, address := range access.Addresses[1:] {
		backend.Mirrors = append(backend.Mirrors, endpoint(address))
	}
	return backend
}

func upstreamBackends(upstream *registryv1alpha1.Upstream) []registryv1alpha1.Backend {
	if upstream == nil {
		return nil
	}
	return []registryv1alpha1.Backend{{
		Name:     registryv1alpha1.BackendUpstream,
		Endpoint: *upstream.Endpoint.DeepCopy(),
		Mirrors:  deepCopyEndpoints(upstream.Mirrors),
	}}
}

func nodeLayouts(
	nodes []string, cache bool, backends []registryv1alpha1.Backend, routes []registryv1alpha1.Route,
) map[string]registryv1alpha1.RegistryNodeSpec {
	if len(nodes) == 0 {
		return nil
	}

	out := make(map[string]registryv1alpha1.RegistryNodeSpec, len(nodes))
	for _, node := range nodes {
		// Every node gets its own copy. They are identical today, but they are
		// separate objects and must not share backing arrays: a future per-node
		// difference would otherwise mutate every other node's layout.
		out[node] = registryv1alpha1.RegistryNodeSpec{
			Cache:            cache,
			Backends:         deepCopyBackends(backends),
			AdditionalRoutes: deepCopyRoutes(routes),
		}
	}
	return out
}

func deepCopyBackends(in []registryv1alpha1.Backend) []registryv1alpha1.Backend {
	if in == nil {
		return nil
	}
	out := make([]registryv1alpha1.Backend, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func deepCopyRoutes(in []registryv1alpha1.Route) []registryv1alpha1.Route {
	if in == nil {
		return nil
	}
	out := make([]registryv1alpha1.Route, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func deepCopyEndpoints(in []registryv1alpha1.Endpoint) []registryv1alpha1.Endpoint {
	if in == nil {
		return nil
	}
	out := make([]registryv1alpha1.Endpoint, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}
