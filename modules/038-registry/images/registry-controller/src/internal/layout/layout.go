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
	"fmt"

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

	// Leader is the address of the replica that holds the election lease, when it has reported.
	//
	// Agents are pointed at it first and reach the others as its mirrors. See LeaderAddress for
	// what pointing them at a follower cost.
	Leader string

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

	// PersistedCredentials are the credentials this module wrote last time, read back
	// from its auth Secret and keyed exactly as they were written.
	//
	// They exist for one case: a held upstream. AppliedUpstream comes from a status, and
	// a status deliberately records addresses without credentials — so the moment the
	// configuration stops supplying them, the only surviving copy is the Secret. Read
	// back here, they are reattached to the upstream that is being held.
	//
	// Without this the hold keeps the address and loses the ability to use it, which is
	// worse than not holding it at all: every node keeps a fallback it cannot
	// authenticate to, so a cache miss stops being slower and becomes a failure, and any
	// image reference naming the upstream by its own name is answered with a 401 the
	// agent has no credentials to avoid. Measured on a cluster that had asked for
	// air-gap with an incomplete cache — which is exactly the state the hold exists for.
	PersistedCredentials map[string]registryv1alpha1.Auth

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
	//
	// Carries no credentials: the caller records this and the recorded copy must not be
	// one more place a credential can be read from. Credentials for it are in
	// Credentials under AuthKeyUpstream.
	HeldUpstream *registryv1alpha1.Upstream

	// Credentials are what the references in the layouts above point at, keyed the same
	// way. The caller persists them into the module's auth Secret before applying
	// anything that references them.
	//
	// Returned rather than written here so this stays a pure function, and returned by
	// the same code that writes the references so the two cannot disagree about a key —
	// a mismatch would leave an agent authenticating with nothing and a 401 that says
	// nothing about why.
	Credentials map[string]registryv1alpha1.Auth
}

// Compute derives the desired layout.
//
// The two configuration axes are `Config.Storage.Cache` and whether
// `Config.Primary.Upstream` is set. Three of their four combinations are valid
// and are handled here; the fourth (no cache, no upstream) leaves nodes with no
// source of images and is rejected before it reaches this point.
func Compute(in Inputs) Desired {
	return withReferencedAuth(compute(in))
}

func compute(in Inputs) Desired {
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
		// Held, so it arrives without credentials: see Inputs.PersistedCredentials.
		effectiveUpstream = withPersistedAuth(in.AppliedUpstream, in.PersistedCredentials)
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
		// The authenticated write endpoint is published on every managed cluster.
		//
		// It used to exist exactly in air-gap, which made the documented order unwalkable: `d8 mirror
		// push` needs the endpoint, the endpoint needed air-gap, and asking for air-gap drops the
		// upstream — so a bundle could never be in the store BEFORE the transition it is meant to make
		// safe. Publishing always also lets an operator put their own modules into any cluster's store.
		//
		// It is a SEPARATE distribution instance over the same data directory, not this one, and that
		// is what makes it possible at all: one instance cannot both proxy an upstream and accept a
		// push (measured: `POST /v2/.../blobs/uploads/` refused), so publishing through the serving
		// instance would turn a cache into a plain registry.
		Publish: true,

		// And the transition window, which publication used to stand for and no longer can.
		// StoreIsAuthority in the syncer is derived from this, so that how completeness is measured
		// does not change on clusters that merely cache.
		AirGapRequested: wantAirGap,
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

// withReferencedAuth moves every credential out of the derived resources and leaves a
// reference in its place, returning the credentials for the caller to persist.
//
// Keyed by what a credential belongs to rather than by who reads it: every node is handed
// the same credentials, so a per-node key would multiply one secret into as many copies as
// there are nodes and give an agent a reason to read another node's.
func withReferencedAuth(d Desired) Desired {
	credentials := map[string]registryv1alpha1.Auth{}

	backendKey := func(name registryv1alpha1.BackendName) string {
		switch name {
		case registryv1alpha1.BackendStorage:
			return constant.AuthKeyStorage
		case registryv1alpha1.BackendUpstream:
			return constant.AuthKeyUpstream
		default:
			// An upstream declared as its own resource. Its name is unique among them,
			// which is what makes it usable as a key.
			return string(name)
		}
	}

	referenceBackends := func(backends []registryv1alpha1.Backend) {
		for i := range backends {
			key := backendKey(backends[i].Name)
			// Kept before the endpoint is rewritten: the comparison below is against the
			// credentials it had, and by then it has none.
			original := backends[i].Endpoint
			collectAuth(credentials, original, key)
			backends[i].Endpoint = authRef(original, key)
			for j := range backends[i].Mirrors {
				mirrorKey := sharedOrOwnKey(original, backends[i].Mirrors[j], key, j)
				collectAuth(credentials, backends[i].Mirrors[j], mirrorKey)
				backends[i].Mirrors[j] = authRef(backends[i].Mirrors[j], mirrorKey)
			}
		}
	}

	referenceRoutes := func(routes []registryv1alpha1.Route) {
		for i := range routes {
			// Keyed by what the route intercepts, which is the only thing that
			// distinguishes one route from another.
			key := "route-" + routes[i].Match
			original := routes[i].Endpoint
			collectAuth(credentials, original, key)
			routes[i].Endpoint = authRef(original, key)
			for j := range routes[i].Mirrors {
				mirrorKey := sharedOrOwnKey(original, routes[i].Mirrors[j], key, j)
				collectAuth(credentials, routes[i].Mirrors[j], mirrorKey)
				routes[i].Mirrors[j] = authRef(routes[i].Mirrors[j], mirrorKey)
			}
		}
	}

	for node, spec := range d.Nodes {
		referenceBackends(spec.Backends)
		referenceRoutes(spec.AdditionalRoutes)
		d.Nodes[node] = spec
	}

	if d.Storage != nil && d.Storage.Upstream != nil {
		// Copied before it is touched: this is the same object the caller passed in as
		// the configured upstream, so replacing its credentials in place would rewrite
		// the input — and the caller would then compare a configuration against itself
		// and conclude nothing had changed.
		d.Storage.Upstream = d.Storage.Upstream.DeepCopy()
		upstream := d.Storage.Upstream
		originalUpstream := upstream.Endpoint
		collectAuth(credentials, originalUpstream, constant.AuthKeyUpstream)
		upstream.Endpoint = authRef(originalUpstream, constant.AuthKeyUpstream)
		for j := range upstream.Mirrors {
			key := sharedOrOwnKey(originalUpstream, upstream.Mirrors[j], constant.AuthKeyUpstream, j)
			collectAuth(credentials, upstream.Mirrors[j], key)
			upstream.Mirrors[j] = authRef(upstream.Mirrors[j], key)
		}
	}

	if d.HeldUpstream != nil {
		// Recorded for an operator to read, so it keeps the addresses and loses the
		// credentials outright — a reference here would only invite something to
		// resolve it, and nothing needs to.
		held := d.HeldUpstream.DeepCopy()
		collectAuth(credentials, held.Endpoint, constant.AuthKeyUpstream)
		held.Endpoint.Auth = nil
		for j := range held.Mirrors {
			collectAuth(credentials, held.Mirrors[j], mirrorAuthKey(constant.AuthKeyUpstream, j))
			held.Mirrors[j].Auth = nil
		}
		d.HeldUpstream = held
	}

	if len(credentials) > 0 {
		d.Credentials = credentials
	}
	return d
}

// authRef replaces credentials with a reference to where they are kept.
//
// Applied to every endpoint of every resource this module derives. Those resources are
// cluster-scoped, and the agent's role is bound to the `system:nodes` group, so a
// credential written into one of them is readable through the API by every kubelet in the
// cluster — including the credentials of registries that node never pulls from.
func authRef(endpoint registryv1alpha1.Endpoint, key string) registryv1alpha1.Endpoint {
	out := *endpoint.DeepCopy()
	if out.Auth.IsEmpty() {
		out.Auth = nil
		return out
	}
	out.Auth = &registryv1alpha1.Auth{
		SecretRef: &registryv1alpha1.AuthSecretRef{
			Name: constant.AuthSecretName,
			Key:  key,
		},
	}
	return out
}

// sharedOrOwnKey decides whether a mirror needs a key of its own.
//
// Mirrors of one backend can each need different credentials, so they cannot simply share
// the backend's key. But the in-cluster cache is the opposite case: every replica is a
// mirror of the same store and every one of them takes the same read credentials, and
// giving each its own key would write one credential into the Secret as many times as
// there are masters.
func sharedOrOwnKey(
	endpoint, mirror registryv1alpha1.Endpoint, baseKey string, index int,
) string {
	if mirror.Auth.Encoded() == endpoint.Auth.Encoded() {
		return baseKey
	}
	return mirrorAuthKey(baseKey, index)
}

// mirrorAuthKey keys a mirror's own credentials. Mirrors serve the same content but can
// each need different credentials, so they cannot share one key.
func mirrorAuthKey(base string, index int) string {
	return fmt.Sprintf("%s-mirror-%d", base, index)
}

// collectAuth records an endpoint's credentials under the key its reference will use.
func collectAuth(into map[string]registryv1alpha1.Auth, endpoint registryv1alpha1.Endpoint, key string) {
	if endpoint.Auth.IsEmpty() {
		return
	}
	into[key] = *endpoint.Auth.DeepCopy()
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

	// The leader first, the rest as its mirrors.
	//
	// Not cosmetic: an agent does not move to the next target on a 404, because for the
	// Storage-then-Upstream chain a 404 means the image is genuinely absent. Mirrors share that
	// list and that rule, so whichever replica is first decides what the node can pull — and a
	// follower that is one image behind answers 404 for an image the cluster holds. The leader is
	// the replica whose completeness authorized dropping the upstream, so it is the one to ask.
	//
	// Falls back to the given order when no leader has reported: some order is needed, and the
	// alternative — serving no storage backend at all — is worse than an imperfect first choice.
	primary := access.Addresses[0]
	if access.Leader != "" {
		for _, address := range access.Addresses {
			if address == access.Leader {
				primary = address
				break
			}
		}
	}

	backend := registryv1alpha1.Backend{
		Name:     registryv1alpha1.BackendStorage,
		Endpoint: endpoint(primary),
	}
	for _, address := range access.Addresses {
		if address != primary {
			backend.Mirrors = append(backend.Mirrors, endpoint(address))
		}
	}
	return backend
}

// withPersistedAuth gives a held upstream back the credentials it was working with.
//
// A held upstream comes from RegistryConfig.status, which records addresses and not
// credentials — deliberately, so a status is not one more place to read a license from. The
// surviving copy is the Secret this module wrote, and this is where it goes back on, under the
// same keys withReferencedAuth used to write it: the endpoint under AuthKeyUpstream, mirror j
// under its own key.
//
// Only ever fills what is empty. A configured upstream arrives with its own credentials and
// must keep them; this exists for the one that does not have any left.
func withPersistedAuth(
	upstream *registryv1alpha1.Upstream, persisted map[string]registryv1alpha1.Auth,
) *registryv1alpha1.Upstream {
	if upstream == nil || len(persisted) == 0 {
		return upstream
	}

	fill := func(endpoint *registryv1alpha1.Endpoint, key string) {
		if !endpoint.Auth.IsEmpty() {
			return
		}
		if auth, ok := persisted[key]; ok && !auth.IsEmpty() {
			endpoint.Auth = auth.DeepCopy()
		}
	}

	// Copied, because the caller passed in an object it also compares against: filling
	// this in place would rewrite the recorded upstream and the comparison would then be
	// against itself.
	out := upstream.DeepCopy()
	fill(&out.Endpoint, constant.AuthKeyUpstream)
	for j := range out.Mirrors {
		fill(&out.Mirrors[j], mirrorAuthKey(constant.AuthKeyUpstream, j))
	}
	return out
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
