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
		return r.executeReset2FA(ctx, op)
	case typeLock:
		return r.executeLock(ctx, op, now)
	case typeUnlock:
		return r.executeUnlock(ctx, op)
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

	if op.Spec.Target != nil {
		return r.lockOfflineSession(ctx, op, lockedUntil)
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

	if err := r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"lockedUntil": lockedUntil.UTC().Format(time.RFC3339),
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: "",
			},
		},
	}); err != nil {
		return err
	}

	if _, err := r.invalidateLocalUserSessions(ctx, op.Spec.User, "Locking user"); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) executeUnlock(ctx context.Context, op operation) error {
	if op.Spec.Target != nil {
		return r.unlockOfflineSession(ctx, op)
	}

	userPassword, err := r.findLocalPassword(ctx, op.Spec.User)
	if err != nil {
		return err
	}

	r.log.Info("Unlocking local user password", append(operationLogKV(op), "user", userPassword.Username)...)

	return r.patchMerge(ctx, controller.PasswordGVK, userPassword.Namespace, userPassword.Name, map[string]any{
		"lockedUntil": nil,
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: nil,
			},
		},
	})
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

	if _, err := r.invalidateLocalUserSessions(ctx, op.Spec.User, "Resetting user password"); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) executeReset2FA(ctx context.Context, op operation) error {
	if op.Spec.User == "" {
		return failed("Reset2FA requires spec.user; it is only supported for local users")
	}
	if op.Spec.Target != nil {
		return failed("Reset2FA does not support an external target; it is only supported for local users")
	}

	anyDeleted, err := r.invalidateLocalUserSessions(ctx, op.Spec.User, "Resetting user 2FA")
	if err != nil {
		return err
	}
	if !anyDeleted {
		r.log.Info("Reset2FA: no 2FA objects found, nothing to delete",
			append(operationLogKV(op), "user", op.Spec.User)...)
	}
	return nil
}

func (r *Reconciler) invalidateLocalUserSessions(ctx context.Context, username, logPrefix string) (bool, error) {
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
		matchesUser := false
		if sess.UserID != "" {
			matchesUser = sess.UserID == username
		} else if len(sess.RefreshTokenIDs) > 0 {
			for _, id := range sess.RefreshTokenIDs {
				rt, ok := tokensByID[id]
				if !ok {
					continue
				}
				if rt.ClaimsUsername == username || rt.ClaimsUserID == username || rt.ClaimsPreferred == username {
					matchesUser = true
					break
				}
			}
		}
		if !matchesUser {
			continue
		}
		r.log.Info(logPrefix+": deleting OfflineSessions", "user", username, "offlinesession", sess.Name)
		if err := r.deleteDexObject(ctx, controller.OfflineSessionsGVK, sess.Namespace, sess.Name); err != nil {
			return anyDeleted, err
		}
		anyDeleted = true
	}

	for _, rt := range tokens {
		if rt.ClaimsUsername == username || rt.ClaimsUserID == username || rt.ClaimsPreferred == username {
			r.log.Info(logPrefix+": deleting RefreshToken", "user", username, "refreshtoken", rt.Name)
			if err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, rt.Namespace, rt.Name); err != nil {
				return anyDeleted, err
			}
			anyDeleted = true
		}
	}

	return anyDeleted, nil
}

func (r *Reconciler) lockOfflineSession(ctx context.Context, op operation, lockedUntil time.Time) error {
	sess, err := r.findOfflineSessionByTarget(ctx, op.Spec.Target)
	if err != nil {
		return err
	}

	until := lockedUntil.UTC().Format(time.RFC3339)
	r.log.Info("Locking external user via OfflineSessions", append(operationLogKV(op),
		"offlinesession", sess.Name,
		"for", op.Spec.Lock.For,
		"lockedUntil", until,
	)...)

	return r.patchMerge(ctx, controller.OfflineSessionsGVK, sess.Namespace, sess.Name, map[string]any{
		"lockedUntil":                    until,
		"incorrectPasswordLoginAttempts": int64(0),
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: "true",
			},
		},
	})
}

func (r *Reconciler) unlockOfflineSession(ctx context.Context, op operation) error {
	sess, err := r.findOfflineSessionByTarget(ctx, op.Spec.Target)
	if err != nil {
		return err
	}

	r.log.Info("Unlocking external user via OfflineSessions", append(operationLogKV(op), "offlinesession", sess.Name)...)

	return r.patchMerge(ctx, controller.OfflineSessionsGVK, sess.Namespace, sess.Name, map[string]any{
		"lockedUntil":                    nil,
		"incorrectPasswordLoginAttempts": int64(0),
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: nil,
			},
		},
	})
}

func (r *Reconciler) findLocalPassword(ctx context.Context, username string) (*passwordView, error) {
	passwords, err := r.listPasswords(ctx)
	if err != nil {
		return nil, err
	}
	for i := range passwords {
		if passwords[i].Username == username {
			return &passwords[i], nil
		}
	}
	return nil, failedf("cannot find password for user: %s", username)
}

func (r *Reconciler) findOfflineSessionByTarget(ctx context.Context, target *operationTarget) (*sessionView, error) {
	if target == nil {
		return nil, failed("target is nil")
	}
	if target.Email == "" || target.ConnectorID == "" {
		return nil, failed("target.connectorID and target.email are required")
	}

	sessions, err := r.listSessions(ctx)
	if err != nil {
		return nil, err
	}
	wantEmail := strings.ToLower(target.Email)
	for i := range sessions {
		sess := &sessions[i]
		if sess.ConnID != target.ConnectorID {
			continue
		}
		if strings.ToLower(sess.Email) != wantEmail {
			continue
		}
		return sess, nil
	}
	return nil, failedf("no OfflineSessions found for connector %q and email %q (the user has likely never logged in yet)", target.ConnectorID, target.Email)
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
