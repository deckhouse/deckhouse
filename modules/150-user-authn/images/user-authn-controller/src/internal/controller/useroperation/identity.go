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

package useroperation

import (
	"context"
	"fmt"
	"slices"
	"time"

	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

// In 1.78 Dex moves from OfflineSessions and Password to UserIdentity, and the old Dex keeps
// writing the old places until it is replaced. Lock, Unlock, and Reset2FA therefore write both
// places, in an order that keeps the UserIdentity copy (package identitymigration) from bringing
// back what an operation removed:
//
//   - Unlock and Reset2FA clear the old place first, then the UserIdentity. A copy that read the
//     old place before it was cleared read the UserIdentity even earlier, and its write fails on
//     the resourceVersion the operation changed.
//   - The UserIdentity write always changes the object (Record.ClearedAt), and creates it when
//     it is missing, so that such a copy always finds the resourceVersion changed.
//   - Lock writes the UserIdentity first: a copy between the two writes has nothing to undo.
//
// The UserIdentity write also copies the pair first, as the copy would (identitymigration.Copier):
// the write marks the pair as copied, and Dex stops reading the old places of a marked pair. An
// operation that runs before the copy reaches the pair would otherwise leave the TOTP secret of
// OfflineSessions and the lock of the old place behind. Unlock and Reset2FA have cleared the old
// place by then, so the copy does not bring back what they remove.

// identityPair is the pair an operation acts on and the claims a new UserIdentity gets.
type identityPair struct {
	userID      string
	connectorID string
	claims      useridentity.Claims
}

// localPair is the pair of a local user, with the claims Dex's password connector returns.
func localPair(pw passwordView) identityPair {
	userID := pw.UserID
	if userID == "" {
		userID = pw.Username
	}
	return identityPair{
		userID:      userID,
		connectorID: naming.LocalConnectorID,
		claims: useridentity.Claims{
			UserID:        userID,
			Username:      pw.Username,
			Email:         pw.Email,
			EmailVerified: true,
			Groups:        slices.Clone(pw.Groups),
		},
	}
}

// externalPair is the pair of OfflineSessions found by spec.target.
func externalPair(sess sessionView) identityPair {
	return identityPair{
		userID:      sess.UserID,
		connectorID: sess.ConnID,
		claims:      useridentity.Claims{UserID: sess.UserID, Email: sess.Email},
	}
}

// lockIdentity sets blockedUntil of the pair and marks the lock as the administrator's.
func (r *Reconciler) lockIdentity(ctx context.Context, p identityPair, until, now time.Time) error {
	return r.mutateIdentity(ctx, p, now, func(id *useridentity.Identity, rec *useridentity.Record) error {
		if err := id.SetBlockedUntil(until); err != nil {
			return err
		}
		id.SetAdminLock(until)
		// The old place gets the same value next; the copy must not take it again.
		rec.LockFrom = useridentity.FormatTime(until)
		return nil
	})
}

// unlockIdentity lifts the lock of the pair.
func (r *Reconciler) unlockIdentity(ctx context.Context, p identityPair, now time.Time) error {
	return r.mutateIdentity(ctx, p, now, func(id *useridentity.Identity, rec *useridentity.Record) error {
		if err := id.SetBlockedUntil(time.Time{}); err != nil {
			return err
		}
		id.ClearAdminLock()
		rec.ClearedAt = useridentity.FormatTime(now)
		return nil
	})
}

// resetIdentityMFA removes every second factor of the pair, as Dex's ResetMFA does.
func (r *Reconciler) resetIdentityMFA(ctx context.Context, p identityPair, now time.Time) error {
	return r.mutateIdentity(ctx, p, now, func(id *useridentity.Identity, rec *useridentity.Record) error {
		id.ClearMFA()
		rec.ClearedAt = useridentity.FormatTime(now)
		return nil
	})
}

// mutateIdentity copies the old places of p into its UserIdentity and applies change, creating
// the object when it is missing.
func (r *Reconciler) mutateIdentity(
	ctx context.Context,
	p identityPair,
	now time.Time,
	change func(*useridentity.Identity, *useridentity.Record) error,
) error {
	name := useridentity.Name(p.userID, p.connectorID)
	_, err := useridentity.Mutate(ctx, r.apiReader, r.client, name, func(current *useridentity.Identity) (*useridentity.Identity, error) {
		id, err := r.copier.Copy(ctx, name, current, now)
		if err != nil {
			return nil, fmt.Errorf("copy the old places of the pair: %w", err)
		}
		if id == nil {
			id = useridentity.New(p.userID, p.connectorID)
			id.SetClaims(p.claims)
			if err := id.SetCreatedAt(now); err != nil {
				return nil, err
			}
		} else if id.UserID() != p.userID || id.ConnectorID() != p.connectorID {
			return nil, failedf("UserIdentity %s belongs to another user", name)
		}

		rec, _, err := id.Record()
		if err != nil {
			// A value that is not a record (edited by hand) is replaced.
			rec = useridentity.Record{}
		}
		if rec.Time == "" {
			rec.Time = useridentity.FormatTime(now)
		}
		rec.AddSource(useridentity.SourceUserOperation)
		if err := change(id, &rec); err != nil {
			return nil, err
		}
		if err := id.SetRecord(rec); err != nil {
			return nil, err
		}
		return id, nil
	})
	if err != nil {
		return fmt.Errorf("write useridentity %s: %w", name, err)
	}
	return nil
}
