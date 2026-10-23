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

package useridentity

import (
	"context"
	"fmt"
	"math"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Pair is a connector and a user of it, the key of a UserIdentity.
type Pair struct {
	UserID      string
	ConnectorID string
}

// Name is the object name of the pair's UserIdentity.
func (p Pair) Name() string {
	return Name(p.UserID, p.ConnectorID)
}

// BumpGeneration raises the generation of the pair by one and returns the new value. From then on
// Dex refuses everything it issued to the pair before.
//
// An existing object gets the new generation and forgets its known devices, nothing else. A missing
// one is created with the
// claims, createdAt, and generation 1, and without the record of the copy: the transitional Dex
// keeps reading the TOTP secret and the lock of the pair from the old places, and the copy
// (identitymigration) brings them over as into any object it has not copied yet.
//
// reader must bypass the cache. The write carries the resourceVersion that was read, so a
// concurrent write by Dex or another worker makes it read and add one again.
func BumpGeneration(ctx context.Context, reader client.Reader, writer client.Writer, pair Pair, claims Claims, now time.Time) (uint64, error) {
	var generation uint64
	_, err := Mutate(ctx, reader, writer, pair.Name(), func(current *Identity) (*Identity, error) {
		if current == nil {
			id := New(pair.UserID, pair.ConnectorID)
			id.SetClaims(claims)
			if err := id.SetCreatedAt(now); err != nil {
				return nil, err
			}
			generation = 1
			id.SetGeneration(generation)
			return id, nil
		}
		if current.UserID() != pair.UserID || current.ConnectorID() != pair.ConnectorID {
			return nil, fmt.Errorf("useridentity %s belongs to another pair", pair.Name())
		}
		old, err := current.Generation()
		if err != nil {
			return nil, err
		}
		if old == math.MaxUint64 {
			return nil, fmt.Errorf("useridentity %s: the generation cannot be raised any more", pair.Name())
		}
		generation = old + 1
		current.SetGeneration(generation)
		// Everything of the pair is revoked: no browser of it is known any more, so a device signed in
		// again counts as new, whoever holds it.
		current.ForgetKnownDevices()
		return current, nil
	})
	if err != nil {
		return 0, fmt.Errorf("raise the generation of useridentity %s: %w", pair.Name(), err)
	}
	return generation, nil
}
