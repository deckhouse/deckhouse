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

package profile

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/domain/fsm"
)

// ErrConfiguration marks a profile the controller cannot act on: a name that is
// not a built-in profile, a missing FencingSLAProfile object, or timings that
// are not usable. Fencing of such an incident stays in its current phase and the
// fast eviction path is not started, because the delays it would have to respect
// are unknown.
var ErrConfiguration = errors.New("fencing SLA profile configuration error")

// Getter reads FencingSLAProfile objects.
type Getter interface {
	GetSLAProfile(ctx context.Context, name string) (*v1alpha1.FencingSLAProfile, error)
}

// Resolver turns spec.profileRef.name of an incident into the timings the
// controller acts on.
//
// The timings are resolved on the first successful reconcile of an incident and
// reused until the incident is over, so editing a FencingSLAProfile cannot move
// the deadlines of an incident that is already being processed. The snapshot
// lives in memory only: after a restart the controller resolves the incident
// again, which the ADR accepts because the profiles are built into the module
// and are not tuned at runtime.
type Resolver struct {
	getter Getter

	mu       sync.Mutex
	resolved map[string]snapshot
}

// snapshot pins timings to the object they were resolved for, so a Node whose
// object was recreated is treated as a new incident.
type snapshot struct {
	uid    types.UID
	params fsm.Params
}

func NewResolver(getter Getter) *Resolver {
	return &Resolver{getter: getter, resolved: make(map[string]snapshot)}
}

// Resolve returns the timings of the incident, reading the profile only the
// first time it is asked for a given object.
func (r *Resolver) Resolve(ctx context.Context, incident *v1alpha1.FencingFailedNodeState) (fsm.Params, error) {
	if incident == nil {
		return fsm.Params{}, errors.New("fencingfailednodestate is nil")
	}

	if params, ok := r.snapshotOf(incident); ok {
		return params, nil
	}

	params, err := r.read(ctx, incident.Spec.ProfileRef.Name)
	if err != nil {
		return fsm.Params{}, err
	}

	r.mu.Lock()
	r.resolved[incident.Name] = snapshot{uid: incident.UID, params: params}
	r.mu.Unlock()

	return params, nil
}

// Forget drops the snapshot of a Node whose incident is over.
func (r *Resolver) Forget(node string) {
	r.mu.Lock()
	delete(r.resolved, node)
	r.mu.Unlock()
}

func (r *Resolver) snapshotOf(incident *v1alpha1.FencingFailedNodeState) (fsm.Params, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	kept, ok := r.resolved[incident.Name]
	if !ok || kept.uid != incident.UID {
		return fsm.Params{}, false
	}

	return kept.params, true
}

func (r *Resolver) read(ctx context.Context, name v1alpha1.ProfileName) (fsm.Params, error) {
	if !builtin(name) {
		return fsm.Params{}, fmt.Errorf("%w: %q is not a built-in profile", ErrConfiguration, name)
	}

	objectName := name.ObjectName()

	profile, err := r.getter.GetSLAProfile(ctx, objectName)

	switch {
	case apierrors.IsNotFound(err):
		return fsm.Params{}, fmt.Errorf("%w: fencingslaprofile %q does not exist", ErrConfiguration, objectName)
	case err != nil:
		// Anything else may be transient, so it is reported as a plain error and
		// controller-runtime retries the incident with backoff.
		return fsm.Params{}, fmt.Errorf("read fencingslaprofile %q: %w", objectName, err)
	}

	params, err := timings(profile)
	if err != nil {
		return fsm.Params{}, fmt.Errorf("%w: fencingslaprofile %q: %w", ErrConfiguration, objectName, err)
	}

	return params, nil
}

// builtin reports whether the name is one of the profiles the module ships. The
// CRD enum admits only these, but an object may predate the enum.
func builtin(name v1alpha1.ProfileName) bool {
	return slices.Contains(v1alpha1.ProfileNames(), name)
}
