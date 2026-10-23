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
	"errors"
	"time"

	"user-authn-controller/internal/useridentity"
)

// errPairMismatch means the UserIdentity of the name has another userID or connectorID. Dex
// refuses such an object too, so the controller leaves it alone.
var errPairMismatch = errors.New("useridentity belongs to another pair")

// plan returns the UserIdentity the pair should have, or nil when there is nothing to write. It
// changes and returns current when the object exists.
//
// The rules, each idempotent:
//   - An empty place in the UserIdentity is filled from the old places.
//   - TOTP (local users only): the secret of OfflineSessions is copied when the UserIdentity has
//     none, a confirmation is copied, and a newer enrollment in the old place (another
//     OfflineSessions object, whose uid differs from Record.TOTPFrom) replaces an older copy. A
//     confirmed secret is never replaced by one that was never confirmed; such an enrollment
//     leaves Record.TOTPFrom as it is, so it replaces the older copy once it is confirmed. A
//     confirmed secret replaces an unconfirmed one only before the first copy: after it, an
//     unconfirmed secret in the UserIdentity is an enrollment the user started there.
//   - Lock: the later of the two locks wins; the old-place lock is taken once (Record.LockFrom).
//   - Nothing is copied back that was removed from the UserIdentity: a secret already copied from
//     the same OfflineSessions object, or a lock value already taken, is not taken again.
//   - Claims and lastLogin follow the old places while the controller owns them: it created the
//     object, or the object still has the lastLogin the controller last wrote (Record.LastLogin).
//     Once Dex logs the user in, they are Dex's.
func plan(current *useridentity.Identity, p pair, now time.Time) (*useridentity.Identity, error) {
	creating := current == nil
	id := current
	if creating {
		if !hasDataToCreate(p, now) {
			return nil, nil
		}
		id = useridentity.New(p.UserID, p.ConnID)
	} else if id.UserID() != p.UserID || id.ConnectorID() != p.ConnID {
		return nil, errPairMismatch
	}

	rec, hasRecord, err := id.Record()
	if err != nil {
		// A value that is not a record (edited by hand) protects nothing; it is replaced.
		rec, hasRecord = useridentity.Record{}, false
	}
	lastLogin, err := id.LastLogin()
	if err != nil {
		return nil, err
	}
	owned := creating || (hasRecord && rec.LastLogin == formatOptional(lastLogin))

	if err := planTOTP(id, &rec, p); err != nil {
		return nil, err
	}
	if err := planLock(id, &rec, p, now); err != nil {
		return nil, err
	}
	if err := planClaims(id, &rec, p, owned); err != nil {
		return nil, err
	}
	if err := planCreatedAt(id, p, now); err != nil {
		return nil, err
	}

	if rec.Time == "" {
		rec.Time = useridentity.FormatTime(now)
	}
	if p.Session != nil {
		rec.AddSource(useridentity.SourceOfflineSessions)
	}
	if p.Password != nil {
		rec.AddSource(useridentity.SourcePassword)
	}
	if err := id.SetRecord(rec); err != nil {
		return nil, err
	}
	return id, nil
}

// hasDataToCreate tells whether the pair needs a UserIdentity. Everyone who has logged in has an
// OfflineSessions. A local user locked before ever logging in has only a Password; a local user
// without either has nothing to copy, and Dex creates the object at the first login.
func hasDataToCreate(p pair, now time.Time) bool {
	if p.Session != nil {
		return true
	}
	until, _ := p.oldLock()
	return until != nil && until.After(now)
}

func planTOTP(id *useridentity.Identity, rec *useridentity.Record, p pair) error {
	s := p.Session
	if !p.isLocal() || s == nil || s.TOTP == "" {
		return nil
	}
	current, err := id.TOTP()
	if err != nil {
		return err
	}

	fromSource := useridentity.MFASecret{Secret: s.TOTP, Confirmed: s.TOTPConfirmed, CreatedAt: s.CreatedAt}
	neverCopied := rec.TOTPFrom == ""
	olderCopy := !neverCopied && rec.TOTPFrom != s.UID
	switch {
	case current == nil:
		if rec.TOTPFrom != s.UID {
			id.SetTOTP(fromSource)
		}
	case current.Secret == s.TOTP:
		if s.TOTPConfirmed && !current.Confirmed {
			id.ConfirmTOTP()
		}
	case current.Confirmed && !s.TOTPConfirmed:
		// A confirmed second factor is never replaced with one that was never confirmed. The
		// enrollment is held back, not passed over: TOTPFrom stays, so once the user confirms
		// it, it is a newer enrollment and replaces an older copy.
		return nil
	case olderCopy, neverCopied && s.TOTPConfirmed && !current.Confirmed:
		// A secret enrolled in the UserIdentity after the copy is the user's own: the copied
		// secret does not come back over it, even when that enrollment is not confirmed yet.
		id.SetTOTP(fromSource)
	}
	rec.TOTPFrom = s.UID
	return nil
}

func planLock(id *useridentity.Identity, rec *useridentity.Record, p pair, now time.Time) error {
	until, admin := p.oldLock()
	if until == nil || !until.After(now) {
		return nil
	}
	from := useridentity.FormatTime(*until)
	if rec.LockFrom == from {
		return nil
	}
	blocked, err := id.BlockedUntil()
	if err != nil {
		return err
	}
	if !until.Before(blocked) {
		if err := id.SetBlockedUntil(*until); err != nil {
			return err
		}
		switch {
		case admin:
			id.SetAdminLock(*until)
		case until.After(blocked):
			id.ClearAdminLock()
		}
	}
	rec.LockFrom = from
	return nil
}

func planClaims(id *useridentity.Identity, rec *useridentity.Record, p pair, owned bool) error {
	current, err := id.Claims()
	if err != nil {
		return err
	}
	if owned || current.IsEmpty() {
		if desired := p.claims(current); !current.Equal(desired) {
			id.SetClaims(desired)
		}
	}
	if !owned {
		return nil
	}

	lastLogin, err := id.LastLogin()
	if err != nil {
		return err
	}
	if fromOld := p.lastLogin(); fromOld.After(lastLogin) {
		if err := id.SetLastLogin(fromOld); err != nil {
			return err
		}
		lastLogin = fromOld
	}
	rec.LastLogin = formatOptional(lastLogin)
	return nil
}

func planCreatedAt(id *useridentity.Identity, p pair, now time.Time) error {
	created, err := id.CreatedAt()
	if err != nil || !created.IsZero() {
		return err
	}
	if p.Session != nil && !p.Session.CreatedAt.IsZero() {
		return id.SetCreatedAt(p.Session.CreatedAt)
	}
	return id.SetCreatedAt(now)
}

func formatOptional(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return useridentity.FormatTime(t)
}
