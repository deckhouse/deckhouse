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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
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
	HashUpdatedAt                   string     `json:"hashUpdatedAt,omitempty"`
	PreviousHashes                  []string   `json:"previousHashes,omitempty"`
	IncorrectPasswordLoginAttempts  int        `json:"incorrectPasswordLoginAttempts,omitempty"`
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
			// Watch the namespace the Password objects live in. On a fresh
			// cluster it is created by the module's Helm release, which only runs
			// after this hook (OnBeforeHelm / OperatorStartup). We gate all
			// Password mutations on this namespace existing (see getDexUsers), and
			// this binding re-runs the hook the moment Helm creates it so Passwords
			// are reconciled immediately instead of waiting for the next cron tick.
			Name:       "namespace",
			ApiVersion: "v1",
			Kind:       "Namespace",
			NameSelector: &types.NameSelector{
				MatchNames: []string{dexNamespace},
			},
			FilterFunc: applyNamespaceNameFilter,
		},
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

	// dexNamespace is created by the module's Helm release, which runs after this
	// hook on a fresh cluster (OnBeforeHelm / OperatorStartup). Creating a Password
	// before the namespace exists fails with "namespaces d8-user-authn not found",
	// which fails the hook and blocks the /modules/user-authn queue - and with it
	// OnBeforeHelm - so Helm never creates the namespace, deadlocking the deploy.
	// Defer all Password mutations until the namespace exists; the "namespace"
	// binding re-runs the hook as soon as Helm creates it. Values and User status
	// patches are still written below so Helm can render and create the namespace.
	namespaceExists := len(input.Snapshots.Get("namespace")) > 0

	users := make([]DexUserInternalValues, 0, len(input.Snapshots.Get("users")))
	mapOfUsersToGroups := map[string]map[string]bool{}

	// Index existing Password objects by their object name. That name equals the
	// FNV-like Dex encoding of the lowercased email, which is exactly how Dex
	// resolves a password on login (cli.idToName(email)). Keying on it keeps our
	// create / update / delete decisions consistent with Dex's own lookup and
	// makes an email change (object name change) behave correctly.
	passwordsSnap := input.Snapshots.Get("passwords")
	passwordsByName := make(map[string]Password, len(passwordsSnap))
	// Secondary index keyed by the (stable) username. User.spec.email is mutable
	// while the Password object name is derived from the email, so after an email
	// change the by-name lookup misses even though a Password already exists for
	// the user. The by-username index lets us find that object and carry its live
	// Dex-managed state onto the renamed object instead of reseeding it.
	passwordsByUsername := make(map[string]Password, len(passwordsSnap))
	allPasswords := make([]Password, 0, len(passwordsSnap))
	for password, err := range sdkobjectpatch.SnapshotIter[Password](passwordsSnap) {
		if err != nil {
			return fmt.Errorf("cannot iterate over 'passwords' snapshot: %w", err)
		}
		passwordsByName[password.Name] = password
		if password.Username != "" {
			passwordsByUsername[password.Username] = password
		}
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

		// livePassword is the Password object that currently backs this user, if
		// any. When the by-name lookup misses we fall back to the username index:
		// this happens on an email change, where the object name (derived from the
		// email) no longer matches but a Password with the live hash still exists
		// under the previous name. Treating a rename as a brand-new user would
		// reseed the immutable creation-time User.spec.password and drop the live
		// hash the moment the old object is cleaned up as an orphan.
		livePassword := existingPassword
		isRename := false
		// Only fall back to a module-managed object: an unmanaged Password that
		// happens to share the username must never seed our recreated object.
		if !passwordExists {
			if prev, ok := passwordsByUsername[dexUser.Name]; ok && isModuleManagedPassword(prev) {
				livePassword = prev
				isRename = true
			}
		}

		// The raw bcrypt from User.spec.password seeds a brand-new Password only.
		// Capture it before we possibly overwrite the rendered value below.
		rawPassword := dexUser.Spec.Password

		// IMPORTANT:
		// Dex updates Password objects when a user changes password in the UI.
		// If we echoed User.spec.password into the internal values we would expose
		// a stale hash. Prefer the live Password.hash when a Password already
		// exists (either under the current name or, on a rename, under the old one).
		if livePassword.Hash != "" {
			dexUser.Spec.Password = livePassword.Hash
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
		if livePassword.LockedUntil != nil && livePassword.LockedUntil.After(now) {
			lock = DexUserLock{
				State:   true,
				Reason:  ptr.To(PasswordPolicyLockout),
				Message: ptr.To("Locked due to too many failed login attempts"),
				Until:   ptr.To(livePassword.LockedUntil.Format(time.RFC3339)),
			}

			// If this annotation exists - we consider lock was set by administrator.
			if _, ok := livePassword.Annotations[PasswordAnnotationLockedByAdministrator]; ok {
				lock.Reason = ptr.To(LockedByAdministrator)
				lock.Message = ptr.To("Locked by administrator")
			}
		} else if _, ok := livePassword.Annotations[PasswordAnnotationLockedByAdministrator]; ok && passwordExists {
			// In this case we have expired or unexisted lock and saved from previous lock annotation.
			// For sure we need to delete it (and persist the change to the cluster).
			// On a rename we skip this: the old object is about to be deleted as an
			// orphan and newPasswordObjectFromExisting only carries the annotation
			// forward while the lock is still active, so no stale annotation lingers.
			input.PatchCollector.PatchWithMutatingFunc(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				annotations := obj.GetAnnotations()
				if annotations == nil {
					return obj, nil
				}
				delete(annotations, PasswordAnnotationLockedByAdministrator)
				obj.SetAnnotations(annotations)
				return obj, nil
			}, "dex.coreos.com/v1", "Password", livePassword.Namespace, livePassword.Name)
			delete(livePassword.Annotations, PasswordAnnotationLockedByAdministrator)
		}
		dexUser.Status.Lock = lock

		// Reconcile the Dex Password object directly instead of rendering it via
		// Helm. This makes a freshly created user usable immediately (no wait for a
		// module Helm reconcile) and lets us stamp hashUpdatedAt on creation, which
		// the password rotation policy relies on. We never touch Dex-managed runtime
		// fields (hash after a user change, hashUpdatedAt, previousHashes,
		// incorrectPasswordLoginAttempts, lockedUntil, requireResetHashOnNextSuccLogin)
		// on an existing object.
		switch {
		case !namespaceExists:
			// Namespace not created by Helm yet: skip the Password mutation. The
			// user still lands in the internal values and its status is synced
			// below; the Password will be reconciled once the namespace appears.
		case passwordExists:
			reconcileExistingPassword(input, existingPassword, dexUser.Name, email, groups)
		case isRename:
			// Email (hence object name) changed. Recreate the object under the new
			// name while preserving the live Dex-managed state, so a rename never
			// reverts the password or resets the rotation clock. The old object is
			// removed by the orphan-cleanup pass below.
			input.PatchCollector.CreateIfNotExists(newPasswordObjectFromExisting(encodedName, dexUser.Name, email, groups, livePassword, now))
		default:
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

	// Delete Password objects we own that no user references anymore. Skipped
	// until the namespace exists: there are no Password objects to clean up yet,
	// and issuing deletes against a missing namespace would fail the hook.
	if namespaceExists {
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

// toUnstructuredSlice converts a []string into the []any that
// unstructured.Unstructured requires: holding a []string makes the runtime
// converter panic during DeepCopy.
func toUnstructuredSlice(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
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
		obj["groups"] = toUnstructuredSlice(groups)
	}

	return &unstructured.Unstructured{Object: obj}
}

// newPasswordObjectFromExisting builds a Password object under a new (email
// derived) name while carrying over the Dex-managed runtime state of the object
// it replaces. It is used when a user's email changes: because the object name
// is derived from the email, the old object cannot simply be patched - it has to
// be recreated under the new name. We copy hash, hashUpdatedAt, previousHashes,
// incorrectPasswordLoginAttempts, lockedUntil, requireResetHashOnNextSuccLogin
// and (only while the lock is still active) the locked-by-administrator
// annotation, so a rename does not revert the password to the immutable
// creation-time User.spec.password, reset the rotation clock, or clear the
// lockout state.
func newPasswordObjectFromExisting(encodedName, username, email string, groups []string, existing Password, now time.Time) *unstructured.Unstructured {
	annotations := map[string]any{
		helmResourcePolicyAnnotation: helmResourcePolicyKeep,
	}
	// Carry the locked-by-administrator marker only while the lock is genuinely
	// active. Copying a stale annotation onto the fresh object would misreport an
	// expired lock as administrator-set until the next reconcile cleaned it up.
	lockActive := existing.LockedUntil != nil && existing.LockedUntil.After(now)
	if v, ok := existing.Annotations[PasswordAnnotationLockedByAdministrator]; ok && lockActive {
		annotations[PasswordAnnotationLockedByAdministrator] = v
	}

	obj := map[string]any{
		"apiVersion": "dex.coreos.com/v1",
		"kind":       "Password",
		"metadata": map[string]any{
			"name":        encodedName,
			"namespace":   dexNamespace,
			"labels":      passwordObjectLabels(),
			"annotations": annotations,
		},
		"email":    email,
		"username": username,
		"userID":   username,
		// existing.Hash is already stored in Dex's on-disk (base64) form, so it is
		// copied verbatim rather than re-encoded.
		"hash":                            existing.Hash,
		"requireResetHashOnNextSuccLogin": existing.RequireResetHashOnNextSuccLogin,
	}
	if existing.HashUpdatedAt != "" {
		obj["hashUpdatedAt"] = existing.HashUpdatedAt
	}
	if existing.IncorrectPasswordLoginAttempts != 0 {
		// unstructured.Unstructured only accepts int64 for integer values; a plain
		// int makes the runtime converter panic during DeepCopy.
		obj["incorrectPasswordLoginAttempts"] = int64(existing.IncorrectPasswordLoginAttempts)
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

// applyNamespaceNameFilter snapshots only the namespace name: the hook just
// needs to know whether dexNamespace exists before touching Password objects.
func applyNamespaceNameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
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
