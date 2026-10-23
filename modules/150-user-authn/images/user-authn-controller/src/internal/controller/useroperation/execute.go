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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

func (r *Reconciler) execute(ctx context.Context, op operation, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if op.DecodeErr != nil {
		var perm failedError
		if errors.As(op.DecodeErr, &perm) {
			return perm
		}
		return failed(op.DecodeErr.Error())
	}

	switch op.Spec.Type {
	case typeResetPassword:
		return r.executeResetPassword(ctx, op)
	case typeReset2FA:
		return r.executeReset2FA(ctx, op, now)
	case typeLock:
		return r.executeLock(ctx, op, now)
	case typeUnlock:
		return r.executeUnlock(ctx, op, now)
	default:
		return failedf("unsupported operation type: %s", op.Spec.Type)
	}
}

func (r *Reconciler) executeLock(ctx context.Context, op operation, now time.Time) error {
	if op.Spec.Lock == nil {
		return failed("lock spec is nil")
	}

	lockedUntil, err := resolveLockUntil(op.Spec.Lock.For, now)
	if err != nil {
		return err
	}
	// The old place keeps whole seconds; UserIdentity gets the same instant, so that the copy
	// recognizes the old-place lock as the one already taken.
	lockedUntil = lockedUntil.UTC().Truncate(time.Second)

	if op.Spec.Target != nil {
		return r.lockExternal(ctx, op, lockedUntil, now)
	}

	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return err
	}

	r.log.Info("Locking local user password", append(operationLogKV(op),
		"user", userPassword.Username,
		"for", op.Spec.Lock.For,
		"lockedUntil", lockedUntil.UTC().Format(time.RFC3339),
	)...)

	if err := r.lockIdentity(ctx, localPair(*userPassword), lockedUntil, now); err != nil {
		return err
	}
	if err := r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"lockedUntil": lockedUntil.UTC().Format(time.RFC3339),
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: "true",
			},
		},
	}); err != nil {
		return err
	}

	return r.endLocalUserSessions(ctx, localPair(*userPassword).userID, "Locking user")
}

func (r *Reconciler) executeUnlock(ctx context.Context, op operation, now time.Time) error {
	if op.Spec.Target != nil {
		return r.unlockExternal(ctx, op, now)
	}

	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return err
	}

	r.log.Info("Unlocking local user password", append(operationLogKV(op), "user", userPassword.Username)...)

	if err := r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"lockedUntil": nil,
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: nil,
			},
		},
	}); err != nil {
		return err
	}
	return r.unlockIdentity(ctx, localPair(*userPassword), now)
}

func (r *Reconciler) executeResetPassword(ctx context.Context, op operation) error {
	if op.Spec.ResetPassword == nil {
		return failed("resetPassword spec is nil")
	}

	rawHash := op.Spec.ResetPassword.NewPasswordHash
	if !strings.HasPrefix(rawHash, "$2") {
		return failed("resetPassword.newPasswordHash must be a raw bcrypt hash starting with $2")
	}
	if _, err := bcrypt.Cost([]byte(rawHash)); err != nil {
		return failedf("resetPassword.newPasswordHash must be a valid bcrypt hash: %s", err.Error())
	}

	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return err
	}

	r.log.Info("Resetting local user password", append(operationLogKV(op), "user", userPassword.Username)...)

	if err := r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"hash":                            base64.StdEncoding.EncodeToString([]byte(rawHash)),
		"requireResetHashOnNextSuccLogin": true,
	}); err != nil {
		return err
	}

	return r.endLocalUserSessions(ctx, localPair(*userPassword).userID, "Resetting user password")
}

func (r *Reconciler) executeReset2FA(ctx context.Context, op operation, now time.Time) error {
	if op.Spec.User == "" {
		return failed("Reset2FA requires spec.user; it is only supported for local users")
	}
	if op.Spec.Target != nil {
		return failed("Reset2FA does not support an external target; it is only supported for local users")
	}

	userPassword, found, err := r.lookupLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return err
	}
	// The User controller writes the username as the userID of a local user, so without a
	// Password the username is the userID of the Dex objects left behind.
	userID := op.Spec.User
	if found {
		userID = localPair(*userPassword).userID
	}

	anyDeleted, err := r.invalidateLocalUserSessions(ctx, userID, "Resetting user 2FA")
	if err != nil {
		return err
	}
	if !anyDeleted {
		r.log.Info("Reset2FA: no 2FA objects found, nothing to delete",
			append(operationLogKV(op), "user", op.Spec.User)...)
	}

	if !found {
		// Without a Password there is no local user, and the copy creates no UserIdentity for one.
		r.log.Info("Reset2FA: no Password, no UserIdentity to reset", append(operationLogKV(op), "user", op.Spec.User)...)
		return nil
	}
	return r.resetIdentityMFA(ctx, localPair(*userPassword), now)
}

// endLocalUserSessions deletes the refresh tokens of the local user userID, so that every client
// has to sign the user in again. The second factor stays: only Reset2FA removes it.
func (r *Reconciler) endLocalUserSessions(ctx context.Context, userID, logPrefix string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tokens, err := r.listRefreshTokens(ctx)
	if err != nil {
		return err
	}
	_, err = r.deleteRefreshTokensOf(ctx, tokens, userID, logPrefix)
	return err
}

// invalidateLocalUserSessions deletes the OfflineSessions and the refresh tokens of the local user
// userID. OfflineSessions is where the Dex of 1.77 keeps the TOTP secret; Reset2FA removes it
// there too.
func (r *Reconciler) invalidateLocalUserSessions(ctx context.Context, userID, logPrefix string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	tokens, err := r.listRefreshTokens(ctx)
	if err != nil {
		return false, err
	}
	tokensByID := make(map[string]refreshTokenView, len(tokens))
	for _, rt := range tokens {
		tokensByID[rt.Name] = rt
	}

	var anyDeleted bool

	sessions, err := r.listSessions(ctx)
	if err != nil {
		return false, err
	}
	for _, sess := range sessions {
		if !sessionOf(sess, userID, tokensByID) {
			continue
		}
		r.log.Info(logPrefix+": deleting OfflineSessions", "userID", userID, "offlinesession", sess.Name)
		if err := r.deleteDexObject(ctx, controller.OfflineSessionsGVK, sess.Namespace, sess.Name); err != nil {
			return anyDeleted, err
		}
		anyDeleted = true
	}

	deleted, err := r.deleteRefreshTokensOf(ctx, tokens, userID, logPrefix)
	return anyDeleted || deleted, err
}

// deleteRefreshTokensOf deletes the refresh tokens of the local user userID among tokens.
func (r *Reconciler) deleteRefreshTokensOf(ctx context.Context, tokens []refreshTokenView, userID, logPrefix string) (bool, error) {
	var anyDeleted bool
	for _, rt := range tokens {
		if !tokenOf(rt, userID) {
			continue
		}
		r.log.Info(logPrefix+": deleting RefreshToken", "userID", userID, "refreshtoken", rt.Name)
		if err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, rt.Namespace, rt.Name); err != nil {
			return anyDeleted, err
		}
		anyDeleted = true
	}
	return anyDeleted, nil
}

// tokenOf reports whether rt is a refresh token of the local user userID: one that Dex issued
// through the local connector with the user's ID in the claims. A user of another provider can
// have the same name, ID or preferred username, and their tokens are not touched.
func tokenOf(rt refreshTokenView, userID string) bool {
	return rt.ConnectorID == naming.LocalConnectorID && rt.ClaimsUserID == userID
}

// sessionOf reports whether sess is the OfflineSessions of the local user userID. An object
// without a userID is the user's when a refresh token it refers to is.
func sessionOf(sess sessionView, userID string, tokensByID map[string]refreshTokenView) bool {
	if sess.UserID != "" {
		return sess.ConnID == naming.LocalConnectorID && sess.UserID == userID
	}
	if sess.ConnID != "" && sess.ConnID != naming.LocalConnectorID {
		return false
	}
	for _, id := range sess.RefreshTokenIDs {
		if rt, ok := tokensByID[id]; ok && tokenOf(rt, userID) {
			return true
		}
	}
	return false
}

func (r *Reconciler) lockExternal(ctx context.Context, op operation, lockedUntil, now time.Time) error {
	target, err := r.findExternalTarget(ctx, op.Spec.Target)
	if err != nil {
		return err
	}

	until := lockedUntil.UTC().Format(time.RFC3339)
	r.log.Info("Locking external user", append(operationLogKV(op),
		"connectorID", target.pair.connectorID,
		"userID", target.pair.userID,
		"for", op.Spec.Lock.For,
		"lockedUntil", until,
	)...)

	if err := r.lockIdentity(ctx, target.pair, lockedUntil, now); err != nil {
		return err
	}
	if target.session == nil {
		return nil
	}
	return r.patchMerge(ctx, controller.OfflineSessionsGVK, target.session.Namespace, target.session.Name, map[string]any{
		"lockedUntil":                    until,
		"incorrectPasswordLoginAttempts": int64(0),
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: "true",
			},
		},
	})
}

func (r *Reconciler) unlockExternal(ctx context.Context, op operation, now time.Time) error {
	target, err := r.findExternalTarget(ctx, op.Spec.Target)
	if err != nil {
		return err
	}

	r.log.Info("Unlocking external user", append(operationLogKV(op),
		"connectorID", target.pair.connectorID,
		"userID", target.pair.userID,
	)...)

	if target.session != nil {
		if err := r.patchMerge(ctx, controller.OfflineSessionsGVK, target.session.Namespace, target.session.Name, map[string]any{
			"lockedUntil":                    nil,
			"incorrectPasswordLoginAttempts": int64(0),
			"metadata": map[string]any{
				"annotations": map[string]any{
					lockedByAdministratorAnnot: nil,
				},
			},
		}); err != nil {
			return err
		}
	}
	return r.unlockIdentity(ctx, target.pair, now)
}

// externalTarget is the account of an external provider that spec.target names.
type externalTarget struct {
	pair identityPair
	// session is the OfflineSessions of the pair, the place the Dex of 1.77 keeps the lock in.
	// Dex v2.46 creates none for a user who signed in without offline access.
	session *sessionView
}

// findExternalTarget finds the account of spec.target: a UserIdentity of the connector whose
// claims have the email, or, until the copy creates the UserIdentity, an OfflineSessions of the
// connector with the email. Every login through Dex v2.46 records the UserIdentity, so any
// provider type can be targeted. More than one account with the email is an error.
func (r *Reconciler) findExternalTarget(ctx context.Context, target *operationTarget) (externalTarget, error) {
	if target == nil {
		return externalTarget{}, failed("target is nil")
	}
	if target.Email == "" || target.ConnectorID == "" {
		return externalTarget{}, failed("target.connectorID and target.email are required")
	}

	identities, err := r.listIdentities(ctx)
	if err != nil {
		return externalTarget{}, err
	}
	sessions, err := r.listSessions(ctx)
	if err != nil {
		return externalTarget{}, err
	}
	sessionsByPair := make(map[string]*sessionView, len(sessions))
	for i := range sessions {
		sess := &sessions[i]
		if sess.ConnID == target.ConnectorID && sess.UserID != "" {
			sessionsByPair[useridentity.Name(sess.UserID, sess.ConnID)] = sess
		}
	}

	found := map[string]externalTarget{}
	for _, ident := range identities {
		if ident.ConnectorID != target.ConnectorID || ident.UserID == "" || !strings.EqualFold(ident.Email, target.Email) {
			continue
		}
		name := useridentity.Name(ident.UserID, ident.ConnectorID)
		found[name] = externalTarget{
			pair: identityPair{
				userID:      ident.UserID,
				connectorID: ident.ConnectorID,
				claims:      useridentity.Claims{UserID: ident.UserID, Email: ident.Email},
			},
			session: sessionsByPair[name],
		}
	}
	for name, sess := range sessionsByPair {
		if _, ok := found[name]; ok || !strings.EqualFold(sess.Email, target.Email) {
			continue
		}
		found[name] = externalTarget{pair: externalPair(*sess), session: sess}
	}

	switch len(found) {
	case 0:
		return externalTarget{}, failedf("no account found for connector %q and email %q (the user has likely never signed in through this provider)", target.ConnectorID, target.Email)
	case 1:
		for _, t := range found {
			return t, nil
		}
	}
	return externalTarget{}, failedf("%d accounts of connector %q have email %q; the target is ambiguous", len(found), target.ConnectorID, target.Email)
}

func (r *Reconciler) findLocalPassword(ctx context.Context, username string) (*passwordView, error) {
	pw, found, err := r.lookupLocalPassword(ctx, username)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, failedf("cannot find password for user: %s", username)
	}
	return pw, nil
}

func (r *Reconciler) lookupLocalPassword(ctx context.Context, username string) (*passwordView, bool, error) {
	passwords, err := r.listPasswords(ctx)
	if err != nil {
		return nil, false, err
	}
	for i := range passwords {
		if passwords[i].Username == username {
			return &passwords[i], true, nil
		}
	}
	return nil, false, nil
}

func (r *Reconciler) listPasswords(ctx context.Context) ([]passwordView, error) {
	list := controller.List(controller.PasswordGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return nil, fmt.Errorf("list passwords: %w", err)
	}
	out := make([]passwordView, 0, len(list.Items))
	for i := range list.Items {
		pw, err := decodePassword(&list.Items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, pw)
	}
	return out, nil
}

func (r *Reconciler) listSessions(ctx context.Context) ([]sessionView, error) {
	list := controller.List(controller.OfflineSessionsGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return nil, fmt.Errorf("list offlinesessions: %w", err)
	}
	out := make([]sessionView, 0, len(list.Items))
	for i := range list.Items {
		sess, err := decodeSession(&list.Items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, nil
}

func (r *Reconciler) listIdentities(ctx context.Context) ([]identityView, error) {
	list := controller.List(controller.UserIdentityGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return nil, fmt.Errorf("list useridentities: %w", err)
	}
	out := make([]identityView, 0, len(list.Items))
	for i := range list.Items {
		ident, err := decodeIdentity(&list.Items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, ident)
	}
	return out, nil
}

func (r *Reconciler) listRefreshTokens(ctx context.Context) ([]refreshTokenView, error) {
	list := controller.List(controller.RefreshTokenGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return nil, fmt.Errorf("list refreshtokens: %w", err)
	}
	out := make([]refreshTokenView, 0, len(list.Items))
	for i := range list.Items {
		rt, err := decodeRefreshToken(&list.Items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, nil
}

func (r *Reconciler) patchMerge(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string, patch map[string]any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal merge patch for %s/%s: %w", gvk.Kind, name, err)
	}
	obj := controller.Object(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	if err := r.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, raw)); err != nil {
		return fmt.Errorf("patch %s %s: %w", gvk.Kind, name, err)
	}
	return nil
}

func (r *Reconciler) deleteDexObject(ctx context.Context, gvk schema.GroupVersionKind, namespace, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj := controller.Object(gvk)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete %s %s: %w", gvk.Kind, name, err)
	}
	return nil
}
