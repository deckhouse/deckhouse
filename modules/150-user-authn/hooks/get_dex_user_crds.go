/*
Copyright 2023 Flant JSC

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

package hooks

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	// dexNamespace is the namespace Dex Password objects live in. It mirrors the
	// d8-<chart name> namespace the module deploys into (chart name: user-authn).
	dexNamespace = "d8-user-authn"

	// helmResourcePolicyAnnotation / helmResourcePolicyKeep tell Helm to skip
	// pruning a resource that is no longer rendered by the chart. We stamp it on
	// every Password object so that removing templates/dex/passwords.yaml does not
	// make Helm delete the existing (previously Helm-owned) Password objects. From
	// that point on the hook is the sole owner of Password objects.
	helmResourcePolicyAnnotation = "helm.sh/resource-policy"
	helmResourcePolicyKeep       = "keep"
)

type userStatusPatch struct {
	ExpireAt string      `json:"expireAt,omitempty"`
	Groups   []string    `json:"groups"`
	Lock     DexUserLock `json:"lock"`
}

type DexUserInternalValues struct {
	Name        string `json:"name"`
	EncodedName string `json:"encodedName"`

	Spec   DexUserSpec   `json:"spec"`
	Status DexUserStatus `json:"status,omitempty"`

	ExpireAt string `json:"-"`
}

type DexUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexUserSpec   `json:"spec"`
	Status            DexUserStatus `json:"status,omitempty"`
}

type DexUserSpec struct {
	Email    string   `json:"email"`
	Password string   `json:"password"`
	UserID   string   `json:"userID,omitempty"`
	Groups   []string `json:"groups,omitempty"`
	TTL      string   `json:"ttl,omitempty"`
}

type DexUserStatus struct {
	ExpireAt string      `json:"expireAt,omitempty"`
	Lock     DexUserLock `json:"lock"`
}

type DexUserLockReason string

const (
	PasswordPolicyLockout = DexUserLockReason("PasswordPolicyLockout")
	LockedByAdministrator = DexUserLockReason("LockedByAdministrator")
)

type DexUserLock struct {
	State   bool               `json:"state"`
	Reason  *DexUserLockReason `json:"reason,omitempty"`
	Message *string            `json:"message,omitempty"`
	Until   *string            `json:"until,omitempty"`
}

type DexGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DexGroupSpec   `json:"spec"`
	Status            DexGroupStatus `json:"status,omitempty"`
}

type DexGroupSpec struct {
	Name    string           `json:"name"`
	Members []DexGroupMember `json:"members" yaml:"members"`
}

type DexGroupMember struct {
	Kind string `json:"kind" yaml:"kind"`
	Name string `json:"name" yaml:"name"`
}

type DexGroupStatus struct {
	Errors []struct {
		Message   string `json:"message"`
		ObjectRef struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		} `json:"objectRef"`
	} `json:"errors,omitempty"`
}

const (
	PasswordAnnotationLockedByAdministrator = "deckhouse.io/locked-by-administrator"
)

type Password struct {
	metav1.TypeMeta                 `json:",inline"`
	metav1.ObjectMeta               `json:"metadata,omitempty"`
	Username                        string     `json:"username"`
	Email                           string     `json:"email"`
	UserID                          string     `json:"userID"`
	Hash                            string     `json:"hash"`
	Groups                          []string   `json:"groups,omitempty"`
	RequireResetHashOnNextSuccLogin bool       `json:"requireResetHashOnNextSuccLogin"`
	LockedUntil                     *time.Time `json:"lockedUntil"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/user-authn",
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "users",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "User",
			FilterFunc: applyDexUserFilter,
		},
		{
			Name:       "groups",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Group",
			FilterFunc: applyDexGroupFilter,
		},
		{
			Name:       "passwords",
			ApiVersion: "dex.coreos.com/v1",
			Kind:       "Password",
			FilterFunc: applyPasswordFilter,
			// The hook itself creates and patches Password objects. Reacting to
			// those self-induced events would cause a reconcile loop, so we only
			// snapshot Password objects (for lock-state sync and orphan cleanup)
			// and let User / Group events plus the cron drive reconciliation.
			// Trade-off: a Password-only change (e.g. Dex setting lockedUntil)
			// surfaces into User.status.lock on the next cron tick rather than
			// instantly.
			ExecuteHookOnEvents: ptr.To(false),
		},
	},
}, getDexUsers)

func getDexUsers(_ context.Context, input *go_hook.HookInput) error {
	now := time.Now()

	users := make([]DexUserInternalValues, 0, len(input.Snapshots.Get("users")))
	mapOfUsersToGroups := map[string]map[string]bool{}

	// Index existing Password objects by their object name. That name equals the
	// FNV-like Dex encoding of the lowercased email, which is exactly how Dex
	// resolves a password on login (cli.idToName(email)). Keying on it keeps our
	// create / update / delete decisions consistent with Dex's own lookup and
	// makes an email change (object name change) behave correctly.
	passwordsSnap := input.Snapshots.Get("passwords")
	passwordsByName := make(map[string]Password, len(passwordsSnap))
	allPasswords := make([]Password, 0, len(passwordsSnap))
	for password, err := range sdkobjectpatch.SnapshotIter[Password](passwordsSnap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'passwords' snapshot: %w", err)
		}
		passwordsByName[password.Name] = password
		allPasswords = append(allPasswords, password)
	}

	groupsSnap := input.Snapshots.Get("groups")
	for group, err := range sdkobjectpatch.SnapshotIter[DexGroup](groupsSnap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'groups' snapshot: %v", err)
		}

		err = makeUserGroupsMap(groupsSnap, group.Spec.Name, []string{}, mapOfUsersToGroups, make(map[string]bool))
		if err != nil {
			return fmt.Errorf("error while make user groups map for group %s: %v", group.Spec.Name, err)
		}
	}

	// Names of the Password objects that should exist after this reconcile.
	// Anything else we own (heritage: deckhouse) is an orphan to delete - this
	// covers both deleted users and users whose email (hence object name) changed.
	expectedPasswordNames := make(map[string]struct{}, len(input.Snapshots.Get("users")))

	for dexUser, err := range sdkobjectpatch.SnapshotIter[DexUser](input.Snapshots.Get("users")) {
		if err != nil {
			return fmt.Errorf("cannot convert user to dex user: cannot iterate over 'users' snapshot: %v", err)
		}

		var groups []string
		for g := range mapOfUsersToGroups[dexUser.Name] {
			groups = append(groups, g)
		}
		groups = set.New(groups...).Slice()

		dexUser.Spec.Groups = groups

		dexUser.Spec.UserID = dexUser.Name

		email := strings.ToLower(dexUser.Spec.Email)
		encodedName := encoding.ToFnvLikeDex(email)
		expectedPasswordNames[encodedName] = struct{}{}

		existingPassword, passwordExists := passwordsByName[encodedName]

		// The raw bcrypt from User.spec.password seeds a brand-new Password only.
		// Capture it before we possibly overwrite the rendered value below.
		rawPassword := dexUser.Spec.Password

		// IMPORTANT:
		// Dex updates Password objects when a user changes password in the UI.
		// If we echoed User.spec.password into the internal values we would expose
		// a stale hash. Prefer the live Password.hash when a Password already exists.
		if passwordExists && existingPassword.Hash != "" {
			dexUser.Spec.Password = existingPassword.Hash
		}

		var expireAt string

		if dexUser.Status.ExpireAt == "" && dexUser.Spec.TTL != "" {
			parsedDuration, err := time.ParseDuration(dexUser.Spec.TTL)
			if err != nil {
				// A malformed TTL must never fail the whole hook. Returning an
				// error here would make addon-operator retry the task forever and
				// block the /modules/user-authn queue (and OnBeforeHelm with it),
				// breaking every other user too. The CRD validates the format, so
				// in practice this only triggers on out-of-range values; log it and
				// treat the user as having no TTL instead of taking the queue down.
				input.Logger.Warn("Ignoring invalid user TTL",
					slog.String("user", dexUser.Name),
					slog.String("ttl", dexUser.Spec.TTL),
					slog.Any("error", err))
				expireAt = dexUser.Status.ExpireAt
			} else {
				expireAt = now.Add(parsedDuration).Format(time.RFC3339)
				dexUser.Spec.TTL = ""
			}
		} else {
			expireAt = dexUser.Status.ExpireAt
		}

		lock := DexUserLock{}
		if passwordExists && existingPassword.LockedUntil != nil && existingPassword.LockedUntil.After(now) {
			lock = DexUserLock{
				State:   true,
				Reason:  ptr.To(PasswordPolicyLockout),
				Message: ptr.To("Locked due to too many failed login attempts"),
				Until:   ptr.To(existingPassword.LockedUntil.Format(time.RFC3339)),
			}

			// If this annotation exists - we consider lock was set by administrator.
			if _, ok := existingPassword.Annotations[PasswordAnnotationLockedByAdministrator]; ok {
				lock.Reason = ptr.To(LockedByAdministrator)
				lock.Message = ptr.To("Locked by administrator")
			}
		} else if _, ok := existingPassword.Annotations[PasswordAnnotationLockedByAdministrator]; ok {
			// In this case we have expired or unexisted lock and saved from previous lock annotation.
			// For sure we need to delete it (and persist the change to the cluster).
			input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				annotations := obj.GetAnnotations()
				if annotations == nil {
					return obj, nil
				}
				delete(annotations, PasswordAnnotationLockedByAdministrator)
				obj.SetAnnotations(annotations)
				return obj, nil
			}, "dex.coreos.com/v1", "Password", existingPassword.Namespace, existingPassword.Name)
			delete(existingPassword.Annotations, PasswordAnnotationLockedByAdministrator)
		}
		dexUser.Status.Lock = lock

		// Reconcile the Dex Password object directly instead of rendering it via
		// Helm. This makes a freshly created user usable immediately (no wait for a
		// module Helm reconcile) and lets us stamp hashUpdatedAt on creation, which
		// the password rotation policy relies on. We never touch Dex-managed runtime
		// fields (hash after a user change, hashUpdatedAt, previousHashes,
		// incorrectPasswordLoginAttempts, lockedUntil, requireResetHashOnNextSuccLogin)
		// on an existing object.
		if passwordExists {
			reconcileExistingPassword(input, existingPassword, dexUser.Name, email, groups)
		} else {
			input.PatchCollector.CreateIfNotExists(newPasswordObject(encodedName, dexUser.Name, email, rawPassword, groups, now))
		}

		users = append(users, DexUserInternalValues{
			Name:        dexUser.Name,
			EncodedName: encodedName,
			Spec:        dexUser.Spec,
			Status:      dexUser.Status,
			ExpireAt:    expireAt,
		})

		patch := userStatusPatch{
			Groups: groups,
			Lock:   lock,
		}
		if expireAt != "" {
			patch.ExpireAt = expireAt
		}
		patchMap := map[string]any{
			"status": patch,
		}

		input.Logger.Info("Sync user status", slog.Any("patch", patch))
		input.PatchCollector.PatchWithMerge(patchMap, "deckhouse.io/v1", "User", "", dexUser.Name, object_patch.WithSubresource("/status"))
	}

	// Delete Password objects we own that no user references anymore.
	for _, password := range allPasswords {
		if _, expected := expectedPasswordNames[password.Name]; expected {
			continue
		}
		if !isModuleManagedPassword(password) {
			continue
		}
		input.Logger.Info("Deleting orphaned Password",
			slog.String("name", password.Name), slog.String("username", password.Username))
		input.PatchCollector.Delete("dex.coreos.com/v1", "Password", password.Namespace, password.Name)
	}

	input.Values.Set("userAuthn.internal.dexUsersCRDs", users)
	return nil
}

// isModuleManagedPassword reports whether a Password object is owned by this
// module (and is therefore safe to delete when orphaned). Both the legacy
// Helm-rendered objects and the ones this hook now creates carry the standard
// deckhouse heritage label.
func isModuleManagedPassword(p Password) bool {
	return p.Labels["heritage"] == "deckhouse"
}

// passwordObjectLabels mirrors the labels the previous Helm template applied via
// helm_lib_module_labels (list . (dict "app" "dex")), so tooling that lists
// Password objects by label (e.g. the Console) keeps working.
func passwordObjectLabels() map[string]any {
	return map[string]any{
		"heritage": "deckhouse",
		"module":   "user-authn",
		"app":      "dex",
	}
}

// encodePasswordHash matches the historical Helm template behaviour: a bcrypt
// hash (it starts with "$2") is stored base64-encoded in Password.hash; anything
// else is stored verbatim.
func encodePasswordHash(rawPassword string) string {
	if strings.HasPrefix(rawPassword, "$2") {
		return base64.StdEncoding.EncodeToString([]byte(rawPassword))
	}
	return rawPassword
}

// newPasswordObject builds a brand-new Dex Password object for a user. It stamps
// hashUpdatedAt with the current time so the rotation policy starts the clock at
// creation instead of treating the password as ancient (zero time), which would
// force a password change on the very first login.
func newPasswordObject(encodedName, username, email, rawPassword string, groups []string, now time.Time) *unstructured.Unstructured {
	metadata := map[string]any{
		"name":      encodedName,
		"namespace": dexNamespace,
		"labels":    passwordObjectLabels(),
		"annotations": map[string]any{
			helmResourcePolicyAnnotation: helmResourcePolicyKeep,
		},
	}

	obj := map[string]any{
		"apiVersion":    "dex.coreos.com/v1",
		"kind":          "Password",
		"metadata":      metadata,
		"email":         email,
		"username":      username,
		"userID":        username,
		"hash":          encodePasswordHash(rawPassword),
		"hashUpdatedAt": now.UTC().Format(time.RFC3339),
	}
	if len(groups) > 0 {
		// unstructured.Unstructured must only hold []interface{} (not []string),
		// otherwise the runtime converter panics.
		groupsAny := make([]any, len(groups))
		for i, g := range groups {
			groupsAny[i] = g
		}
		obj["groups"] = groupsAny
	}

	return &unstructured.Unstructured{Object: obj}
}

// reconcileExistingPassword updates only the fields Deckhouse owns (email,
// username, userID, groups) plus the heritage labels and the resource-policy:keep
// annotation. It deliberately never sends hash, hashUpdatedAt, previousHashes,
// incorrectPasswordLoginAttempts, lockedUntil or requireResetHashOnNextSuccLogin,
// so a JSON merge patch leaves those Dex-managed runtime fields intact. The patch
// is skipped entirely when nothing it owns has changed, to avoid churn on every
// cron tick.
func reconcileExistingPassword(input *go_hook.HookInput, existing Password, username, email string, groups []string) {
	hasKeepAnnotation := existing.Annotations[helmResourcePolicyAnnotation] == helmResourcePolicyKeep
	if hasKeepAnnotation &&
		existing.Email == email &&
		existing.Username == username &&
		existing.UserID == username &&
		equalStringSets(existing.Groups, groups) {
		return
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

	input.PatchCollector.PatchWithMerge(patch, "dex.coreos.com/v1", "Password", existing.Namespace, existing.Name)
}

// equalStringSets reports whether a and b contain the same elements, ignoring
// order. Group lists are already de-duplicated by the caller.
func equalStringSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]struct{}, len(a))
	for _, v := range a {
		seen[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := seen[v]; !ok {
			return false
		}
	}
	return true
}

func applyDexGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var group = &DexGroup{}
	err := sdk.FromUnstructured(obj, group)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}
	return group, nil
}

func applyDexUserFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var user = &DexUser{}
	err := sdk.FromUnstructured(obj, user)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}
	return user, nil
}

func applyPasswordFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var password = &Password{}
	err := sdk.FromUnstructured(obj, password)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}
	return password, nil
}

func findGroup(groups []pkg.Snapshot, groupName string) (*DexGroup, error) {
	for group, err := range sdkobjectpatch.SnapshotIter[DexGroup](groups) {
		if err != nil {
			return nil, fmt.Errorf("cannot iterate over 'groups' snapshot: %v", err)
		}

		if group.Spec.Name == groupName {
			return &group, err
		}
	}
	return nil, nil
}

func makeUserGroupsMap(
	groups []pkg.Snapshot,
	targetGroup string,
	accumulatedGroupList []string,
	mapOfUsersToGroups map[string]map[string]bool,
	visited map[string]bool,
) error {
	if len(groups) == 0 {
		return nil
	}
	// If this group has already been visited, exit to prevent infinite recursion
	if visited[targetGroup] {
		return nil
	}
	visited[targetGroup] = true

	group, err := findGroup(groups, targetGroup)
	if err != nil {
		return fmt.Errorf("error while find group %s: %v", targetGroup, err)
	}
	if group == nil {
		return nil
	}

	skipAddGroup := false
	for _, g := range accumulatedGroupList {
		if g == targetGroup {
			skipAddGroup = true
		}
	}
	if !skipAddGroup {
		accumulatedGroupList = append(accumulatedGroupList, targetGroup)
	}
	for _, member := range group.Spec.Members {
		switch member.Kind {
		case "User":
			if mapOfUsersToGroups[member.Name] == nil {
				mapOfUsersToGroups[member.Name] = map[string]bool{}
			}
			for _, g := range accumulatedGroupList {
				mapOfUsersToGroups[member.Name][g] = true
			}
		case "Group":
			err := makeUserGroupsMap(groups, member.Name, accumulatedGroupList, mapOfUsersToGroups, visited)
			if err != nil {
				return fmt.Errorf("error while make user groups map for group %s: %v", member.Name, err)
			}
		}
	}
	return nil
}
