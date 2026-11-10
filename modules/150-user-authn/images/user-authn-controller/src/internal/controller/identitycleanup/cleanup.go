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

// Package identitycleanup deletes the UserIdentity objects of the DexProviders that are gone.
//
// A UserIdentity holds what Dex keeps about a user of a provider: the second factors, the lock, the claims and the
// generation of the pair. Nothing removed it when the provider was deleted, and the admission policy keeps people
// from deleting it by hand, since an identity deleted at the wrong moment comes back at generation 0, below the
// revocations the subscribers hold.
//
// The first pass that finds no DexProvider of the connector of an identity marks it with OrphanedSinceAnnotation; a
// pass that finds the provider again removes the mark. The cleanup deletes an identity when all of these hold:
//   - no DexProvider of that name exists;
//   - the retention has passed since the mark and since the pair was last in use (UserIdentity.LastActivity): a
//     provider deleted and created again within the retention finds the second factors and the locks of all its
//     users where they were, those who have not signed in for long included;
//   - the account is not locked: a lock ends when its time does, not when the provider goes;
//   - no Revocation of the pair is kept: the subscribers compare the generation of a new identity, 0, with it.
//
// Every token issued through the provider is long gone by then (the ID tokens run out within hours, and Dex no longer
// refreshes a token of a connector it does not have), so the deletion needs no revocation and no other state. With the
// identity go the refresh tokens, the browser sessions and the OfflineSessions of the pair. The identities of local
// users are never touched: they belong to Users. Those of a TokenExchangeProvider stay while it exists; the connector
// of a deleted or replaced one is gone like that of a deleted DexProvider, and its identities go the same way.
package identitycleanup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/revoke"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

const (
	// DefaultRetention is how long a pair of a deleted provider may be idle before its identity is deleted.
	DefaultRetention = 30 * 24 * time.Hour

	minInterval = 30 * time.Second
	maxInterval = time.Hour
)

// PairDeleter deletes what Dex keeps of a pair besides its identity.
type PairDeleter interface {
	DeletePair(ctx context.Context, userID, connectorID string) (revoke.PairDeleted, error)
}

var _ PairDeleter = (*revoke.Revoker)(nil)

// Cleanup is the leader-only runnable that makes the passes.
type Cleanup struct {
	client client.Client
	// reader reads an identity fresh before the mark is written: the cache holds it without its secrets.
	reader    client.Reader
	deleter   PairDeleter
	log       logr.Logger
	now       func() time.Time
	retention time.Duration
}

// New returns a Cleanup. reader must bypass the cache. retention <= 0 means DefaultRetention; now defaults to
// time.Now.
func New(c client.Client, reader client.Reader, deleter PairDeleter, log logr.Logger, now func() time.Time, retention time.Duration) *Cleanup {
	if now == nil {
		now = time.Now
	}
	if retention <= 0 {
		retention = DefaultRetention
	}
	return &Cleanup{client: c, reader: reader, deleter: deleter, log: log, now: now, retention: retention}
}

// Register adds the cleanup to the manager.
//
// Required RBAC: dexproviders, tokenexchangeproviders, revocations list, watch (the cache); useridentities list,
// watch, get, update (the mark) and delete; the rights of
// the revoke package for the refresh tokens, the browser sessions and the OfflineSessions.
func Register(mgr manager.Manager, retention time.Duration) error {
	log := ctrl.Log.WithName("identitycleanup")
	deleter := revoke.New(mgr.GetClient(), mgr.GetAPIReader(), log, nil)
	return mgr.Add(New(mgr.GetClient(), mgr.GetAPIReader(), deleter, log, nil, retention))
}

// NeedLeaderElection makes the cleanup run on the leader only.
func (c *Cleanup) NeedLeaderElection() bool {
	return true
}

// interval is how often a pass runs: a fraction of the retention, between 30 seconds and an hour.
func (c *Cleanup) interval() time.Duration {
	return min(max(c.retention/4, minInterval), maxInterval)
}

// Start makes a pass at once and then every interval, until ctx ends.
func (c *Cleanup) Start(ctx context.Context) error {
	for {
		if _, err := c.Run(ctx); err != nil && ctx.Err() == nil {
			c.log.Error(err, "Failed to delete the UserIdentity objects of the deleted DexProviders")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(c.interval()):
		}
	}
}

// Run makes one pass and returns how many identities it deleted.
func (c *Cleanup) Run(ctx context.Context) (int, error) {
	live, err := c.liveConnectors(ctx)
	if err != nil {
		return 0, err
	}

	identities := controller.List(controller.UserIdentityGVK)
	if err := c.client.List(ctx, identities, client.InNamespace(naming.DexNamespace)); err != nil {
		return 0, fmt.Errorf("list useridentities: %w", err)
	}

	now := c.now()
	deleted := 0
	var errs []error
	for i := range identities.Items {
		obj := &identities.Items[i]
		identity := useridentity.FromUnstructured(obj)
		connectorID := identity.ConnectorID()
		if connectorID == "" || connectorID == naming.LocalConnectorID {
			continue
		}
		since, marked := identity.OrphanedSince()
		if _, ok := live[connectorID]; ok {
			if marked {
				errs = append(errs, c.mark(ctx, obj.GetName(), time.Time{}))
			}
			continue
		}
		if !marked {
			// The retention counts from now on as well: the moment the provider went is not known otherwise.
			errs = append(errs, c.mark(ctx, obj.GetName(), now))
			continue
		}
		if now.Sub(laterOf(identity.LastActivity(), since)) < c.retention || identity.LockedAt(now) {
			continue
		}
		revoked, err := c.pairRevoked(ctx, identity.UserID(), connectorID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if revoked {
			continue
		}
		gone, err := c.delete(ctx, obj, identity)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if gone {
			deleted++
		}
	}
	return deleted, errors.Join(errs...)
}

func laterOf(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// mark sets OrphanedSinceAnnotation to at, or removes it when at is zero. The identity is read fresh and written with
// the resourceVersion read, so nothing Dex keeps in it is lost: the cache holds it without its secrets.
func (c *Cleanup) mark(ctx context.Context, name string, at time.Time) error {
	_, err := useridentity.Mutate(ctx, c.reader, c.client, name, func(current *useridentity.Identity) (*useridentity.Identity, error) {
		if current == nil {
			return nil, nil
		}
		_, marked := current.OrphanedSince()
		switch {
		case at.IsZero() && marked:
			current.ClearOrphanedSince()
		case !at.IsZero() && !marked:
			current.SetOrphanedSince(at)
		default:
			return nil, nil
		}
		return current, nil
	})
	if err != nil {
		return fmt.Errorf("mark useridentity %s: %w", name, err)
	}
	return nil
}

// pairRevoked reports whether a Revocation of the pair is kept. A new identity starts at generation 0, and the
// subscribers would refuse its tokens below the generation of the revocation until it expires.
func (c *Cleanup) pairRevoked(ctx context.Context, userID, connectorID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	list := controller.List(controller.RevocationGVK)
	if err := c.client.List(ctx, list, client.InNamespace(naming.DexNamespace),
		client.MatchingLabels{revocation.PairLabel: naming.OfflineTokenName(userID, connectorID)}); err != nil {
		return false, fmt.Errorf("list the revocations of %s: %w", naming.OfflineTokenName(userID, connectorID), err)
	}
	return len(list.Items) > 0, nil
}

// liveConnectors are the connector IDs whose identities stay: the names of the DexProviders, enabled or not, and the
// connectors of the TokenExchangeProviders.
func (c *Cleanup) liveConnectors(ctx context.Context) (map[string]struct{}, error) {
	live := map[string]struct{}{}

	providers := controller.List(controller.DexProviderGVK)
	if err := c.client.List(ctx, providers); err != nil {
		return nil, fmt.Errorf("list dexproviders: %w", err)
	}
	for i := range providers.Items {
		live[providers.Items[i].GetName()] = struct{}{}
	}

	exchanges := controller.List(controller.TokenExchangeProviderGVK)
	if err := c.client.List(ctx, exchanges); err != nil {
		return nil, fmt.Errorf("list tokenexchangeproviders: %w", err)
	}
	for i := range exchanges.Items {
		if id, _, _ := unstructured.NestedString(exchanges.Items[i].Object, "status", "connectorID"); id != "" {
			live[id] = struct{}{}
		}
	}
	return live, nil
}

// delete deletes what Dex keeps of the pair, and the identity last: a failure on the way leaves the identity, and the
// next pass starts again. The identity goes with the UID and the resourceVersion read, so one that Dex has written in
// the meantime, a login of the user, stays. It reports whether it deleted the identity: one already gone is not
// counted or logged.
func (c *Cleanup) delete(ctx context.Context, obj *unstructured.Unstructured, identity *useridentity.Identity) (bool, error) {
	userID := identity.UserID()
	connectorID := identity.ConnectorID()

	var removed revoke.PairDeleted
	if userID != "" {
		var err error
		if removed, err = c.deleter.DeletePair(ctx, userID, connectorID); err != nil {
			return false, fmt.Errorf("delete what Dex keeps of %s: %w", obj.GetName(), err)
		}
	}

	uid, resourceVersion := obj.GetUID(), obj.GetResourceVersion()
	err := c.client.Delete(ctx, obj, client.Preconditions{UID: &uid, ResourceVersion: &resourceVersion})
	switch {
	case apierrors.IsNotFound(err), apierrors.IsConflict(err):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("delete useridentity %s: %w", obj.GetName(), err)
	}
	c.log.Info("Deleted the UserIdentity of a deleted DexProvider",
		"connectorID", connectorID,
		"useridentity", obj.GetName(),
		"refreshTokensDeleted", removed.RefreshTokens,
		"authSessionsDeleted", removed.AuthSessions,
		"lastActivity", identity.LastActivity().UTC().Format(time.RFC3339),
	)
	return true, nil
}
