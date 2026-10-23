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

package identitymigration

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/useridentity"
)

// Copier applies the copy of a pair to a UserIdentity that another writer writes.
//
// Every write of the controller leaves the annotation user-authn.deckhouse.io/migrated on the
// object, and Dex stops reading the old places for a pair whose UserIdentity has it. A UserOperation
// that runs before the copy reaches the pair therefore applies the copy first, in the same write:
// otherwise the pair would count as copied without its TOTP secret and old-place lock.
type Copier struct {
	places oldPlaces
}

// NewCopier returns a Copier. c serves cached reads (the Password index, refresh token claims);
// apiReader must bypass the cache, which strips the TOTP secret.
func NewCopier(c, apiReader client.Reader, log logr.Logger) *Copier {
	return &Copier{places: oldPlaces{client: c, apiReader: apiReader, log: log}}
}

// Copy returns the UserIdentity name with the old places of its pair copied in, by the rules the
// controller copies with: current changed, a new object when current is nil and the old places
// hold something to copy, or current as it is when they hold nothing or the pair is not to be
// copied. current is the object as the caller's useridentity.Mutate read it, or nil. Copy must run
// inside that Mutate, so that the old places it reads are not older than current.
func (c *Copier) Copy(ctx context.Context, name string, current *useridentity.Identity, now time.Time) (*useridentity.Identity, error) {
	p, reason, err := c.places.readPair(ctx, name)
	if err != nil {
		return nil, err
	}
	if reason != "" || p.UserID == "" {
		return current, nil
	}

	desired, err := plan(current, p, now)
	switch {
	case errors.Is(err, errPairMismatch):
		return current, nil
	case err != nil:
		return nil, err
	case desired == nil:
		return current, nil
	}
	return desired, nil
}
