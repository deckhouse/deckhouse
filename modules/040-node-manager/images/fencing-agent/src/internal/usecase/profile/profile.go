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

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// ChecksumAnnotation holds the checksum of the profile spec. Helm renders the
// same value into the agent ConfigMap.
const ChecksumAnnotation = "node-manager.deckhouse.io/fencing-profile-checksum"

// getTimeout limits one GET, so a stuck connection does not stop the caller.
const getTimeout = 5 * time.Second

// ErrInvalid marks a profile that fails the validation. Other errors of Fetch
// are transient: a later call can succeed without a change of the ConfigMap.
var ErrInvalid = errors.New("invalid profile")

type Getter interface {
	GetSLAProfile(ctx context.Context, name string) (*v1alpha1.FencingSLAProfile, error)
}

// Fetch gets the profile and returns its spec. The checksum annotation must be
// equal to checksum, else the answer is older than the ConfigMap. The profile
// must pass Validate.
func Fetch(ctx context.Context, getter Getter, name, checksum string) (v1alpha1.FencingSLAProfileSpec, error) {
	ctx, cancel := context.WithTimeout(ctx, getTimeout)
	defer cancel()

	p, err := getter.GetSLAProfile(ctx, name)
	if err != nil {
		return v1alpha1.FencingSLAProfileSpec{}, err
	}

	if checksum == "" {
		return v1alpha1.FencingSLAProfileSpec{}, fmt.Errorf("profile %q: the ConfigMap has no checksum", name)
	}

	if got := p.Annotations[ChecksumAnnotation]; got != checksum {
		return v1alpha1.FencingSLAProfileSpec{}, fmt.Errorf("profile %q: checksum %q is not %q from the ConfigMap", name, got, checksum)
	}

	if err := Validate(p); err != nil {
		return v1alpha1.FencingSLAProfileSpec{}, fmt.Errorf("profile %q: %w: %w", name, ErrInvalid, err)
	}

	return p.Spec, nil
}
