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

package useraccount

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

type desiredAccount struct {
	Name   string
	Labels map[string]string
	Owner  *metav1.OwnerReference
	Status v1alpha1.UserAccountStatus
}

// lockState is a lock in one place and whether an administrator set it.
type lockState struct {
	until   *time.Time
	byAdmin bool
}

// enforcedLock is the lock Dex enforces on a pair: the one in its UserIdentity. Until the
// controller marks the UserIdentity as migrated, the transitional Dex patch also reads the lock of
// the old place (Password or OfflineSessions), and the later of the two is in force.
func enforcedLock(old lockState, ident *identityView) lockState {
	if ident != nil && ident.Migrated {
		return identityLock(ident)
	}
	return laterLock(old, identityLock(ident))
}

// laterLock is the lock that ends later.
func laterLock(a, b lockState) lockState {
	switch {
	case a.until == nil:
		return b
	case b.until == nil:
		return a
	case a.until.After(*b.until):
		return a
	case b.until.After(*a.until):
		return b
	default:
		return lockState{until: a.until, byAdmin: a.byAdmin || b.byAdmin}
	}
}

func identityLock(ident *identityView) lockState {
	if ident == nil {
		return lockState{}
	}
	return lockState{until: ident.BlockedUntil, byAdmin: ident.LockedByAdmin}
}

// externalSource is what is known of an external account: its UserIdentity, its OfflineSessions,
// or both. OfflineSessions keeps the account in place until the copy creates the UserIdentity.
type externalSource struct {
	UserID    string
	ConnID    string
	Email     string
	Attempts  int64
	Lock      lockState
	LastLogin *time.Time
	TwoFactor *v1alpha1.UserAccountTwoFactor
}

func newExternalSource(sess *sessionView, ident *identityView) externalSource {
	var src externalSource
	var oldLock lockState
	if sess != nil {
		src = externalSource{
			UserID:   sess.UserID,
			ConnID:   sess.ConnID,
			Email:    sess.Email,
			Attempts: sess.Attempts,
		}
		oldLock = lockState{until: sess.LockedUntil, byAdmin: hasAdminLock(sess.Annotations)}
	}
	src.Lock = enforcedLock(oldLock, ident)
	if ident == nil {
		return src
	}
	src.UserID, src.ConnID = ident.UserID, ident.ConnectorID
	// The claims of the last login; OfflineSessions.email is what the Dex of 1.77 wrote.
	if ident.Email != "" {
		src.Email = ident.Email
	}
	// Dex counts failed attempts of external users in UserIdentity; the counter of OfflineSessions
	// is the one of the Dex of 1.77 and matters only until the pair is migrated.
	if ident.Migrated || ident.Attempts > src.Attempts {
		src.Attempts = ident.Attempts
	}
	src.LastLogin = ident.LastLogin
	twoFactor := ident.TwoFactor
	src.TwoFactor = &twoFactor
	return src
}

func projectLocal(pw passwordView, ident *identityView, user *userView, now time.Time) desiredAccount {
	lock := enforcedLock(lockState{until: pw.LockedUntil, byAdmin: hasAdminLock(pw.Annotations)}, ident)
	locked := timeLockActive(lock.until, now)
	lockedByAdmin := locked && lock.byAdmin

	groups := slices.Clone(pw.Groups)
	userRef := ""
	var expireAt *metav1.Time
	var owner *metav1.OwnerReference
	if user != nil {
		userRef = user.Name
		if user.Groups != nil {
			groups = slices.Clone(user.Groups)
		}
		expireAt = metaTimePtr(user.ExpireAt)
		owner = &metav1.OwnerReference{
			APIVersion:         controller.UserGVK.GroupVersion().String(),
			Kind:               controller.UserGVK.Kind,
			Name:               user.Name,
			UID:                user.UID,
			Controller:         boolPtr(true),
			BlockOwnerDeletion: boolPtr(false),
		}
	}

	return desiredAccount{
		Name: naming.LocalName(localNameInput(pw.Email, pw.Username)),
		Labels: map[string]string{
			v1alpha1.LabelKind:        v1alpha1.KindLocal,
			v1alpha1.LabelConnectorID: naming.LocalConnectorID,
			v1alpha1.LabelLocked:      strconv.FormatBool(locked),
		},
		Owner: owner,
		Status: v1alpha1.UserAccountStatus{
			Email:                  pw.Email,
			Username:               pw.Username,
			UserID:                 pw.UserID,
			Kind:                   v1alpha1.KindLocal,
			ConnectorID:            naming.LocalConnectorID,
			IncorrectLoginAttempts: pw.Attempts,
			Locked:                 locked,
			LockedUntil:            metaTimePtr(lock.until),
			LockedByAdministrator:  lockedByAdmin,
			UserRef:                userRef,
			ExpireAt:               expireAt,
			Groups:                 groups,
			LastLogin:              identityLastLogin(ident),
			TwoFactor:              identityTwoFactor(ident),
		},
	}
}

func identityLastLogin(ident *identityView) *metav1.Time {
	if ident == nil {
		return nil
	}
	return metaTimePtr(ident.LastLogin)
}

func identityTwoFactor(ident *identityView) *v1alpha1.UserAccountTwoFactor {
	if ident == nil {
		return nil
	}
	twoFactor := ident.TwoFactor
	return &twoFactor
}

func projectExternal(src externalSource, providerType string, now time.Time) desiredAccount {
	locked := timeLockActive(src.Lock.until, now)
	lockedByAdmin := locked && src.Lock.byAdmin

	return desiredAccount{
		Name: naming.ExternalName(src.ConnID, src.UserID),
		Labels: map[string]string{
			v1alpha1.LabelKind:        v1alpha1.KindExternal,
			v1alpha1.LabelConnectorID: src.ConnID,
			v1alpha1.LabelLocked:      strconv.FormatBool(locked),
		},
		Status: v1alpha1.UserAccountStatus{
			Email:                  src.Email,
			UserID:                 src.UserID,
			Kind:                   v1alpha1.KindExternal,
			ConnectorID:            src.ConnID,
			ProviderType:           providerType,
			IncorrectLoginAttempts: src.Attempts,
			Locked:                 locked,
			LockedUntil:            metaTimePtr(src.Lock.until),
			LockedByAdministrator:  lockedByAdmin,
			LastLogin:              metaTimePtr(src.LastLogin),
			TwoFactor:              src.TwoFactor,
		},
	}
}

const (
	// maxUserAgent and maxIPAddress bound what a device puts into the status: the User-Agent is the
	// browser's own header and could otherwise grow the object by kilobytes per device.
	maxUserAgent = 512
	maxIPAddress = 64

	// DefaultEstablishedDeviceAge is how long a device must have been signed in to act on the other
	// devices of the user through user-api (its minimum device age), unless configured.
	DefaultEstablishedDeviceAge = 24 * time.Hour
)

// device is a device of a pair while it is projected: lastActivity is exact, for ordering.
type device struct {
	session      v1alpha1.UserAccountSession
	lastActivity time.Time
}

// projectSessions lists the devices of a pair: each live browser session with the applications
// signed in through it, and each grant not issued under a live browser session. A grant issued under
// a session that has ended is a device of its own: the application keeps it until logout or
// revocation. The most recently active come first, at most v1alpha1.MaxSessions of them. A device
// whose browser completed a login of the pair before its sign-in (known, by the hash of the device
// mark) is known since then.
func projectSessions(browsers []browserView, grants []grantView, known map[string]time.Time, establishedAge time.Duration, now time.Time) ([]v1alpha1.UserAccountSession, bool) {
	devices := make(map[string]*device, len(browsers)+len(grants))
	for _, b := range browsers {
		if !b.Expiry.IsZero() && !b.Expiry.After(now) {
			continue
		}
		devices[b.ID] = &device{
			session: v1alpha1.UserAccountSession{
				ID:         b.ID,
				Kind:       v1alpha1.SessionKindBrowser,
				Clients:    slices.Clone(b.Clients),
				IPAddress:  truncate(b.IPAddress, maxIPAddress),
				UserAgent:  truncate(b.UserAgent, maxUserAgent),
				SignedInAt: metaTimeOrNil(b.CreatedAt),
				KnownSince: knownSince(known, b.DeviceHash, b.CreatedAt),
			},
			lastActivity: b.LastActivity,
		}
	}
	for _, g := range grants {
		if d, ok := devices[g.SessionID]; ok && g.SessionID != "" && d.session.Kind == v1alpha1.SessionKindBrowser {
			if g.ClientID != "" && !slices.Contains(d.session.Clients, g.ClientID) {
				d.session.Clients = append(d.session.Clients, g.ClientID)
				slices.Sort(d.session.Clients)
			}
			d.lastActivity = laterTime(d.lastActivity, g.LastUsed)
			continue
		}
		var clients []string
		if g.ClientID != "" {
			clients = []string{g.ClientID}
		}
		devices[g.ID] = &device{
			session: v1alpha1.UserAccountSession{
				ID:         g.ID,
				Kind:       v1alpha1.SessionKindGrant,
				Clients:    clients,
				IPAddress:  truncate(g.IPAddress, maxIPAddress),
				UserAgent:  truncate(g.UserAgent, maxUserAgent),
				SignedInAt: metaTimeOrNil(g.CreatedAt),
				KnownSince: knownSince(known, g.DeviceHash, g.CreatedAt),
			},
			lastActivity: laterTime(g.CreatedAt, g.LastUsed),
		}
	}

	ordered := slices.Collect(maps.Values(devices))
	for _, d := range ordered {
		// Rounded down to the hour, and ordered by the rounded time: the status is rewritten at most once
		// an hour, not whenever two devices used within the same hour take turns. A device signed in within
		// the hour keeps its sign-in time: its activity cannot come before it, and the sign-in does not move.
		d.lastActivity = d.lastActivity.Truncate(time.Hour)
		if signedIn := d.session.SignedInAt; signedIn != nil && d.lastActivity.Before(signedIn.Time) {
			d.lastActivity = signedIn.Time
		}
	}
	byActivity := func(a, b *device) int {
		if c := b.lastActivity.Compare(a.lastActivity); c != 0 {
			return c
		}
		return strings.Compare(a.session.ID, b.session.ID)
	}
	slices.SortFunc(ordered, byActivity)
	truncated := len(ordered) > v1alpha1.MaxSessions
	if truncated {
		// The devices signed in long enough to act on the others are kept first: a flood of new sign-ins
		// must not push the owner's device off the list, where user-api would no longer find it.
		established := func(d *device) bool {
			since := d.session.SignedInAt
			if d.session.KnownSince != nil {
				since = d.session.KnownSince
			}
			return since != nil && now.Sub(since.Time) >= establishedAge
		}
		slices.SortStableFunc(ordered, func(a, b *device) int {
			if ea, eb := established(a), established(b); ea != eb {
				if ea {
					return -1
				}
				return 1
			}
			return 0
		})
		ordered = ordered[:v1alpha1.MaxSessions]
		slices.SortFunc(ordered, byActivity)
	}
	if len(ordered) == 0 {
		return nil, false
	}
	out := make([]v1alpha1.UserAccountSession, 0, len(ordered))
	for _, d := range ordered {
		d.session.LastActivity = metaTimeOrNil(d.lastActivity)
		out = append(out, d.session)
	}
	return out, truncated
}

// knownSinceMargin is how much earlier than a sign-in its browser must have been first seen for the
// device to be listed as known since then. The first login of a browser is remembered at its first
// factor, a moment before its browser session and seconds or, with a second factor to set up, minutes
// before its grant: that is the same sign-in, not an earlier one.
const knownSinceMargin = time.Minute

// knownSince is when the browser hash was first seen completing a login of the pair, when that is more
// than knownSinceMargin before signedIn: the browser signed in before this sign-in. Nil otherwise.
func knownSince(known map[string]time.Time, hash string, signedIn time.Time) *metav1.Time {
	firstSeen, ok := known[hash]
	if hash == "" || !ok || signedIn.IsZero() || !firstSeen.Before(signedIn.Add(-knownSinceMargin)) {
		return nil
	}
	return metaTimeOrNil(firstSeen)
}

func laterTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

func metaTimeOrNil(t time.Time) *metav1.Time {
	if t.IsZero() {
		return nil
	}
	return metaTimePtr(&t)
}

// truncate cuts s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func localNameInput(email, username string) string {
	if email != "" {
		return email
	}
	return username
}

func timeLockActive(until *time.Time, now time.Time) bool {
	return until != nil && until.After(now)
}

func metaTimePtr(t *time.Time) *metav1.Time {
	if t == nil {
		return nil
	}
	mt := metav1.NewTime(t.UTC())
	return &mt
}

func boolPtr(v bool) *bool {
	return &v
}

func matchUserByEmail(users []userView, email string) *userView {
	if email == "" {
		return nil
	}
	for i := range users {
		if strings.EqualFold(users[i].Email, email) {
			return &users[i]
		}
	}
	return nil
}

func isProjectablePassword(pw passwordView) bool {
	return pw.Email != "" || pw.Username != ""
}

// isExternalConnector tells whether accounts of connID are external: every connector but local.
func isExternalConnector(connID string) bool {
	return connID != "" && !strings.EqualFold(connID, naming.LocalConnectorID)
}
