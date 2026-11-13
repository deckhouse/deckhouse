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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/domain/fsm"
)

// ErrConfiguration marks a profile that the controller cannot use: a missing
// FencingSLAProfile object. Such an incident keeps its phase, because its
// deadlines are not known.
var ErrConfiguration = errors.New("fencing SLA profile configuration error")

// unarmedHeartbeatTTL is how long the incident of an unarmed node stays in
// FallbackAlive after the controller sees a heartbeat. It is the heartbeat TTL
// of the slow profile, the longest of the built-in profiles.
const unarmedHeartbeatTTL = 20 * time.Second

// Getter reads FencingSLAProfile objects.
type Getter interface {
	GetSLAProfile(ctx context.Context, name string) (*v1alpha1.FencingSLAProfile, error)
}

// Resolver gives the timings of an incident. An armed node uses the profile in
// spec.profileRef.name. The module ships the profiles. An unarmed node has no
// spec.profileRef. The controller never proves its failure, so it needs only
// the heartbeat TTL, and the TTL is the same for each unarmed node.
type Resolver struct {
	getter Getter
}

func NewResolver(getter Getter) *Resolver {
	return &Resolver{getter: getter}
}

// Resolve returns the timings of the incident. On an error, Params.Profile is
// the name of the profile.
func (r *Resolver) Resolve(ctx context.Context, incident *v1alpha1.FencingFailedNodeState) (fsm.Params, error) {
	if incident == nil {
		return fsm.Params{}, errors.New("fencingfailednodestate is nil")
	}

	if incident.Spec.ProfileRef == nil {
		return fsm.Params{HeartbeatTTL: unarmedHeartbeatTTL}, nil
	}

	name := incident.Spec.ProfileRef.Name

	params, err := r.read(ctx, name)
	if err != nil {
		// The name goes with the error, so the caller can report the profile.
		return fsm.Params{Profile: name}, err
	}

	params.Armed = incident.Spec.Armed

	return params, nil
}

// read returns the timings of the profile.
func (r *Resolver) read(ctx context.Context, name v1alpha1.ProfileName) (fsm.Params, error) {
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

	params := timings(profile)
	params.Profile = name

	return params, nil
}
