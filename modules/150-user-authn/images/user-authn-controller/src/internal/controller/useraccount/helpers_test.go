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
	"testing"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/naming"
)

func TestUniqueRequests(t *testing.T) {
	t.Parallel()

	req := func(name string) reconcile.Request {
		return reconcile.Request{NamespacedName: types.NamespacedName{Name: name}}
	}

	if got := uniqueRequests(nil); got != nil {
		t.Errorf("nil = %v, want nil", got)
	}
	single := []reconcile.Request{req("a")}
	if got := uniqueRequests(single); len(got) != 1 || got[0].Name != "a" {
		t.Errorf("single = %v", got)
	}

	got := uniqueRequests([]reconcile.Request{req("a"), req("b"), req("a")})
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("dedup = %v, want a,b", got)
	}
}

func TestApplyLabelsAndOurLabelsEqual(t *testing.T) {
	t.Parallel()

	desired := map[string]string{
		v1alpha1.LabelKind:        v1alpha1.KindLocal,
		v1alpha1.LabelConnectorID: naming.LocalConnectorID,
		v1alpha1.LabelLocked:      "true",
	}
	merged := applyLabels(nil, desired)
	if merged[v1alpha1.LabelKind] != v1alpha1.KindLocal {
		t.Fatalf("applyLabels(nil) = %v", merged)
	}
	merged["keep"] = "x"
	again := applyLabels(merged, desired)
	if again["keep"] != "x" {
		t.Error("applyLabels dropped unrelated label")
	}
	if !ourLabelsEqual(again, desired) {
		t.Error("ourLabelsEqual want true")
	}
	again[v1alpha1.LabelLocked] = "false"
	if ourLabelsEqual(again, desired) {
		t.Error("ourLabelsEqual want false after locked change")
	}
}

func TestOwnerEqual(t *testing.T) {
	t.Parallel()

	if !ownerEqual(nil, nil) {
		t.Error("empty owners should be equal")
	}
	want := &metav1.OwnerReference{
		APIVersion:         "deckhouse.io/v1",
		Kind:               "User",
		Name:               "jane",
		UID:                "uid-1",
		Controller:         boolPtr(true),
		BlockOwnerDeletion: boolPtr(false),
	}
	if ownerEqual(nil, want) {
		t.Error("nil existing vs desired should not be equal")
	}
	if !ownerEqual([]metav1.OwnerReference{*want}, want) {
		t.Error("matching owner should be equal")
	}
	if ownerEqual([]metav1.OwnerReference{*want, *want}, want) {
		t.Error("two owners should not equal one desired")
	}
	other := *want
	other.Name = "other"
	if ownerEqual([]metav1.OwnerReference{other}, want) {
		t.Error("different name should not be equal")
	}
}

func TestBoolPtrValAndMetaTimeEqual(t *testing.T) {
	t.Parallel()

	if boolPtrVal(nil) {
		t.Error("nil pointer should be false")
	}
	if !boolPtrVal(boolPtr(true)) {
		t.Error("true pointer should be true")
	}
	if boolPtrVal(boolPtr(false)) {
		t.Error("false pointer should be false")
	}

	if !metaTimeEqual(nil, nil) {
		t.Error("nil times should be equal")
	}
	now := metav1.NewTime(time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC))
	if metaTimeEqual(&now, nil) {
		t.Error("nil vs value should not be equal")
	}
	offset := metav1.NewTime(now.Time.In(time.FixedZone("plus3", 3*3600)))
	if !metaTimeEqual(&now, &offset) {
		t.Error("same instant in different zones should be equal")
	}
}

func TestProjectHelpers(t *testing.T) {
	t.Parallel()

	if localNameInput("a@b.c", "user") != "a@b.c" {
		t.Error("email should win")
	}
	if localNameInput("", "user") != "user" {
		t.Error("username fallback")
	}
	if isProjectablePassword(passwordView{}) {
		t.Error("empty password is not projectable")
	}
	if !isProjectablePassword(passwordView{Username: "jane"}) {
		t.Error("username-only should be projectable")
	}
	if isExternalCandidate(sessionView{Email: "a@b.c", ConnID: naming.LocalConnectorID}) {
		t.Error("local connector is not external")
	}
	if !isExternalCandidate(sessionView{Email: "a@b.c", ConnID: "ldap"}) {
		t.Error("ldap session should be external")
	}
	if isExternalCandidate(sessionView{ConnID: "ldap"}) {
		t.Error("missing email is not external")
	}
	if isLockableProvider("Github") {
		t.Error("github is not lockable")
	}
	if !isLockableProvider(providerTypeLDAP) {
		t.Error("ldap should be lockable")
	}

	users := []userView{{Name: "jane", Email: "Jane@Example.com"}}
	if matchUserByEmail(users, "") != nil {
		t.Error("empty email")
	}
	got := matchUserByEmail(users, "jane@example.com")
	if got == nil || got.Name != "jane" {
		t.Errorf("case-insensitive match = %+v", got)
	}
}

func TestCouldBeLocalAccountNameAndSessionNameFromAccount(t *testing.T) {
	t.Parallel()

	if !couldBeLocalAccountName(naming.LocalName("jane@example.com")) {
		t.Error("local-prefixed name should be local")
	}
	if !couldBeLocalAccountName(naming.ToFnvLikeDex("long-email")) {
		t.Error("hash-only name can be a truncated local account")
	}
	if couldBeLocalAccountName(naming.ExternalName("corp-ldap", "alice")) {
		t.Error("external name must not trigger password list fallback")
	}

	connID := "corp-ldap"
	userID := "alice"
	account := naming.ExternalName(connID, userID)
	if got := sessionNameFromAccount(account); got != naming.OfflineTokenName(userID, connID) {
		t.Errorf("sessionNameFromAccount(%q) = %q, want OfflineTokenName", account, got)
	}
	hash := naming.OfflineTokenName(userID, connID)
	if got := sessionNameFromAccount(hash); got != hash {
		t.Errorf("truncated account name = %q, want %q", got, hash)
	}
}

func TestMapPasswordUserAndSession(t *testing.T) {
	t.Parallel()

	r := &Reconciler{log: logr.Discard(), now: time.Now}

	if reqs := r.mapPassword(t.Context(), nil); reqs != nil {
		t.Errorf("typed-nil object: %v", reqs)
	}

	pw := passwordUnstructured("pw", map[string]any{"email": "jane@example.com", "username": "jane"})
	reqs := r.mapPassword(t.Context(), pw)
	if len(reqs) != 1 || reqs[0].Name != naming.LocalName("jane@example.com") {
		t.Errorf("mapPassword = %v", reqs)
	}

	emptyPW := passwordUnstructured("pw-empty", map[string]any{})
	if reqs := r.mapPassword(t.Context(), emptyPW); reqs != nil {
		t.Errorf("empty password mapped: %v", reqs)
	}

	user := userUnstructured("jane", "jane@example.com", nil, "")
	reqs = r.mapUser(t.Context(), user)
	if len(reqs) != 1 || reqs[0].Name != naming.LocalName("jane@example.com") {
		t.Errorf("mapUser = %v", reqs)
	}

	sess := sessionUnstructured("sess", map[string]any{"email": "ext@example.com", "userID": "u1", "connID": "ldap"})
	reqs = r.mapOfflineSessions(t.Context(), sess)
	if len(reqs) != 1 || reqs[0].Name != naming.ExternalName("ldap", "u1") {
		t.Errorf("mapOfflineSessions = %v", reqs)
	}

	localSess := sessionUnstructured("local", map[string]any{"email": "a@b.c", "userID": "u", "connID": ""})
	if reqs := r.mapOfflineSessions(t.Context(), localSess); reqs != nil {
		t.Errorf("empty connID mapped: %v", reqs)
	}
}

func TestMapPasswordRejectsNonUnstructured(t *testing.T) {
	t.Parallel()

	r := &Reconciler{log: logr.Discard(), now: time.Now}
	if reqs := r.mapPassword(t.Context(), &v1alpha1.UserAccount{}); reqs != nil {
		t.Errorf("typed UserAccount mapped: %v", reqs)
	}
}

func TestDecodePasswordErrors(t *testing.T) {
	t.Parallel()

	if _, err := decodePassword(nil); err == nil {
		t.Fatal("nil object: want error")
	}

	badAttempts := passwordUnstructured("pw", map[string]any{
		"email":                          "a@b.c",
		"incorrectPasswordLoginAttempts": "nope",
	})
	if _, err := decodePassword(badAttempts); err == nil {
		t.Fatal("bad attempts: want error")
	}

	badUntil := passwordUnstructured("pw", map[string]any{
		"email":       "a@b.c",
		"lockedUntil": "not-a-time",
	})
	if _, err := decodePassword(badUntil); err == nil {
		t.Fatal("bad lockedUntil: want error")
	}
}

func TestDecodeSessionAndUserErrors(t *testing.T) {
	t.Parallel()

	if _, err := decodeSession(nil); err == nil {
		t.Fatal("nil session: want error")
	}
	badSess := sessionUnstructured("s", map[string]any{"incorrectPasswordLoginAttempts": "x"})
	if _, err := decodeSession(badSess); err == nil {
		t.Fatal("bad session attempts: want error")
	}

	if _, err := decodeUser(nil); err == nil {
		t.Fatal("nil user: want error")
	}
	badUser := userUnstructured("u", "a@b.c", nil, "not-a-time")
	if _, err := decodeUser(badUser); err == nil {
		t.Fatal("bad expireAt: want error")
	}

	if _, err := decodeProvider(nil); err == nil {
		t.Fatal("nil provider: want error")
	}
}
