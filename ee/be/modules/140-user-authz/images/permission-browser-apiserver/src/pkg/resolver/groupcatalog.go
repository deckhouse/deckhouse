/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

// Group and User are user-authn resources. They are read through the dynamic
// client because they are CRDs: the module may be disabled, in which case the
// catalog degrades to "no groups resolved" instead of failing the report.
var (
	groupGVR = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "groups"}
	userGVR  = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "users"}
)

// catalogTTL bounds how stale the group graph may be. Group membership changes
// rarely and a report is a human-triggered query, so a short TTL keeps the
// common case free of API calls without hiding recent edits for long.
const catalogTTL = 30 * time.Second

// GroupCatalog resolves which groups a user belongs to, following nested group
// membership.
//
// Group membership lives in user-authn (Group resources with spec.members), not
// in RBAC: a RoleBinding to group "admins" applies to a user only because the
// identity provider puts "admins" into the user's token. Resolving the graph
// here is what makes a report for a user match what that user will actually be
// able to do.
type GroupCatalog struct {
	client dynamic.Interface
	ttl    time.Duration

	mu        sync.RWMutex
	fetchedAt time.Time
	groups    []catalogGroup
	users     []catalogUser
}

type catalogGroup struct {
	// rbacName is spec.name: the name that appears in tokens and bindings.
	rbacName string
	// objectName is metadata.name, used as a fallback identity and for
	// resolving nested references that point at the object rather than the
	// RBAC name.
	objectName string
	members    []catalogMember
}

type catalogMember struct {
	kind string
	name string
}

type catalogUser struct {
	objectName string
	email      string
}

// NewGroupCatalog creates a catalog backed by the dynamic client. A nil client
// disables resolution.
func NewGroupCatalog(client dynamic.Interface) *GroupCatalog {
	return &GroupCatalog{client: client, ttl: catalogTTL}
}

// Available reports whether group resolution can be attempted at all.
func (c *GroupCatalog) Available() bool {
	return c != nil && c.client != nil
}

// ResolveUserGroups returns the groups the user belongs to, directly or through
// nested groups, with the nesting chain recorded in Via.
//
// A failure to read the catalog is returned as an error but is not fatal for
// the caller: the report is still valid, it just does not account for
// group-derived grants, which the caller surfaces as a note.
func (c *GroupCatalog) ResolveUserGroups(ctx context.Context, userName string) ([]v1alpha1.ResolvedGroup, error) {
	if !c.Available() {
		return nil, nil
	}

	groups, users, err := c.snapshot(ctx)
	if err != nil {
		return nil, err
	}

	identities := userIdentities(userName, users)

	// Edge "member group name" -> parent group indexes. A member may reference
	// a group either by its RBAC name or by its object name, so both are keys.
	parentsByChild := make(map[string][]int, len(groups))
	for index, group := range groups {
		for _, member := range group.members {
			if member.kind == "Group" && member.name != "" {
				parentsByChild[member.name] = append(parentsByChild[member.name], index)
			}
		}
	}

	type discovery struct {
		index int
		via   []string
	}

	visited := make(map[int]struct{}, len(groups))
	queue := make([]discovery, 0, len(groups))

	for index, group := range groups {
		for _, member := range group.members {
			if member.kind == "User" && identities[member.name] {
				visited[index] = struct{}{}
				queue = append(queue, discovery{index: index})
				break
			}
		}
	}

	resolved := make([]v1alpha1.ResolvedGroup, 0, len(queue))

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		group := groups[current.index]
		name := group.rbacName
		if name == "" {
			name = group.objectName
		}
		if name == "" {
			continue
		}

		resolved = append(resolved, v1alpha1.ResolvedGroup{
			Name:   name,
			Source: v1alpha1.GroupSourceCatalog,
			Via:    current.via,
		})

		// A group that is itself a member of another group passes its members
		// up: being in B, where B belongs to A, grants A's permissions too.
		via := append(slices.Clone(current.via), name)
		for _, childName := range []string{group.rbacName, group.objectName} {
			if childName == "" {
				continue
			}
			for _, parent := range parentsByChild[childName] {
				if _, seen := visited[parent]; seen {
					continue
				}
				visited[parent] = struct{}{}
				queue = append(queue, discovery{index: parent, via: via})
			}
		}
	}

	slices.SortFunc(resolved, func(a, b v1alpha1.ResolvedGroup) int {
		return strings.Compare(a.Name, b.Name)
	})

	return resolved, nil
}

// userIdentities collects the names a Group member entry may use to reference
// the user: the requested name plus the email and object name of the matching
// User resource, since installations reference users either way.
func userIdentities(userName string, users []catalogUser) map[string]bool {
	identities := map[string]bool{userName: true}

	for _, user := range users {
		if user.email != userName && user.objectName != userName {
			continue
		}
		if user.email != "" {
			identities[user.email] = true
		}
		if user.objectName != "" {
			identities[user.objectName] = true
		}
	}

	return identities
}

// snapshot returns the cached catalog, refreshing it when stale. On a refresh
// failure the previous snapshot is served if there is one: a report built from
// slightly stale group membership is more useful than one with no groups.
func (c *GroupCatalog) snapshot(ctx context.Context) ([]catalogGroup, []catalogUser, error) {
	c.mu.RLock()
	fresh := time.Since(c.fetchedAt) < c.ttl
	groups, users := c.groups, c.users
	c.mu.RUnlock()

	if fresh {
		return groups, users, nil
	}

	newGroups, err := c.listGroups(ctx)
	if err != nil {
		if groups != nil {
			klog.V(4).Infof("GroupCatalog: refresh failed, serving cached snapshot: %v", err)
			return groups, users, nil
		}
		return nil, nil, err
	}

	// Users are only needed to reconcile email/name identities; a failure here
	// degrades matching but must not drop the group graph.
	newUsers, err := c.listUsers(ctx)
	if err != nil {
		klog.V(4).Infof("GroupCatalog: listing users failed, matching by name only: %v", err)
		newUsers = nil
	}

	c.mu.Lock()
	c.groups, c.users, c.fetchedAt = newGroups, newUsers, time.Now()
	c.mu.Unlock()

	return newGroups, newUsers, nil
}

func (c *GroupCatalog) listGroups(ctx context.Context) ([]catalogGroup, error) {
	list, err := c.client.Resource(groupGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// No Group CRD: user-authn is disabled or too old.
			return []catalogGroup{}, nil
		}
		return nil, fmt.Errorf("listing groups.deckhouse.io: %w", err)
	}

	groups := make([]catalogGroup, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		rbacName, _, _ := unstructured.NestedString(item.Object, "spec", "name")
		members, _, _ := unstructured.NestedSlice(item.Object, "spec", "members")

		group := catalogGroup{
			rbacName:   rbacName,
			objectName: item.GetName(),
			members:    make([]catalogMember, 0, len(members)),
		}

		for _, raw := range members {
			member, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			kind, _, _ := unstructured.NestedString(member, "kind")
			name, _, _ := unstructured.NestedString(member, "name")
			group.members = append(group.members, catalogMember{kind: kind, name: name})
		}

		groups = append(groups, group)
	}

	return groups, nil
}

func (c *GroupCatalog) listUsers(ctx context.Context) ([]catalogUser, error) {
	list, err := c.client.Resource(userGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []catalogUser{}, nil
		}
		return nil, fmt.Errorf("listing users.deckhouse.io: %w", err)
	}

	users := make([]catalogUser, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		email, _, _ := unstructured.NestedString(item.Object, "spec", "email")
		users = append(users, catalogUser{objectName: item.GetName(), email: email})
	}

	return users, nil
}
