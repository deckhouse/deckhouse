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

package identitycleanup

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/revoke"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

var now = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

const retention = 30 * 24 * time.Hour

type fakeDeleter struct {
	pairs []string
	err   error
}

func (f *fakeDeleter) DeletePair(_ context.Context, userID, connectorID string) (revoke.PairDeleted, error) {
	if f.err != nil {
		return revoke.PairDeleted{}, f.err
	}
	f.pairs = append(f.pairs, userID+"@"+connectorID)
	return revoke.PairDeleted{RefreshTokens: 1}, nil
}

// identityJSON is a UserIdentity as the API server serves it: the json of the kubernetes storage type of Dex.
func identityJSON(userID, connectorID, lastLogin, claimsRefreshedAt, createdAt string) string {
	object := map[string]any{
		"apiVersion": "dex.coreos.com/v1",
		"kind":       "UserIdentity",
		"metadata": map[string]any{
			"name": useridentity.Name(userID, connectorID), "namespace": naming.DexNamespace,
			"uid": "uid-" + userID + connectorID, "resourceVersion": "12",
			"creationTimestamp": "2026-01-01T00:00:00Z",
		},
		"userID":      userID,
		"connectorID": connectorID,
		"claims":      map[string]any{"userID": userID, "email": userID + "@example.com"},
		"mfaSecrets":  map[string]any{"totp": map[string]any{"confirmed": true}},
	}
	for key, value := range map[string]string{"lastLogin": lastLogin, "claimsRefreshedAt": claimsRefreshedAt, "createdAt": createdAt} {
		if value != "" {
			object[key] = value
		}
	}
	raw, err := json.Marshal(object)
	if err != nil {
		panic(err)
	}
	return string(raw)
}

// orphaned is an identity marked by an earlier pass as without a provider since the time given.
func orphaned(obj *unstructured.Unstructured, since string) *unstructured.Unstructured {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[useridentity.OrphanedSinceAnnotation] = since
	obj.SetAnnotations(annotations)
	return obj
}

func identity(t *testing.T, raw string) *unstructured.Unstructured {
	t.Helper()
	obj := &unstructured.Unstructured{}
	if err := json.Unmarshal([]byte(raw), &obj.Object); err != nil {
		t.Fatal(err)
	}
	return obj
}

func provider(name string, spec map[string]any) *unstructured.Unstructured {
	obj := controller.Object(controller.DexProviderGVK)
	obj.SetName(name)
	obj.Object["spec"] = spec
	return obj
}

func tokenExchange(name, connectorID string) *unstructured.Unstructured {
	obj := controller.Object(controller.TokenExchangeProviderGVK)
	obj.SetName(name)
	obj.Object["status"] = map[string]any{"connectorID": connectorID}
	return obj
}

type setup struct {
	c       client.Client
	at      time.Time
	deleter *fakeDeleter
	cleanup *Cleanup
	deletes []client.Object
	// preconditions are those of the last delete of an identity.
	preconditions *metav1.Preconditions
}

func newSetup(t *testing.T, objs ...client.Object) *setup {
	t.Helper()
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{controller.DexProviderGVK.GroupVersion(), controller.UserIdentityGVK.GroupVersion(), controller.TokenExchangeProviderGVK.GroupVersion()})
	mapper.Add(controller.DexProviderGVK, meta.RESTScopeRoot)
	mapper.Add(controller.TokenExchangeProviderGVK, meta.RESTScopeRoot)
	mapper.Add(controller.UserIdentityGVK, meta.RESTScopeNamespace)
	mapper.Add(controller.RevocationGVK, meta.RESTScopeNamespace)

	s := &setup{deleter: &fakeDeleter{}, at: now}
	base := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithRESTMapper(mapper).WithObjects(objs...).Build()
	s.c = interceptor.NewClient(base, interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			s.deletes = append(s.deletes, obj.DeepCopyObject().(client.Object))
			deleteOptions := &client.DeleteOptions{}
			deleteOptions.ApplyOptions(opts)
			s.preconditions = deleteOptions.Preconditions
			return c.Delete(ctx, obj, opts...)
		},
	})
	s.cleanup = New(s.c, s.c, s.deleter, logr.Discard(), func() time.Time { return s.at }, retention)
	return s
}

func (s *setup) exists(t *testing.T, name string) bool {
	t.Helper()
	err := s.c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, controller.Object(controller.UserIdentityGVK))
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	return err == nil
}

func (s *setup) run(t *testing.T) int {
	t.Helper()
	deleted, err := s.cleanup.Run(t.Context())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return deleted
}

func ago(d time.Duration) string { return now.Add(-d).Format(time.RFC3339Nano) }

// DeletePair returns an error when the OfflineSessions changed under it. The identity stays, so the next pass
// can try again; deleting it would leave the OfflineSessions, and the migration would recreate the identity from it.
func TestRun_KeepsTheIdentityWhenWhatDexKeepsCannotBeDeleted(t *testing.T) {
	t.Parallel()

	old := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention+time.Hour), "", ago(retention*3))), ago(retention*2))
	s := newSetup(t, old)
	s.deleter.err = errors.New("delete offlinesessions: conflict")

	deleted, err := s.cleanup.Run(t.Context())
	if err == nil {
		t.Fatal("Run: want the error of the deletion")
	}
	if deleted != 0 || !s.exists(t, old.GetName()) {
		t.Fatalf("deleted %d; the identity must stay for the next pass", deleted)
	}
}

func TestRun_DeletesTheIdentityOfADeletedProviderIdleLongerThanTheRetention(t *testing.T) {
	t.Parallel()

	old := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention+time.Hour), ago(retention+2*time.Hour), ago(retention*3))), ago(retention*2))
	s := newSetup(t, old)

	if got := s.run(t); got != 1 {
		t.Fatalf("deleted %d identities, want 1", got)
	}
	if s.exists(t, old.GetName()) {
		t.Error("the identity is still there")
	}
	if !slices.Equal(s.deleter.pairs, []string{"ann@corp-ldap"}) {
		t.Errorf("what Dex keeps of the pair was deleted for %v", s.deleter.pairs)
	}
}

func TestRun_KeepsTheIdentityOfAProviderThatExists(t *testing.T) {
	t.Parallel()

	old := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*2), "", ago(retention*3))), ago(retention*2))
	s := newSetup(t, old, provider("corp-ldap", map[string]any{"type": "LDAP"}))

	if got := s.run(t); got != 0 || !s.exists(t, old.GetName()) {
		t.Fatalf("deleted %d; the identity of an existing provider is gone", got)
	}
	if len(s.deleter.pairs) != 0 {
		t.Errorf("what Dex keeps was deleted for %v", s.deleter.pairs)
	}
}

// A provider that is turned off still exists, and its users keep their second factors and locks.
func TestRun_KeepsTheIdentityOfADisabledProvider(t *testing.T) {
	t.Parallel()

	old := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*2), "", ago(retention*3))), ago(retention*2))
	s := newSetup(t, old, provider("corp-ldap", map[string]any{"type": "LDAP", "enabled": false}))

	if got := s.run(t); got != 0 || !s.exists(t, old.GetName()) {
		t.Fatalf("deleted %d; the identity of a disabled provider is gone", got)
	}
}

func TestRun_KeepsTheIdentityUntilTheRetentionHasPassed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		lastLogin    string
		claimsAt     string
		createdAt    string
		wantDeletion bool
	}{
		{name: "the last login is within the retention", lastLogin: ago(retention - time.Hour), createdAt: ago(retention * 3)},
		{name: "the claims were read within the retention", lastLogin: ago(retention * 2), claimsAt: ago(retention - time.Hour), createdAt: ago(retention * 3)},
		{name: "the account was created within the retention", createdAt: ago(time.Hour)},
		{name: "everything is older than the retention", lastLogin: ago(retention + time.Minute), claimsAt: ago(retention + time.Hour), createdAt: ago(retention * 3), wantDeletion: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obj := orphaned(identity(t, identityJSON("ann", "corp-ldap", tc.lastLogin, tc.claimsAt, tc.createdAt)), ago(retention*2))
			s := newSetup(t, obj)
			got := s.run(t)
			if (got == 1) != tc.wantDeletion || s.exists(t, obj.GetName()) == tc.wantDeletion {
				t.Fatalf("deleted %d, want deletion=%v", got, tc.wantDeletion)
			}
		})
	}
}

// An identity with no time at all is as old as its object: the creation timestamp stands for the activity.
func TestRun_AnIdentityWithoutTimesGoesByItsCreationTimestamp(t *testing.T) {
	t.Parallel()

	obj := orphaned(identity(t, identityJSON("ann", "corp-ldap", "", "", "")), ago(retention*2))
	s := newSetup(t, obj)
	if got := s.run(t); got != 1 {
		t.Fatalf("deleted %d, want 1: the object was created in January", got)
	}
}

func TestRun_NeverTouchesLocalUsersOrTheConnectorsOfTokenExchangeProviders(t *testing.T) {
	t.Parallel()

	local := identity(t, identityJSON("jane", naming.LocalConnectorID, ago(retention*2), "", ago(retention*3)))
	exchange := identity(t, identityJSON("ci", "tokenexchange-ab12", ago(retention*2), "", ago(retention*3)))
	s := newSetup(t, local, exchange, tokenExchange("ci-tokens", "tokenexchange-ab12"))

	if got := s.run(t); got != 0 {
		t.Fatalf("deleted %d identities, want none", got)
	}
	for _, name := range []string{local.GetName(), exchange.GetName()} {
		if !s.exists(t, name) {
			t.Errorf("the identity %s was deleted", name)
		}
	}
}

// The identity goes with the UID and the resourceVersion read, so a login that Dex wrote in the meantime keeps it.
func TestRun_DeletesWithThePreconditionsOfTheObjectRead(t *testing.T) {
	t.Parallel()

	old := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*2), "", ago(retention*3))), ago(retention*2))
	s := newSetup(t, old)
	s.run(t)

	p := s.preconditions
	if p == nil || p.UID == nil || *p.UID == "" || p.ResourceVersion == nil || *p.ResourceVersion != "12" {
		t.Fatalf("the identity was deleted without the UID and the resourceVersion: %+v", p)
	}
}

// What Dex keeps of the pair goes first and the identity last: a failure leaves the identity for the next pass.
func TestRun_AFailureOnTheWayLeavesTheIdentity(t *testing.T) {
	t.Parallel()

	old := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*2), "", ago(retention*3))), ago(retention*2))
	s := newSetup(t, old)
	s.deleter.err = errors.New("the API server is away")

	if _, err := s.cleanup.Run(t.Context()); err == nil {
		t.Fatal("the failure was not reported")
	}
	if !s.exists(t, old.GetName()) {
		t.Error("the identity was deleted before what Dex keeps of the pair")
	}
}

func TestInterval(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		retention time.Duration
		want      time.Duration
	}{
		{retention: 5 * time.Second, want: 30 * time.Second},
		{retention: 10 * time.Minute, want: 150 * time.Second},
		{retention: retention, want: time.Hour},
	} {
		if got := New(nil, nil, nil, logr.Discard(), nil, tc.retention).interval(); got != tc.want {
			t.Errorf("interval for retention %v = %v, want %v", tc.retention, got, tc.want)
		}
	}
}

func (s *setup) get(t *testing.T, name string) *useridentity.Identity {
	t.Helper()
	obj := controller.Object(controller.UserIdentityGVK)
	if err := s.c.Get(t.Context(), types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj); err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	return useridentity.FromUnstructured(obj)
}

// The moment the provider went is not known, so the first pass marks the identity and the retention counts from the
// mark too: a provider deleted and created again does not lose the users who have not signed in for long.
func TestRun_CountsTheRetentionFromTheMarkOfTheFirstPass(t *testing.T) {
	t.Parallel()

	idle := identity(t, identityJSON("ann", "corp-ldap", ago(retention*3), "", ago(retention*4)))
	s := newSetup(t, idle)

	if got := s.run(t); got != 0 {
		t.Fatalf("the first pass deleted %d identities, want none", got)
	}
	since, marked := s.get(t, idle.GetName()).OrphanedSince()
	if !marked || !since.Equal(now) {
		t.Fatalf("the first pass marked %v (%v), want %v", since, marked, now)
	}
	// The mark keeps what Dex wrote: the secret of the second factor is still there.
	if s.get(t, idle.GetName()).Object().Object["mfaSecrets"] == nil {
		t.Fatal("the mark lost the second factor of the identity")
	}

	s.at = now.Add(retention - time.Hour)
	if got := s.run(t); got != 0 || !s.exists(t, idle.GetName()) {
		t.Fatalf("deleted %d before the retention passed since the mark", got)
	}

	s.at = now.Add(retention)
	if got := s.run(t); got != 1 || s.exists(t, idle.GetName()) {
		t.Fatalf("deleted %d once the retention passed since the mark, want 1", got)
	}
}

func TestRun_RemovesTheMarkWhenTheProviderIsBack(t *testing.T) {
	t.Parallel()

	marked := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*3), "", ago(retention*4))), ago(retention*2))
	s := newSetup(t, marked, provider("corp-ldap", map[string]any{"type": "LDAP"}))

	if got := s.run(t); got != 0 {
		t.Fatalf("deleted %d identities of an existing provider", got)
	}
	if _, still := s.get(t, marked.GetName()).OrphanedSince(); still {
		t.Fatal("the mark stayed with the provider back")
	}
}

// A lock ends when its time does, not when the provider goes: the identity of a locked account stays.
func TestRun_KeepsALockedAccount(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		set  func(*unstructured.Unstructured)
	}{
		{name: "blockedUntil in the future", set: func(obj *unstructured.Unstructured) {
			obj.Object["blockedUntil"] = now.Add(24 * time.Hour).Format(time.RFC3339)
		}},
		{name: "a lock of an administrator in the future", set: func(obj *unstructured.Unstructured) {
			obj.SetAnnotations(map[string]string{
				useridentity.OrphanedSinceAnnotation: ago(retention * 2),
				useridentity.AdminLockAnnotation:     now.Add(365 * 24 * time.Hour).Format(time.RFC3339),
			})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obj := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*3), "", ago(retention*4))), ago(retention*2))
			tc.set(obj)
			s := newSetup(t, obj)
			if got := s.run(t); got != 0 || !s.exists(t, obj.GetName()) {
				t.Fatalf("deleted %d identities of a locked account", got)
			}
		})
	}

	ended := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*3), "", ago(retention*4))), ago(retention*2))
	ended.Object["blockedUntil"] = ago(time.Hour)
	if got := newSetup(t, ended).run(t); got != 1 {
		t.Fatalf("deleted %d identities whose lock has ended, want 1", got)
	}
}

// A new identity starts at generation 0: while a Revocation of the pair is kept, the subscribers would refuse it.
func TestRun_KeepsAPairWithARevocation(t *testing.T) {
	t.Parallel()

	obj := orphaned(identity(t, identityJSON("ann", "corp-ldap", ago(retention*3), "", ago(retention*4))), ago(retention*2))
	rev := controller.Object(controller.RevocationGVK)
	rev.SetNamespace(naming.DexNamespace)
	rev.SetName("pair-ann")
	rev.SetLabels(map[string]string{revocation.PairLabel: naming.OfflineTokenName("ann", "corp-ldap")})
	other := controller.Object(controller.RevocationGVK)
	other.SetNamespace(naming.DexNamespace)
	other.SetName("pair-bob")
	other.SetLabels(map[string]string{revocation.PairLabel: naming.OfflineTokenName("bob", "corp-ldap")})

	s := newSetup(t, obj, rev, other)
	if got := s.run(t); got != 0 || !s.exists(t, obj.GetName()) {
		t.Fatalf("deleted %d identities whose pair has a revocation", got)
	}
	if got := newSetup(t, obj.DeepCopy(), other).run(t); got != 1 {
		t.Fatalf("deleted %d identities when only another pair has a revocation, want 1", got)
	}
}
