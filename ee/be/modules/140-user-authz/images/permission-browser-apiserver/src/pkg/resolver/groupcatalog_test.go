/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

func groupObject(objectName, rbacName string, members ...map[string]interface{}) *unstructured.Unstructured {
	converted := make([]interface{}, 0, len(members))
	for _, member := range members {
		converted = append(converted, member)
	}

	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "Group",
		"metadata":   map[string]interface{}{"name": objectName},
		"spec": map[string]interface{}{
			"name":    rbacName,
			"members": converted,
		},
	}}
}

func userObject(objectName, email string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "User",
		"metadata":   map[string]interface{}{"name": objectName},
		"spec":       map[string]interface{}{"email": email},
	}}
}

func member(kind, name string) map[string]interface{} {
	return map[string]interface{}{"kind": kind, "name": name}
}

func newTestCatalog(objs ...runtime.Object) *GroupCatalog {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{
		groupGVR: "GroupList",
		userGVR:  "UserList",
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objs...)

	return NewGroupCatalog(client)
}

func groupNames(groups []v1alpha1.ResolvedGroup) []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Name)
	}

	return names
}

func TestGroupCatalog_DirectMembership(t *testing.T) {
	catalog := newTestCatalog(
		groupObject("admins", "admins", member("User", "alice@example.com")),
		groupObject("others", "others", member("User", "bob@example.com")),
	)

	resolved, err := catalog.ResolveUserGroups(context.Background(), "alice@example.com")
	require.NoError(t, err)

	assert.Equal(t, []string{"admins"}, groupNames(resolved))
	assert.Equal(t, "catalog", resolved[0].Source)
	assert.Empty(t, resolved[0].Via)
}

func TestGroupCatalog_NestedMembership(t *testing.T) {
	// alice is in "developers"; "developers" is a member of "everyone", so the
	// permissions of "everyone" apply to alice too.
	catalog := newTestCatalog(
		groupObject("developers", "developers", member("User", "alice@example.com")),
		groupObject("everyone", "everyone", member("Group", "developers")),
		groupObject("root", "root", member("Group", "everyone")),
	)

	resolved, err := catalog.ResolveUserGroups(context.Background(), "alice@example.com")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"developers", "everyone", "root"}, groupNames(resolved))

	byName := map[string][]string{}
	for _, group := range resolved {
		byName[group.Name] = group.Via
	}
	assert.Equal(t, []string{"developers"}, byName["everyone"])
	assert.Equal(t, []string{"developers", "everyone"}, byName["root"])
}

func TestGroupCatalog_CycleDoesNotHang(t *testing.T) {
	catalog := newTestCatalog(
		groupObject("a", "a", member("User", "alice"), member("Group", "b")),
		groupObject("b", "b", member("Group", "a")),
	)

	resolved, err := catalog.ResolveUserGroups(context.Background(), "alice")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"a", "b"}, groupNames(resolved))
}

func TestGroupCatalog_MatchesUserByEmailAndObjectName(t *testing.T) {
	// Installations reference users either by email or by the User object name;
	// both must resolve to the same membership.
	catalog := newTestCatalog(
		userObject("alice", "alice@example.com"),
		groupObject("by-email", "by-email", member("User", "alice@example.com")),
		groupObject("by-name", "by-name", member("User", "alice")),
	)

	resolved, err := catalog.ResolveUserGroups(context.Background(), "alice@example.com")
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{"by-email", "by-name"}, groupNames(resolved))
}

func TestGroupCatalog_RbacNameWinsOverObjectName(t *testing.T) {
	catalog := newTestCatalog(
		groupObject("group-object", "Everyone", member("User", "alice")),
	)

	resolved, err := catalog.ResolveUserGroups(context.Background(), "alice")
	require.NoError(t, err)

	// spec.name is what lands in the token and in bindings.
	assert.Equal(t, []string{"Everyone"}, groupNames(resolved))
}

func TestGroupCatalog_DisabledWithoutClient(t *testing.T) {
	catalog := NewGroupCatalog(nil)

	assert.False(t, catalog.Available())

	resolved, err := catalog.ResolveUserGroups(context.Background(), "alice")
	require.NoError(t, err)
	assert.Empty(t, resolved)
}

func TestGroupCatalog_ServesFromCacheUntilTTLExpires(t *testing.T) {
	catalog := newTestCatalog(groupObject("admins", "admins", member("User", "alice")))

	first, err := catalog.ResolveUserGroups(context.Background(), "alice")
	require.NoError(t, err)
	require.Equal(t, []string{"admins"}, groupNames(first))

	// Swap the backing data: within the TTL the report must not pay for another
	// listing, which is what keeps repeated queries cheap.
	catalog.client = newTestCatalog(groupObject("changed", "changed", member("User", "alice"))).client

	cached, err := catalog.ResolveUserGroups(context.Background(), "alice")
	require.NoError(t, err)
	assert.Equal(t, []string{"admins"}, groupNames(cached))

	catalog.ttl = 0

	refreshed, err := catalog.ResolveUserGroups(context.Background(), "alice")
	require.NoError(t, err)
	assert.Equal(t, []string{"changed"}, groupNames(refreshed))
}
