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

package layout

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

// Origin says where a layout came from, which the agent reports so that an operator
// can tell "configured" from "running on what it remembers".
type Origin string

const (
	// OriginAPI means the layout was read from the API server.
	OriginAPI Origin = "API"

	// OriginCache means the API server could not be reached and the layout came from
	// the on-disk copy. The node keeps pulling images; it just will not learn about a
	// change until the API comes back.
	OriginCache Origin = "Cache"

	// OriginBootstrap means the node has never had a layout from the API server and is
	// running on the one it was installed with. Expected while a cluster is being
	// bootstrapped, and a sign of a node that cannot reach the API at all otherwise.
	OriginBootstrap Origin = "Bootstrap"
)

// Snapshot is a layout together with where it came from.
type Snapshot struct {
	Spec       *registryv1alpha1.RegistryNodeSpec
	Generation int64
	Origin     Origin
}

// Source produces the node layout, preferring the API and falling back to disk.
type Source struct {
	Log    *slog.Logger
	Client client.Client

	// Node is the name of this node, and of its layout object.
	Node string

	// Cache is the on-disk copy.
	Cache *Cache

	// Bootstrap is the layout the node was installed with. Optional, and used only
	// when the API server has never been reached.
	Bootstrap *Bootstrap

	// Resolver fills in the credentials the layout references. Required whenever a
	// layout from the API can reference any, which is always in practice.
	Resolver *Resolver
}

// Validate refuses a Source that cannot do its job, at startup rather than per pass.
//
// Worth a method of its own because of how the alternative failed. With no resolver wired,
// every pass reported "the API server is unreachable, running on the layout the node was
// installed with" — true in effect and false in fact, and an operator reading it would go and
// examine a healthy API server. The node kept pulling from its bootstrap layout and the agent
// reported success, so nothing else said otherwise.
func (s *Source) Validate() error {
	switch {
	case s.Node == "":
		return fmt.Errorf("no node name, so there is no layout to look for")
	case s.Cache == nil:
		return fmt.Errorf("no on-disk copy, so an API outage would stop this node pulling")
	case s.Resolver == nil:
		return fmt.Errorf("no credential resolver, so any layout naming its credentials " +
			"would be refused and quietly replaced by the one this node was installed with")
	}
	return nil
}

// Get returns the layout to apply and where it came from.
//
// A read failure is not propagated as long as there is a copy on disk. That is the
// whole point of the copy: an unreachable API server must not stop a node from
// pulling images, because the images it cannot pull include the ones that would fix
// the API server.
//
// A layout that is genuinely absent — deleted, or a node that is not managed — is
// distinguished from an unreachable API and returns nothing rather than the copy: an
// object that was removed on purpose must not be resurrected from a cache forever.
func (s *Source) Get(ctx context.Context) (*Snapshot, error) {
	if s.Node == "" {
		return nil, fmt.Errorf("no node name")
	}

	// No client at all is the same situation as one that cannot reach the API: the node
	// has no credentials yet, which is where a node being bootstrapped starts. Handled
	// as a read failure so that there is exactly one fallback path to reason about.
	if s.Client == nil {
		return s.withoutTheAPI(errNoCredentials)
	}

	object := &registryv1alpha1.RegistryNode{}
	err := s.Client.Get(ctx, types.NamespacedName{Name: s.Node}, object)

	switch {
	case err == nil:
		// Before the copy is written, so what lands on disk can authenticate on its own.
		// A reference cannot be dereferenced when the API server is gone, which is the
		// one situation the copy exists for.
		if resolveErr := s.Resolver.Resolve(ctx, s.Client, &object.Spec); resolveErr != nil {
			// Treated as a failed read rather than propagated: a layout whose credentials
			// could not be read is one that cannot pull, and the copy on disk still can.
			s.Log.Error("cannot resolve the credentials the layout references; "+
				"falling back to the layout already on disk",
				"error", resolveErr.Error())
			return s.withoutTheAPI(resolveErr)
		}

		stored := Stored{Generation: object.Generation, Spec: &object.Spec}
		if saveErr := s.Cache.Save(stored); saveErr != nil {
			// Not fatal: the layout at hand is good, and failing here would trade a
			// working pull path for a disk problem. But it does mean the next restart
			// during an outage has nothing to fall back on, so it is worth saying.
			s.Log.Error("cannot store the layout for use without the API server",
				"error", saveErr.Error())
		}
		return &Snapshot{Spec: stored.Spec, Generation: stored.Generation, Origin: OriginAPI}, nil

	case isNotFound(err):
		// Two situations look identical from here, and the cache is what tells them
		// apart: whether this node has ever had a layout.
		//
		// If it has, the object was removed on purpose — the node is no longer managed,
		// and reviving it from disk would keep a node configured after the cluster
		// decided otherwise. If it has not, the layout has not been compiled yet, which
		// is the normal state of a node bootstrapping into a new cluster, and the one
		// it was installed with is what carries it until then.
		stored, cacheErr := s.Cache.Load()
		if cacheErr != nil || stored != nil {
			return nil, nil
		}
		return s.fromBootstrap()
	}

	return s.withoutTheAPI(err)
}

// errNoCredentials is the absence of any way to ask, which the fallback treats exactly
// like an API server that will not answer.
var errNoCredentials = errors.New("the node has no credentials for the API server yet")

// withoutTheAPI answers from the node's own disk, given why the API could not.
//
// Two files, in order of authority: the cache holds what the cluster last said, the
// bootstrap file holds what the node was installed with. The cluster's answer is the
// newer of the two whenever it exists.
func (s *Source) withoutTheAPI(reason error) (*Snapshot, error) {
	cached, cacheErr := s.Cache.Load()
	if cacheErr != nil {
		return nil, fmt.Errorf("the API server is unreachable (%w) and the stored layout is unusable: %w", reason, cacheErr)
	}

	if cached == nil {
		// Nothing remembered, so fall back to what the node was installed with. This is
		// the path a first master takes: it has to pull the control plane before there
		// is a control plane to ask.
		snapshot, bootstrapErr := s.fromBootstrap()
		if bootstrapErr != nil {
			return nil, fmt.Errorf("the API server is unreachable (%w) and the bootstrap layout is unusable: %w",
				reason, bootstrapErr)
		}
		if snapshot != nil {
			s.Log.Warn("the API server is unreachable, running on the layout the node was installed with",
				"error", reason.Error())
			return snapshot, nil
		}
		// Nothing to fall back on at all: a node that has never reached the API and was
		// installed without a layout. Reported as an error, since there is none to
		// apply and no way to invent one.
		return nil, fmt.Errorf("the API server is unreachable and no layout has ever been stored: %w", reason)
	}

	s.Log.Warn("the API server is unreachable, running on the stored layout", "error", reason.Error())
	return &Snapshot{Spec: cached.Spec, Generation: cached.Generation, Origin: OriginCache}, nil
}

// fromBootstrap returns the layout the node was installed with, or nothing.
//
// Not written to the cache. The cache means "this is what the cluster last said", and
// the two must not become indistinguishable: a node whose seed had been promoted into
// the cache would refuse to fall back to the seed after the layout was legitimately
// deleted, and would keep the runtime pointed at itself forever.
func (s *Source) fromBootstrap() (*Snapshot, error) {
	spec, err := s.Bootstrap.Load()
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return nil, nil
	}
	// Generation zero, because it belongs to no object version. The status then reports
	// an observed generation of zero, which is exactly right: no layout of the cluster's
	// has been applied.
	return &Snapshot{Spec: spec, Generation: 0, Origin: OriginBootstrap}, nil
}

func isNotFound(err error) bool {
	return client.IgnoreNotFound(err) == nil && err != nil
}
