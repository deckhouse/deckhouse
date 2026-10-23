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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
)

// The logouts narrowed down to one client, one browser session, or every session but one. They keep
// the generation of the pair: the rest of what the pair was issued stays valid.

// ErrSessionNotFound means the browser session does not exist or belongs to another pair. The two
// are not told apart, so that an operation cannot find out the sessions of other users.
var ErrSessionNotFound = errors.New("session not found")

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
	if _, err := r.endSession(ctx, p, s, refs, revocation.ReasonLogoutSession, opUID, &res); err != nil {
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
	if _, err := r.sessionOf(ctx, p, keep); err != nil {
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
	for clientID, ref := range refs {
		grants[ref.id] = grant{id: ref.id, clientID: clientID, hasRef: true, sessionID: ref.sessionID}
	}
	for _, id := range sortedKeys(grants) {
		g := grants[id]
		if _, ok := done[id]; ok || (g.hasRef && g.sessionID == keep) {
			continue
		}
		if err := r.revokeGrant(ctx, p, g, revocation.ReasonLogoutExceptSession, opUID, &res); err != nil {
			return res, err
		}
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

// grant is a refresh token of the pair: an object, a reference of OfflineSessions, or both.
type grant struct {
	id        string
	clientID  string
	hasRef    bool
	sessionID string
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
	if !g.hasRef {
		return nil
	}
	return r.removeRef(ctx, p.name(), g.clientID, g.id)
}

// endSession writes the Revocation of the session s, deletes the grants issued under it, then the
// session. It returns the ids of the grants.
func (r *Revoker) endSession(ctx context.Context, p Pair, s session, refs map[string]ref, reason, opUID string, res *Result) ([]string, error) {
	name := revocation.SessionName(s.name)
	if err := r.writeRevocation(ctx, revocation.Revocation{
		Name:         name,
		Type:         revocation.TypeSession,
		ConnectorID:  p.ConnectorID,
		UserID:       p.UserID,
		Subject:      revocation.Subject(p.UserID, p.ConnectorID),
		SessionID:    s.name,
		Clients:      revocation.SortedClients(s.clients),
		Reason:       reason,
		Source:       revocation.SourceController,
		CreatedAt:    r.now(),
		OperationUID: opUID,
	}); err != nil {
		return nil, err
	}
	res.addRevocation(name)

	var ended []string
	for _, clientID := range sortedKeys(refs) {
		ref := refs[clientID]
		if ref.sessionID != s.name {
			continue
		}
		ended = append(ended, ref.id)
		deleted, err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, ref.id)
		if err != nil {
			return nil, err
		}
		if deleted {
			res.RefreshTokensDeleted++
		}
		if err := r.removeRef(ctx, p.name(), clientID, ref.id); err != nil {
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
