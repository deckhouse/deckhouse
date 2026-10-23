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

package revoke

import (
	"context"
	"errors"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

// The logouts narrowed down to one client, one browser session, or every session but one. They keep
// the generation of the pair: the rest of what the pair was issued stays valid.

// ErrSessionNotFound means the browser session does not exist or belongs to another pair. The two
// are not told apart, so that an operation cannot find out the sessions of other users.
var ErrSessionNotFound = errors.New("session not found")

// ErrGrantNotFound means the grant does not exist or belongs to another pair, told apart no more
// than sessions are.
var ErrGrantNotFound = errors.New("grant not found")

// LogoutSession ends one browser session of the pair and the grants issued under it. It returns
// ErrSessionNotFound when the session does not exist or is not the pair's.
func (r *Revoker) LogoutSession(ctx context.Context, p Pair, sid, opUID string) (Result, error) {
	res := Result{}
	s, err := r.sessionOf(ctx, p, sid)
	if err != nil {
		return res, err
	}
	refs, err := r.offlineRefs(ctx, p)
	if err != nil {
		return res, err
	}
	ended, err := r.endSession(ctx, p, s, refs, revocation.ReasonLogoutSession, opUID, &res)
	if err != nil {
		return res, err
	}
	if err := r.forgetDevices(ctx, p, append(refHashes(refs, ended), s.deviceHash), s.name); err != nil {
		return res, err
	}

	r.log.Info("Ended a browser session of the pair",
		"connectorID", p.ConnectorID,
		"userID", p.UserID,
		"session", sid,
		"refreshTokensDeleted", res.RefreshTokensDeleted,
	)
	return res, nil
}

// LogoutExceptSession ends every browser session of the pair except keep, and revokes every grant
// of the pair that was not issued under keep. The generation does not change, so this is best
// effort: a grant issued while the operation runs survives it. It returns ErrSessionNotFound when
// keep does not exist or is not the pair's.
func (r *Revoker) LogoutExceptSession(ctx context.Context, p Pair, keep, opUID string) (Result, error) {
	pairName := p.name()
	res := Result{}
	kept, err := r.sessionOf(ctx, p, keep)
	if err != nil {
		return res, err
	}

	refs, err := r.offlineRefs(ctx, p)
	if err != nil {
		return res, err
	}
	sessions, err := r.sessionsOf(ctx, pairName)
	if err != nil {
		return res, err
	}
	done := map[string]struct{}{}
	for _, s := range sessions {
		if s.name == keep {
			continue
		}
		ended, err := r.endSession(ctx, p, s, refs, revocation.ReasonLogoutExceptSession, opUID, &res)
		if err != nil {
			return res, err
		}
		for _, id := range ended {
			done[id] = struct{}{}
		}
	}

	tokens, err := r.tokensOf(ctx, pairName)
	if err != nil {
		return res, err
	}
	grants := map[string]grant{}
	for _, t := range tokens {
		grants[t.name] = grant{id: t.name, clientID: t.clientID}
	}
	for _, rf := range refs {
		grants[rf.id] = grant{id: rf.id, clientID: rf.clientID, ref: &rf}
	}
	for _, id := range sortedKeys(grants) {
		g := grants[id]
		if _, ok := done[id]; ok || (g.ref != nil && g.sessionID() == keep) {
			continue
		}
		if err := r.revokeGrant(ctx, p, g, revocation.ReasonLogoutExceptSession, opUID, &res); err != nil {
			return res, err
		}
	}
	if err := r.forgetOtherDevices(ctx, p, kept.deviceHash); err != nil {
		return res, err
	}

	r.log.Info("Ended every browser session of the pair but one",
		"connectorID", p.ConnectorID,
		"userID", p.UserID,
		"keptSession", keep,
		"refreshTokensDeleted", res.RefreshTokensDeleted,
		"authSessionsDeleted", res.AuthSessionsDeleted,
	)
	return res, nil
}

// LogoutGrant revokes one grant of the pair: a device signed in without a browser session, such as
// a kubeconfig, or an application signed in while the sessions of Dex are off. The generation and
// the other grants stay. It returns ErrGrantNotFound when the grant is not the pair's.
func (r *Revoker) LogoutGrant(ctx context.Context, p Pair, grantID, opUID string) (Result, error) {
	res := Result{}
	grants, err := r.grantsOf(ctx, p)
	if err != nil {
		return res, err
	}
	g, ok := grants[grantID]
	if !ok {
		return res, ErrGrantNotFound
	}
	if err := r.revokeGrant(ctx, p, g, revocation.ReasonLogoutGrant, opUID, &res); err != nil {
		return res, err
	}
	if err := r.forgetGrantClient(ctx, g); err != nil {
		return res, err
	}
	if err := r.forgetDevices(ctx, p, []string{g.deviceHash()}, ""); err != nil {
		return res, err
	}

	r.log.Info("Revoked a grant of the pair",
		"connectorID", p.ConnectorID,
		"userID", p.UserID,
		"grant", grantID,
		"refreshTokensDeleted", res.RefreshTokensDeleted,
	)
	return res, nil
}

// LogoutExceptGrant ends every browser session of the pair and revokes every grant but keep: signing
// out on the other devices from a device that has no browser session of Dex. A session that keep was
// issued under stays too, the other grants of it go. The generation does not change, so this is best
// effort, as LogoutExceptSession is. It returns ErrGrantNotFound when keep is not the pair's.
func (r *Revoker) LogoutExceptGrant(ctx context.Context, p Pair, keep, opUID string) (Result, error) {
	pairName := p.name()
	res := Result{}
	grants, err := r.grantsOf(ctx, p)
	if err != nil {
		return res, err
	}
	kept, ok := grants[keep]
	if !ok {
		return res, ErrGrantNotFound
	}

	refs, err := r.offlineRefs(ctx, p)
	if err != nil {
		return res, err
	}
	// The grants of the sessions ended below go with them; keep is no grant of those.
	delete(refs, keep)
	sessions, err := r.sessionsOf(ctx, pairName)
	if err != nil {
		return res, err
	}
	done := map[string]struct{}{}
	for _, s := range sessions {
		if kept.sessionID() != "" && s.name == kept.sessionID() {
			continue
		}
		ended, err := r.endSession(ctx, p, s, refs, revocation.ReasonLogoutExceptGrant, opUID, &res)
		if err != nil {
			return res, err
		}
		for _, id := range ended {
			done[id] = struct{}{}
		}
	}
	for _, id := range sortedKeys(grants) {
		if _, ok := done[id]; ok || id == keep {
			continue
		}
		if err := r.revokeGrant(ctx, p, grants[id], revocation.ReasonLogoutExceptGrant, opUID, &res); err != nil {
			return res, err
		}
		if err := r.forgetGrantClient(ctx, grants[id]); err != nil {
			return res, err
		}
	}
	if err := r.forgetOtherDevices(ctx, p, kept.deviceHash()); err != nil {
		return res, err
	}

	r.log.Info("Revoked every grant of the pair but one",
		"connectorID", p.ConnectorID,
		"userID", p.UserID,
		"keptGrant", keep,
		"refreshTokensDeleted", res.RefreshTokensDeleted,
		"authSessionsDeleted", res.AuthSessionsDeleted,
	)
	return res, nil
}

// grantsOf returns the grants of the pair by ID: its refresh token objects and the references of
// its OfflineSessions, a reference carrying the session the grant was issued under.
func (r *Revoker) grantsOf(ctx context.Context, p Pair) (map[string]grant, error) {
	tokens, err := r.tokensOf(ctx, p.name())
	if err != nil {
		return nil, err
	}
	refs, err := r.offlineRefs(ctx, p)
	if err != nil {
		return nil, err
	}
	grants := make(map[string]grant, len(tokens)+len(refs))
	for _, t := range tokens {
		grants[t.name] = grant{id: t.name, clientID: t.clientID}
	}
	for _, rf := range refs {
		grants[rf.id] = grant{id: rf.id, clientID: rf.clientID, ref: &rf}
	}
	return grants, nil
}

// grant is a refresh token of the pair: an object, a reference of OfflineSessions (ref), or both.
type grant struct {
	id       string
	clientID string
	ref      *ref
}

// deviceHash is the hash of the device mark of the browser of the grant, "" when unknown or none.
func (g grant) deviceHash() string {
	if g.ref == nil {
		return ""
	}
	return g.ref.deviceHash
}

// refHashes returns the device hashes of the references ids of refs.
func refHashes(refs map[string]ref, ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, refs[id].deviceHash)
	}
	return out
}

// forgetDevices removes the browsers of the devices an operation ended from the known devices of
// the pair: a device ended by someone else, an administrator or another device, signs in as a new
// device next time, so a thief signed out by the owner waits the full 24 hours again after signing
// in with the stolen password.
//
// A browser that still holds a live sign-in of the pair stays known: it is forgotten with its last
// sign-in. Otherwise ending an old grant of the browser the user works in, a console token left from
// an earlier session of it, would make that very browser new. endedSession is the session the
// operation ended, which the cache may still hold.
func (r *Revoker) forgetDevices(ctx context.Context, p Pair, hashes []string, endedSession string) error {
	held, err := r.heldBrowsers(ctx, p, endedSession)
	if err != nil {
		return err
	}
	hashes = slices.DeleteFunc(slices.Clone(hashes), func(h string) bool {
		_, ok := held[h]
		return ok
	})
	return useridentity.ForgetDevices(ctx, r.apiReader, r.client,
		useridentity.Pair{UserID: p.UserID, ConnectorID: p.ConnectorID}, hashes)
}

// heldBrowsers returns the browsers, by the hash of their device mark, that hold a live sign-in of
// the pair: a session other than endedSession that has not expired, or a grant. The references of
// the grants are read fresh, so the grants the operation has revoked are not among them.
func (r *Revoker) heldBrowsers(ctx context.Context, p Pair, endedSession string) (map[string]struct{}, error) {
	sessions, err := r.sessionsOf(ctx, p.name())
	if err != nil {
		return nil, err
	}
	refs, err := r.offlineRefs(ctx, p)
	if err != nil {
		return nil, err
	}
	now := r.now()
	held := make(map[string]struct{}, len(sessions)+len(refs))
	for _, s := range sessions {
		if s.name == endedSession || s.deviceHash == "" || s.expired(now) {
			continue
		}
		held[s.deviceHash] = struct{}{}
	}
	for _, rf := range refs {
		if rf.deviceHash != "" {
			held[rf.deviceHash] = struct{}{}
		}
	}
	return held, nil
}

// forgetOtherDevices removes every browser but keep, the browser of the device the user signs out
// of the others from, from the known devices of the pair: the browsers signed out now, and the
// browsers known from earlier sign-ins that are not signed in, so none of them counts as known when
// it signs in again.
func (r *Revoker) forgetOtherDevices(ctx context.Context, p Pair, keep string) error {
	return useridentity.ForgetOtherDevices(ctx, r.apiReader, r.client,
		useridentity.Pair{UserID: p.UserID, ConnectorID: p.ConnectorID}, keep)
}

// sessionID is the browser session the grant was issued under, "" when unknown or none.
func (g grant) sessionID() string {
	if g.ref == nil {
		return ""
	}
	return g.ref.sessionID
}

// revokeGrant writes the Revocation of g, then deletes the token and its reference.
func (r *Revoker) revokeGrant(ctx context.Context, p Pair, g grant, reason, opUID string, res *Result) error {
	name := revocation.GrantName(g.id)
	if err := r.writeRevocation(ctx, revocation.Revocation{
		Name:         name,
		Type:         revocation.TypeGrant,
		ConnectorID:  p.ConnectorID,
		UserID:       p.UserID,
		Subject:      revocation.Subject(p.UserID, p.ConnectorID),
		GrantID:      g.id,
		ClientID:     g.clientID,
		Clients:      revocation.SortedClients([]string{g.clientID}),
		Reason:       reason,
		Source:       revocation.SourceController,
		CreatedAt:    r.now(),
		OperationUID: opUID,
	}); err != nil {
		return err
	}
	res.addRevocation(name)
	deleted, err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, g.id)
	if err != nil {
		return err
	}
	if deleted {
		res.RefreshTokensDeleted++
	}
	if g.ref == nil {
		return nil
	}
	return r.removeRef(ctx, p.name(), *g.ref)
}

// endSession writes the Revocation of the session s, deletes the grants issued under it, then the
// session. It returns the ids of the grants.
func (r *Revoker) endSession(ctx context.Context, p Pair, s session, refs map[string]ref, reason, opUID string, res *Result) ([]string, error) {
	if err := r.writeSessionRevocation(ctx, p, s, reason, opUID, res); err != nil {
		return nil, err
	}

	var ended []string
	for _, id := range sortedKeys(refs) {
		rf := refs[id]
		if rf.sessionID != s.name {
			continue
		}
		ended = append(ended, rf.id)
		deleted, err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, rf.id)
		if err != nil {
			return nil, err
		}
		if deleted {
			res.RefreshTokensDeleted++
		}
		if err := r.removeRef(ctx, p.name(), rf); err != nil {
			return nil, err
		}
	}

	deleted, err := r.deleteDexObject(ctx, controller.AuthSessionGVK, s.name)
	if err != nil {
		return nil, err
	}
	if deleted {
		res.AuthSessionsDeleted++
	}
	return ended, nil
}

// sessionOf returns the session sid of the pair, or ErrSessionNotFound.
func (r *Revoker) sessionOf(ctx context.Context, p Pair, sid string) (session, error) {
	obj := controller.Object(controller.AuthSessionGVK)
	err := r.client.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: sid}, obj)
	if apierrors.IsNotFound(err) {
		return session{}, ErrSessionNotFound
	}
	if err != nil {
		return session{}, fmt.Errorf("get auth session: %w", err)
	}
	userID, _, _ := unstructured.NestedString(obj.Object, "userID")
	connectorID, _, _ := unstructured.NestedString(obj.Object, "connectorID")
	if userID != p.UserID || connectorID != p.ConnectorID {
		return session{}, ErrSessionNotFound
	}
	return decodeSession(obj), nil
}

// forgetGrantClient removes the client of the revoked grant g from the browser session g was issued
// under: the browser is signed out of that application, so the session no longer lists it among its
// applications and Dex does not sign the browser in to it again without a password. A grant issued
// without a session has nothing to remove; a session that is gone or no longer holds the client is
// left as it is.
func (r *Revoker) forgetGrantClient(ctx context.Context, g grant) error {
	if g.sessionID() == "" || g.clientID == "" {
		return nil
	}
	return r.removeClientState(ctx, g.sessionID(), g.clientID)
}

// removeClientState removes clientID from the client states of the session name.
func (r *Revoker) removeClientState(ctx context.Context, name, clientID string) error {
	patch := fmt.Sprintf(`[{"op":"remove","path":%q}]`, "/clientStates/"+escapePointer(clientID))
	return r.patchJSON(ctx, controller.AuthSessionGVK, name, patch)
}
