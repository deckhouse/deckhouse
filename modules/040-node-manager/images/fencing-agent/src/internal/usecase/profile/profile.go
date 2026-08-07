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
	"net"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/deckhouse/deckhouse/pkg/log"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

// retryInterval paces retries of transient API failures inside the caller's
// budget; deterministic failures (NotFound, invalid values) end immediately.
const retryInterval = 2 * time.Second

type Getter interface {
	GetSLAProfile(ctx context.Context, name string) (*v1alpha1.FencingSLAProfile, error)
}

// Load fetches the profile by ProfileName.ObjectName and validates it,
// retrying only transient API errors until ctx expires.
func Load(ctx context.Context, getter Getter, name v1alpha1.ProfileName, logger *log.Logger) (v1alpha1.FencingSLAProfileSpec, error) {
	return load(ctx, getter, name, logger, retryInterval)
}

func load(ctx context.Context, getter Getter, name v1alpha1.ProfileName, logger *log.Logger, retryIn time.Duration) (v1alpha1.FencingSLAProfileSpec, error) {
	objectName := name.ObjectName()

	for {
		p, err := getter.GetSLAProfile(ctx, objectName)

		switch {
		case err == nil:
			if validateErr := Validate(p); validateErr != nil {
				return v1alpha1.FencingSLAProfileSpec{}, fmt.Errorf("SLA profile %q is invalid: %w", objectName, validateErr)
			}

			return p.Spec, nil
		case apierrors.IsNotFound(err):
			return v1alpha1.FencingSLAProfileSpec{}, fmt.Errorf("SLA profile %q not found: %w", objectName, err)
		case !isTransient(err):
			return v1alpha1.FencingSLAProfileSpec{}, fmt.Errorf("SLA profile %q is invalid: %w", objectName, err)
		}

		logger.Warn("get SLA profile failed, retrying",
			"profile", objectName,
			"error", err,
			"retry_interval", retryIn.String(),
		)

		select {
		case <-ctx.Done():
			return v1alpha1.FencingSLAProfileSpec{}, fmt.Errorf("get SLA profile %q: %w", objectName, err)
		case <-time.After(retryIn):
		}
	}
}

func isTransient(err error) bool {
	if apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) {
		return false
	}

	var status apierrors.APIStatus
	if errors.As(err, &status) {
		return true
	}

	var netErr net.Error

	return errors.As(err, &netErr)
}
