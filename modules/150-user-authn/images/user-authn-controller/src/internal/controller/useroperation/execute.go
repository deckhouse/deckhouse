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
	"math"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/revoke"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

// outcome is what a successful operation reports in its status.
type outcome struct {
	// revocation is set by the operations that revoke credentials.
	revocation *revoke.Result
}

func revoked(res revoke.Result) outcome {
	return outcome{revocation: &res}
}

// message is status.message of a successful operation.
func (o outcome) message() string {
	if o.revocation == nil {
		return ""
	}
	counts := fmt.Sprintf("%d refresh tokens and %d sessions deleted", o.revocation.RefreshTokensDeleted, o.revocation.AuthSessionsDeleted)
	if o.revocation.Generation == 0 {
		return counts
	}
	return fmt.Sprintf("generation %d; %s", o.revocation.Generation, counts)
}

func (r *Reconciler) execute(ctx context.Context, op operation, now time.Time) (outcome, error) {
	if err := ctx.Err(); err != nil {
		return outcome{}, err
	}
	if op.DecodeErr != nil {
		var perm failedError
		if errors.As(op.DecodeErr, &perm) {
			return outcome{}, perm
		}
		return outcome{}, failed(op.DecodeErr.Error())
	}

	switch op.Spec.Type {
	case typeResetPassword:
		return r.executeResetPassword(ctx, op)
	case typeReset2FA:
		return r.executeReset2FA(ctx, op, now)
	case typeLock:
		return r.executeLock(ctx, op, now)
	case typeUnlock:
		return outcome{}, r.executeUnlock(ctx, op, now)
	case typeLogout:
		return r.executeLogout(ctx, op)
	case typeRequirePasswordChange:
		return outcome{}, r.executeRequirePasswordChange(ctx, op)
	default:
		return outcome{}, failedf("unsupported operation type: %s", op.Spec.Type)
	}
}

// executeLock locks the user, then revokes everything issued to the pair: the lock keeps the user
// from signing in again, the revocation ends what was issued before.
func (r *Reconciler) executeLock(ctx context.Context, op operation, now time.Time) (outcome, error) {
	if op.Spec.Lock == nil {
		return outcome{}, failed("lock spec is nil")
	}

	lockedUntil, err := resolveLockUntil(op.Spec.Lock.For, now)
	if err != nil {
		return outcome{}, err
	}
	// The old place keeps whole seconds; UserIdentity gets the same instant, so that the copy
	// recognizes the old-place lock as the one already taken.
	lockedUntil = lockedUntil.UTC().Truncate(time.Second)

	if op.Spec.Target != nil {
		return r.lockExternal(ctx, op, lockedUntil, now)
	}

	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return outcome{}, err
	}
	pair := localPair(*userPassword)

	r.log.Info("Locking local user password", append(operationLogKV(op),
		"user", userPassword.Username,
		"for", op.Spec.Lock.For,
		"lockedUntil", lockedUntil.UTC().Format(time.RFC3339),
	)...)

	if err := r.lockIdentity(ctx, pair, lockedUntil, now); err != nil {
		return outcome{}, err
	}
	if err := r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"lockedUntil": lockedUntil.UTC().Format(time.RFC3339),
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: "true",
			},
		},
	}); err != nil {
		return outcome{}, err
	}

	return r.logoutPair(ctx, op, pair, revocation.ReasonLock)
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

func (r *Reconciler) executeResetPassword(ctx context.Context, op operation) (outcome, error) {
	if op.Spec.ResetPassword == nil {
		return outcome{}, failed("resetPassword spec is nil")
	}

	rawHash := op.Spec.ResetPassword.NewPasswordHash
	if !strings.HasPrefix(rawHash, "$2") {
		return outcome{}, failed("resetPassword.newPasswordHash must be a raw bcrypt hash starting with $2")
	}
	if _, err := bcrypt.Cost([]byte(rawHash)); err != nil {
		return outcome{}, failedf("resetPassword.newPasswordHash must be a valid bcrypt hash: %s", err.Error())
	}

	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return outcome{}, err
	}

	r.log.Info("Resetting local user password", append(operationLogKV(op), "user", userPassword.Username)...)

	if err := r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"hash":                            base64.StdEncoding.EncodeToString([]byte(rawHash)),
		"requireResetHashOnNextSuccLogin": true,
	}); err != nil {
		return outcome{}, err
	}

	return r.logoutPair(ctx, op, localPair(*userPassword), revocation.ReasonResetPassword)
}

// executeRequirePasswordChange makes the local user set a new password at the next sign-in, and
// revokes nothing. It is how a user asks to change their own password through user-api, which has
// no right to write Password objects.
func (r *Reconciler) executeRequirePasswordChange(ctx context.Context, op operation) error {
	if op.Spec.User == "" || op.Spec.Target != nil {
		return failed("RequirePasswordChange requires spec.user; it applies to local users only")
	}

	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return err
	}

	r.log.Info("Requiring a password change of a local user", append(operationLogKV(op), "user", userPassword.Username)...)

	return r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"requireResetHashOnNextSuccLogin": true,
	})
}

func (r *Reconciler) executeReset2FA(ctx context.Context, op operation, now time.Time) (outcome, error) {
	if op.Spec.User == "" {
		return outcome{}, failed("Reset2FA requires spec.user; it is only supported for local users")
	}
	if op.Spec.Target != nil {
		return outcome{}, failed("Reset2FA does not support an external target; it is only supported for local users")
	}

	userPassword, found, err := r.lookupLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return outcome{}, err
	}
	// The User controller writes the username as the userID of a local user, so without a
	// Password the username is the userID of the Dex objects left behind.
	pair := identityPair{userID: op.Spec.User, connectorID: naming.LocalConnectorID}
	if found {
		pair = localPair(*userPassword)
	}

	// OfflineSessions is where the Dex of 1.77 keeps the TOTP secret; it goes first, as in Unlock.
	deleted, err := r.deletePairSession(ctx, pair)
	if err != nil {
		return outcome{}, err
	}
	if !found {
		// Without a Password there is no local user to sign in, and the copy creates no
		// UserIdentity for one.
		r.log.Info("Reset2FA: no Password, no UserIdentity to reset",
			append(operationLogKV(op), "user", op.Spec.User, "offlineSessionsDeleted", deleted)...)
		return outcome{}, nil
	}
	if err := r.resetIdentityMFA(ctx, pair, now); err != nil {
		return outcome{}, err
	}
	return r.logoutPair(ctx, op, pair, revocation.ReasonReset2FA)
}

// deletePairSession deletes the OfflineSessions of the pair, the object named after the pair whose
// fields do not name another pair.
func (r *Reconciler) deletePairSession(ctx context.Context, p identityPair) (bool, error) {
	name := useridentity.Name(p.userID, p.connectorID)
	obj := controller.Object(controller.OfflineSessionsGVK)
	err := r.client.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get offlinesessions %s: %w", name, err)
	}
	sess, err := decodeSession(obj)
	if err != nil {
		return false, err
	}
	if (sess.UserID != "" && sess.UserID != p.userID) || (sess.ConnID != "" && sess.ConnID != p.connectorID) {
		return false, nil
	}
	r.log.Info("Deleting the OfflineSessions of the pair", "connectorID", p.connectorID, "userID", p.userID, "offlinesessions", name)
	if err := r.deleteDexObject(ctx, controller.OfflineSessionsGVK, naming.DexNamespace, name); err != nil {
		return false, err
	}
	return true, nil
}

// executeLogout ends the sessions of a user: by default everything of the pair, through a new
// generation; with spec.logout, one browser session, every session but one, or one client.
func (r *Reconciler) executeLogout(ctx context.Context, op operation) (outcome, error) {
	pair, err := r.operationPair(ctx, op)
	if err != nil {
		return outcome{}, err
	}
	l := op.Spec.Logout
	if l == nil {
		l = &logoutSpec{}
	}
	set := 0
	for _, v := range []string{l.Session, l.ExceptSession} {
		if v != "" {
			set++
		}
	}
	if set > 1 {
		return outcome{}, failed("logout accepts at most one of session and exceptSession")
	}

	r.log.Info("Logging the user out", append(operationLogKV(op), "connectorID", pair.connectorID, "userID", pair.userID)...)

	p := revokePair(pair)
	var res revoke.Result
	switch {
	case l.Session != "":
		res, err = r.revoker.LogoutSession(ctx, p, l.Session, op.UID)
	case l.ExceptSession != "":
		res, err = r.revoker.LogoutExceptSession(ctx, p, l.ExceptSession, op.UID)
	default:
		return r.logoutPair(ctx, op, pair, revocation.ReasonLogout)
	}
	if errors.Is(err, revoke.ErrSessionNotFound) {
		return outcome{}, failed("session not found")
	}
	if err != nil {
		return outcome{}, err
	}
	return revoked(res), nil
}

// operationPair is the pair spec.user or spec.target names.
func (r *Reconciler) operationPair(ctx context.Context, op operation) (identityPair, error) {
	if op.Spec.Target != nil {
		target, err := r.findExternalTarget(ctx, op.Spec.Target)
		if err != nil {
			return identityPair{}, err
		}
		return target.pair, nil
	}
	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return identityPair{}, err
	}
	return localPair(*userPassword), nil
}

// logoutPair revokes everything issued to the pair. It runs after the operation's own writes: a lock
// keeps the user from signing in again by then, and the revocation ends what was issued before.
func (r *Reconciler) logoutPair(ctx context.Context, op operation, p identityPair, reason string) (outcome, error) {
	res, err := r.revoker.LogoutPair(ctx, revokePair(p), reason, op.UID)
	if errors.Is(err, useridentity.ErrInvalidGeneration) {
		// Dex refuses the pair until the annotation is fixed; a retry changes nothing.
		return outcome{}, failedf("cannot raise the generation: %s", err.Error())
	}
	if err != nil {
		return outcome{}, err
	}
	return revoked(res), nil
}

func revokePair(p identityPair) revoke.Pair {
	return revoke.Pair{UserID: p.userID, ConnectorID: p.connectorID, Claims: p.claims}
}

func (r *Reconciler) lockExternal(ctx context.Context, op operation, lockedUntil, now time.Time) (outcome, error) {
	target, err := r.findExternalTarget(ctx, op.Spec.Target)
	if err != nil {
		return outcome{}, err
	}

	until := lockedUntil.UTC().Format(time.RFC3339)
	r.log.Info("Locking external user", append(operationLogKV(op),
		"connectorID", target.pair.connectorID,
		"userID", target.pair.userID,
		"for", op.Spec.Lock.For,
		"lockedUntil", until,
	)...)

	if err := r.lockIdentity(ctx, target.pair, lockedUntil, now); err != nil {
		return outcome{}, err
	}
	if target.session != nil {
		if err := r.patchMerge(ctx, controller.OfflineSessionsGVK, target.session.Namespace, target.session.Name, map[string]any{
			"lockedUntil":                    until,
			"incorrectPasswordLoginAttempts": int64(0),
			"metadata": map[string]any{
				"annotations": map[string]any{
					lockedByAdministratorAnnot: "true",
				},
			},
		}); err != nil {
			return outcome{}, err
		}
	}
	return r.logoutPair(ctx, op, target.pair, revocation.ReasonLock)
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

// externalTarget is the account that spec.target names.
type externalTarget struct {
	pair identityPair
	// session is the OfflineSessions of the pair, the place the Dex of 1.77 keeps the lock in.
	// Dex v2.46 creates none for a user who signed in without offline access.
	session *sessionView
}

// findExternalTarget finds the account of spec.target: a UserIdentity whose claims have the email,
// or, until the copy creates the UserIdentity, an OfflineSessions with the email. connectorID and
// userID, when set, narrow the search down to the pairs with them. Every login through Dex v2.46
// records the UserIdentity, so any provider type can be targeted. More than one account is an
// error.
func (r *Reconciler) findExternalTarget(ctx context.Context, target *operationTarget) (externalTarget, error) {
	if target == nil {
		return externalTarget{}, failed("target is nil")
	}
	if target.Email == "" {
		return externalTarget{}, failed("target.email is required")
	}
	// A local user is addressed by spec.user: Lock and Unlock of a local user also write
	// the Password object, which the target path does not.
	if target.ConnectorID == naming.LocalConnectorID {
		return externalTarget{}, failed("target cannot name the local provider; use spec.user for a local user")
	}
	matches := func(connectorID, userID, email string) bool {
		return userID != "" && connectorID != "" && connectorID != naming.LocalConnectorID &&
			(target.ConnectorID == "" || connectorID == target.ConnectorID) &&
			(target.UserID == "" || userID == target.UserID) &&
			strings.EqualFold(email, target.Email)
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
		if sess.UserID != "" && sess.ConnID != "" {
			sessionsByPair[useridentity.Name(sess.UserID, sess.ConnID)] = sess
		}
	}

	found := map[string]externalTarget{}
	for _, ident := range identities {
		if !matches(ident.ConnectorID, ident.UserID, ident.Email) {
			continue
		}
		name := useridentity.Name(ident.UserID, ident.ConnectorID)
		session := sessionsByPair[name]
		if session != nil && (session.UserID != ident.UserID || session.ConnID != ident.ConnectorID) {
			session = nil
		}
		found[name] = externalTarget{
			pair: identityPair{
				userID:      ident.UserID,
				connectorID: ident.ConnectorID,
				claims:      useridentity.Claims{UserID: ident.UserID, Email: ident.Email},
			},
			session: session,
		}
	}
	for name, sess := range sessionsByPair {
		if _, ok := found[name]; ok || !matches(sess.ConnID, sess.UserID, sess.Email) {
			continue
		}
		found[name] = externalTarget{pair: externalPair(*sess), session: sess}
	}

	switch len(found) {
	case 0:
		if target.ConnectorID == "" {
			return externalTarget{}, failedf("no account found for email %q (the user has likely never signed in)", target.Email)
		}
		return externalTarget{}, failedf("no account found for connector %q and email %q (the user has likely never signed in through this provider)", target.ConnectorID, target.Email)
	case 1:
		for _, t := range found {
			return t, nil
		}
	}
	if target.ConnectorID == "" {
		return externalTarget{}, failedf("%d accounts have email %q; the target is ambiguous", len(found), target.Email)
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

// revocationStatus is status.revocation of an operation that revoked credentials.
func revocationStatus(res *revoke.Result) map[string]any {
	status := map[string]any{
		"refreshTokensDeleted": int64(res.RefreshTokensDeleted),
		"authSessionsDeleted":  int64(res.AuthSessionsDeleted),
	}
	if res.Name != "" {
		status["name"] = res.Name
	}
	if res.Generation > 0 && res.Generation <= math.MaxInt64 {
		status["generation"] = int64(res.Generation)
	}
	return status
}
