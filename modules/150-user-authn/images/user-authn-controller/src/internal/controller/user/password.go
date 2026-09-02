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

package user

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

func passwordName(email string) string {
	return naming.ToFnvLikeDex(strings.ToLower(email))
}

func encodePasswordHash(rawPassword string) string {
	if strings.HasPrefix(rawPassword, "$2") {
		return base64.StdEncoding.EncodeToString([]byte(rawPassword))
	}
	return rawPassword
}

func (r *Reconciler) getPassword(ctx context.Context, name string) (passwordView, bool, error) {
	obj := controller.Object(controller.PasswordGVK)
	err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: naming.DexNamespace}, obj)
	if apierrors.IsNotFound(err) {
		return passwordView{}, false, nil
	}
	if err != nil {
		return passwordView{}, false, fmt.Errorf("get password %s: %w", name, err)
	}
	pw, err := decodePassword(obj)
	if err != nil {
		return passwordView{}, false, err
	}
	return pw, true, nil
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

func (r *Reconciler) reconcilePassword(
	ctx context.Context,
	user userView,
	email string,
	groups []string,
	now time.Time,
) (passwordView, error) {
	if err := ctx.Err(); err != nil {
		return passwordView{}, err
	}

	encodedName := passwordName(email)
	existing, passwordExists, err := r.getPassword(ctx, encodedName)
	if err != nil {
		return passwordView{}, err
	}

	live := existing
	isRename := false
	if !passwordExists {
		passwords, err := r.listPasswords(ctx)
		if err != nil {
			return passwordView{}, err
		}
		for _, pw := range passwords {
			if pw.Username != user.Name || !isModuleManagedPassword(pw) {
				continue
			}
			live = pw
			isRename = true
			break
		}
	}

	switch {
	case passwordExists:
		if err := r.patchExistingPassword(ctx, existing, user.Name, email, groups); err != nil {
			return passwordView{}, err
		}
		if err := r.stripExpiredAdminLock(ctx, existing, now); err != nil {
			return passwordView{}, err
		}
		if stripped := shouldStripAdminLock(existing, now); stripped {
			delete(existing.Annotations, lockedByAdministratorAnnot)
			live = existing
		}
	case isRename:
		fromAPI, err := r.passwordFromAPI(ctx, live.Name, live.Namespace)
		if err != nil {
			return passwordView{}, err
		}
		if err := r.createPasswordFromExisting(ctx, encodedName, user.Name, email, groups, fromAPI, now); err != nil {
			return passwordView{}, err
		}
		live = fromAPI
	default:
		if err := r.createPassword(ctx, encodedName, user.Name, email, user.Password, groups, now); err != nil {
			return passwordView{}, err
		}
	}

	return live, nil
}

func (r *Reconciler) passwordFromAPI(ctx context.Context, name, namespace string) (passwordView, error) {
	obj := controller.Object(controller.PasswordGVK)
	if err := r.apiReader.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
		return passwordView{}, fmt.Errorf("get password %s from api: %w", name, err)
	}
	return decodePassword(obj)
}

func shouldStripAdminLock(existing passwordView, now time.Time) bool {
	lockActive := existing.LockedUntil != nil && existing.LockedUntil.After(now)
	if lockActive {
		return false
	}
	_, ok := existing.Annotations[lockedByAdministratorAnnot]
	return ok
}

func (r *Reconciler) stripExpiredAdminLock(ctx context.Context, existing passwordView, now time.Time) error {
	if !shouldStripAdminLock(existing, now) {
		return nil
	}
	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				lockedByAdministratorAnnot: nil,
			},
		},
	}
	return r.patchPassword(ctx, existing.Name, existing.Namespace, patch)
}

func (r *Reconciler) patchExistingPassword(ctx context.Context, existing passwordView, username, email string, groups []string) error {
	hasKeep := existing.Annotations[helmResourcePolicyAnnotation] == helmResourcePolicyKeep
	if hasKeep &&
		existing.Email == email &&
		existing.Username == username &&
		existing.UserID == username &&
		equalStringSets(existing.Groups, groups) {
		return nil
	}

	patch := map[string]any{
		"metadata": map[string]any{
			"labels": passwordObjectLabels(),
			"annotations": map[string]any{
				helmResourcePolicyAnnotation: helmResourcePolicyKeep,
			},
		},
		"email":    email,
		"username": username,
		"userID":   username,
		"groups":   groups,
	}
	if groups == nil {
		patch["groups"] = []string{}
	}
	return r.patchPassword(ctx, existing.Name, existing.Namespace, patch)
}

func (r *Reconciler) patchPassword(ctx context.Context, name, namespace string, patch map[string]any) error {
	raw, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal password patch: %w", err)
	}
	obj := controller.Object(controller.PasswordGVK)
	obj.SetName(name)
	obj.SetNamespace(namespace)
	if err := r.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, raw)); err != nil {
		return fmt.Errorf("patch password %s: %w", name, err)
	}
	return nil
}

func (r *Reconciler) createPassword(ctx context.Context, encodedName, username, email, rawPassword string, groups []string, now time.Time) error {
	obj := newPasswordObject(encodedName, username, email, rawPassword, groups, now)
	if err := r.client.Create(ctx, obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create password %s: %w", encodedName, err)
	}
	return nil
}

func (r *Reconciler) createPasswordFromExisting(ctx context.Context, encodedName, username, email string, groups []string, existing passwordView, now time.Time) error {
	obj := newPasswordObjectFromExisting(encodedName, username, email, groups, existing, now)
	if err := r.client.Create(ctx, obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return r.patchExistingPassword(ctx, passwordView{
				Name:        encodedName,
				Namespace:   naming.DexNamespace,
				Annotations: map[string]string{},
			}, username, email, groups)
		}
		return fmt.Errorf("create renamed password %s: %w", encodedName, err)
	}
	return nil
}

func newPasswordObject(encodedName, username, email, rawPassword string, groups []string, now time.Time) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": controller.PasswordGVK.GroupVersion().String(),
		"kind":       controller.PasswordGVK.Kind,
		"metadata": map[string]any{
			"name":      encodedName,
			"namespace": naming.DexNamespace,
			"labels":    passwordObjectLabels(),
			"annotations": map[string]any{
				helmResourcePolicyAnnotation: helmResourcePolicyKeep,
			},
		},
		"email":         email,
		"username":      username,
		"userID":        username,
		"hash":          encodePasswordHash(rawPassword),
		"hashUpdatedAt": now.UTC().Format(time.RFC3339),
	}
	if len(groups) > 0 {
		obj["groups"] = toUnstructuredSlice(groups)
	}
	return &unstructured.Unstructured{Object: obj}
}

func newPasswordObjectFromExisting(encodedName, username, email string, groups []string, existing passwordView, now time.Time) *unstructured.Unstructured {
	annotations := map[string]any{
		helmResourcePolicyAnnotation: helmResourcePolicyKeep,
	}
	lockActive := existing.LockedUntil != nil && existing.LockedUntil.After(now)
	if v, ok := existing.Annotations[lockedByAdministratorAnnot]; ok && lockActive {
		annotations[lockedByAdministratorAnnot] = v
	}

	obj := map[string]any{
		"apiVersion": controller.PasswordGVK.GroupVersion().String(),
		"kind":       controller.PasswordGVK.Kind,
		"metadata": map[string]any{
			"name":        encodedName,
			"namespace":   naming.DexNamespace,
			"labels":      passwordObjectLabels(),
			"annotations": annotations,
		},
		"email":                           email,
		"username":                        username,
		"userID":                          username,
		"hash":                            existing.Hash,
		"requireResetHashOnNextSuccLogin": existing.RequireResetHashOnNextSuccLogin,
	}
	if existing.HashUpdatedAt != "" {
		obj["hashUpdatedAt"] = existing.HashUpdatedAt
	}
	if existing.IncorrectPasswordLoginAttempts != 0 {
		obj["incorrectPasswordLoginAttempts"] = existing.IncorrectPasswordLoginAttempts
	}
	if existing.LockedUntil != nil {
		obj["lockedUntil"] = existing.LockedUntil.UTC().Format(time.RFC3339)
	}
	if len(existing.PreviousHashes) > 0 {
		obj["previousHashes"] = toUnstructuredSlice(existing.PreviousHashes)
	}
	if len(groups) > 0 {
		obj["groups"] = toUnstructuredSlice(groups)
	}
	return &unstructured.Unstructured{Object: obj}
}

func (r *Reconciler) cleanupPasswords(ctx context.Context, reqName string, userFound bool, user userView) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	passwords, err := r.listPasswords(ctx)
	if err != nil {
		return err
	}
	expectedName := ""
	if userFound {
		expectedName = passwordName(user.Email)
	}
	var canonicalNames map[string]struct{}
	for _, pw := range passwords {
		if !isModuleManagedPassword(pw) {
			continue
		}
		if expectedName != "" && pw.Name == expectedName {
			continue
		}
		if pw.Username != "" {
			if pw.Username != reqName {
				continue
			}
		} else {
			if canonicalNames == nil {
				var listErr error
				canonicalNames, listErr = r.canonicalPasswordNames(ctx)
				if listErr != nil {
					return listErr
				}
			}
			if _, ok := canonicalNames[pw.Name]; ok {
				continue
			}
		}
		if err := r.deletePassword(ctx, pw); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) canonicalPasswordNames(ctx context.Context) (map[string]struct{}, error) {
	list := controller.List(controller.UserGVK)
	if err := r.client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	out := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		u, err := decodeUser(&list.Items[i])
		if err != nil {
			return nil, err
		}
		if u.Email == "" {
			continue
		}
		out[passwordName(u.Email)] = struct{}{}
	}
	return out, nil
}

func (r *Reconciler) deletePassword(ctx context.Context, pw passwordView) error {
	obj := controller.Object(controller.PasswordGVK)
	obj.SetName(pw.Name)
	obj.SetNamespace(pw.Namespace)
	if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete password %s: %w", pw.Name, err)
	}
	r.log.Info("deleted orphaned password", "name", pw.Name, "username", pw.Username)
	return nil
}
