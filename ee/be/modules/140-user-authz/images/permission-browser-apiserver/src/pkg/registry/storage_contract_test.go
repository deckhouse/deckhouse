/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/authorizer/composite"
	"permission-browser-apiserver/pkg/authorizer/multitenancy"
	"permission-browser-apiserver/pkg/authorizer/rbacadapter"
	"permission-browser-apiserver/pkg/authorizer/scopefilter"
)

// This file locks the BulkSAR HTTP contract: MT + real RBAC + IdentityRead +
// BindSubject via storage.Create. Optimization (scope snapshot, rule snapshot)
// must not change Allowed/Denied/Reason for any item.

const (
	contractEditorUser = "editor@example.io"
	contractSuperUser  = "super@example.io"
	contractAdminUser  = "pba-admin"
	filteredReadReason = "the namespace ACL answers this read with the accessible subset instead of a denial"
)

var carLabels = map[string]string{"heritage": "deckhouse", "module": "user-authz"}

func contractScope() staticResourceScope {
	return staticResourceScope{
		"/pods":                                  true,
		"/services":                              true,
		"/configmaps":                            true,
		"/secrets":                               true,
		"/namespaces":                            false,
		"/nodes":                                 false,
		"/persistentvolumes":                     false,
		"apps/deployments":                       true,
		"batch/jobs":                             true,
		"networking.k8s.io/ingresses":            true,
		"rbac.authorization.k8s.io/roles":        true,
		"rbac.authorization.k8s.io/clusterroles": false,
		"apiextensions.k8s.io/customresourcedefinitions": false,
		"deckhouse.io/projects":                          false,
	}
}

type servedResources map[string]struct{}

func (s servedResources) HasResource(group, resource string) bool {
	_, ok := s[group+"/"+resource]
	return ok
}

func projectRegistry() servedResources {
	return servedResources{"deckhouse.io/projects": {}}
}

func wildcardRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
		{NonResourceURLs: []string{"*"}, Verbs: []string{"*"}},
	}
}

// restrictedEditorRules is the e2e sec40 shape: namespaced workload reads,
// no nodes, no clusterroles, no projects (IdentityRead must supply projects).
func restrictedEditorRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"pods", "services", "configmaps", "secrets"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"get", "list", "watch"}},
		{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"get", "list", "watch"}},
	}
}

func clusterRole(name string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}, Rules: rules}
}

func carCRB(name, role, userName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: carLabels},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: userName}},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: role},
	}
}

func userCRB(name, role, userName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: userName}},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: role},
	}
}

func reviewerObjects() []runtime.Object {
	return []runtime.Object{
		clusterRole("pba-reviewer", wildcardRules()),
		userCRB("pba-reviewer", "pba-reviewer", contractAdminUser),
	}
}

func labeledNS() []runtime.Object {
	return []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "ns-in",
			Labels: map[string]string{"pba-perf": "true"},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-out"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-system"}},
	}
}

func ra(verb, group, resource, ns string) v1alpha1.SubjectAccessReviewRequest {
	return v1alpha1.SubjectAccessReviewRequest{
		ResourceAttributes: &v1alpha1.ResourceAttributes{
			Verb:      verb,
			Group:     group,
			Resource:  resource,
			Namespace: ns,
		},
	}
}

func consoleContractRequests() []v1alpha1.SubjectAccessReviewRequest {
	return []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "", "pods", ""),
		ra("watch", "", "pods", ""),
		ra("list", "", "services", ""),
		ra("list", "", "secrets", ""),
		ra("list", "apps", "deployments", ""),
		ra("list", "batch", "jobs", ""),
		ra("list", "networking.k8s.io", "ingresses", ""),
		ra("get", "", "nodes", ""),
		ra("list", "", "nodes", ""),
		ra("list", "", "namespaces", ""),
		ra("list", "rbac.authorization.k8s.io", "clusterroles", ""),
		ra("list", "apiextensions.k8s.io", "customresourcedefinitions", ""),
		ra("list", "deckhouse.io", "projects", ""),
		ra("create", "deckhouse.io", "projects", ""),
		ra("get", "", "pods", "ns-in"),
		ra("list", "", "pods", "ns-in"),
		ra("create", "", "pods", "ns-in"),
		ra("get", "apps", "deployments", "ns-in"),
		ra("get", "", "secrets", "ns-in"),
		ra("get", "", "pods", "ns-out"),
		ra("get", "", "secrets", "ns-out"),
		ra("create", "apps", "deployments", "ns-out"),
		ra("get", "", "pods", "kube-system"),
		ra("get", "", "pods", "d8-system"),
		ra("list", "example.com", "newcrds", ""),
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods", Subresource: "log"}},
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "pods", Subresource: "log", Namespace: "ns-in"}},
		{NonResourceAttributes: &v1alpha1.NonResourceAttributes{Verb: "get", Path: "/healthz"}},
	}
}

func sec40Requests() []v1alpha1.SubjectAccessReviewRequest {
	return []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "", "pods", ""),
		ra("get", "", "pods", "ns-in"),
		ra("get", "", "pods", "ns-out"),
		ra("get", "", "nodes", ""),
	}
}

type reviewOutcome struct {
	Allowed bool
	Denied  bool
	Reason  string
}

func outcomeFromResult(r v1alpha1.SubjectAccessReviewResult) reviewOutcome {
	return reviewOutcome{Allowed: r.Allowed, Denied: r.Denied, Reason: r.Reason}
}

func outcomeFromDecision(d authorizer.Decision, reason string) reviewOutcome {
	switch d {
	case authorizer.DecisionAllow:
		return reviewOutcome{Allowed: true, Reason: reason}
	case authorizer.DecisionDeny:
		return reviewOutcome{Denied: true, Reason: reason}
	default:
		return reviewOutcome{Reason: reason}
	}
}

type contractStack struct {
	storage *BulkSARStorage
	auth    authorizer.Authorizer
}

func newContractStack(t *testing.T, mtConfig string, objs []runtime.Object, scope staticResourceScope) contractStack {
	t.Helper()
	return newContractStackWithRegistry(t, mtConfig, objs, scope, projectRegistry())
}

func newContractStackWithRegistry(t *testing.T, mtConfig string, objs []runtime.Object, scope staticResourceScope, registry scopefilter.ResourceRegistry) contractStack {
	t.Helper()
	client := fake.NewSimpleClientset(objs...)
	factory := informers.NewSharedInformerFactory(client, 0)
	nsInformer := factory.Core().V1().Namespaces()
	nsLister := nsInformer.Lister()
	rbac := rbacadapter.NewRBACAuthorizer(factory)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	factory.Start(stop)
	for typ, ok := range factory.WaitForCacheSync(stop) {
		require.True(t, ok, "informer %v failed to sync", typ)
	}

	engine, err := multitenancy.NewEngine(writeMTConfig(t, mtConfig), nsLister, func() bool { return true }, scope)
	require.NoError(t, err)
	engine.SetIndependentRBACChecker(rbac)

	auth := scopefilter.NewIdentityReadAuthorizer(composite.NewCompositeAuthorizer(engine, rbac), registry)
	return contractStack{
		storage: NewBulkSARStorage(auth),
		auth:    auth,
	}
}

func createBulkSAR(t *testing.T, storage *BulkSARStorage, subject string, groups []string, reqs []v1alpha1.SubjectAccessReviewRequest) []v1alpha1.SubjectAccessReviewResult {
	t.Helper()
	got, err := createBulkSARResult(storage, subject, groups, reqs)
	require.NoError(t, err)
	require.Len(t, got, len(reqs))
	return got
}

func createBulkSARResult(storage *BulkSARStorage, subject string, groups []string, reqs []v1alpha1.SubjectAccessReviewRequest) ([]v1alpha1.SubjectAccessReviewResult, error) {
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{
		Name:   contractAdminUser,
		Groups: []string{"system:authenticated"},
	})
	out, err := storage.Create(ctx, &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			User:     subject,
			Groups:   groups,
			Requests: reqs,
		},
	}, nil, nil)
	if err != nil {
		return nil, err
	}
	got, ok := out.(*v1alpha1.BulkSubjectAccessReview)
	if !ok {
		return nil, errors.New("object is not a BulkSubjectAccessReview")
	}
	return got.Status.Results, nil
}

func authorizeUnbound(t *testing.T, auth authorizer.Authorizer, subject string, groups []string, reqs []v1alpha1.SubjectAccessReviewRequest) []reviewOutcome {
	t.Helper()
	out := make([]reviewOutcome, len(reqs))
	subjectInfo := &user.DefaultInfo{Name: subject, Groups: groups}
	for i := range reqs {
		attrs := &accessAttributes{
			user:                  subjectInfo,
			resourceAttributes:    reqs[i].ResourceAttributes,
			nonResourceAttributes: reqs[i].NonResourceAttributes,
		}
		d, reason, err := auth.Authorize(context.Background(), attrs)
		require.NoError(t, err)
		out[i] = outcomeFromDecision(d, reason)
	}
	return out
}

func assertBoundEqualsUnbound(t *testing.T, stack contractStack, subject string, groups []string, reqs []v1alpha1.SubjectAccessReviewRequest) []v1alpha1.SubjectAccessReviewResult {
	t.Helper()
	bound := createBulkSAR(t, stack.storage, subject, groups, reqs)
	unbound := authorizeUnbound(t, stack.auth, subject, groups, reqs)
	require.Len(t, unbound, len(bound))
	for i := range bound {
		got := outcomeFromResult(bound[i])
		assert.Equal(t, unbound[i].Allowed, got.Allowed, "allowed item %d %+v bound=%+v unbound=%+v", i, reqs[i], got, unbound[i])
		assert.Equal(t, unbound[i].Denied, got.Denied, "denied item %d %+v bound=%+v unbound=%+v", i, reqs[i], got, unbound[i])
		if got.Denied || unbound[i].Denied {
			assert.Equal(t, unbound[i].Reason, got.Reason, "deny reason item %d %+v", i, reqs[i])
		}
	}
	return bound
}

func assertOutcome(t *testing.T, got reviewOutcome, allowed, denied bool, reason string) {
	t.Helper()
	assert.Equal(t, allowed, got.Allowed, "allowed: %+v", got)
	assert.Equal(t, denied, got.Denied, "denied: %+v", got)
	if reason != "" {
		assert.Equal(t, reason, got.Reason)
	}
}

func wildcardRBAC(editor, super string) []runtime.Object {
	objs := append(labeledNS(), reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		clusterRole("user-authz:super-admin", wildcardRules()),
		carCRB("user-authz:editor:editor", "user-authz:editor", editor),
		carCRB("user-authz:super:super-admin", "user-authz:super-admin", super),
	)
	return objs
}

func restrictedRBAC(editor, super string) []runtime.Object {
	objs := append(labeledNS(), reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", restrictedEditorRules()),
		clusterRole("user-authz:super-admin", wildcardRules()),
		carCRB("user-authz:editor:editor", "user-authz:editor", editor),
		carCRB("user-authz:super:super-admin", "user-authz:super-admin", super),
	)
	return objs
}

const editorLabelSelectorConfig = `{
	"crds": [{
		"name": "editor-sel",
		"spec": {
			"accessLevel": "Editor",
			"namespaceSelector": {"labelSelector": {"matchLabels": {"pba-perf": "true"}}},
			"subjects": [{"kind": "User", "name": "editor@example.io"}]
		}
	}]
}`

// TestBulkSARStorage_Create_Sec40RestrictedEditor locks the e2e sec40
// four-item matrix against real Editor RBAC (no nodes). get nodes is
// NoOpinion, not Deny: MT lets cluster-scoped nodes through, RBAC has no grant.
func TestBulkSARStorage_Create_Sec40RestrictedEditor(t *testing.T) {
	editorStack := newContractStack(t, bulkSAREditorConfig, restrictedRBAC(contractEditorUser, contractSuperUser), contractScope())
	superStack := newContractStack(t, bulkSARSuperConfig, restrictedRBAC(contractEditorUser, contractSuperUser), contractScope())

	editor := assertBoundEqualsUnbound(t, editorStack, contractEditorUser, nil, sec40Requests())
	assertOutcome(t, outcomeFromResult(editor[0]), false, true, "making cluster-scoped requests for namespaced resources is not allowed")
	assertOutcome(t, outcomeFromResult(editor[1]), true, false, "")
	assertOutcome(t, outcomeFromResult(editor[2]), false, true, "either you have no access to the namespace or the namespace does not exist")
	assert.False(t, editor[3].Allowed, "Editor RBAC has no nodes: %+v", editor[3])
	assert.False(t, editor[3].Denied, "nodes are cluster-scoped, MT must not Deny: %+v", editor[3])

	super := assertBoundEqualsUnbound(t, superStack, contractSuperUser, nil, sec40Requests())
	for i, r := range super {
		assert.True(t, r.Allowed, "SuperAdmin request %d: %+v", i, r)
		assert.False(t, r.Denied)
	}
}

// TestBulkSARStorage_Create_WildcardEditorClusterReality locks the live
// cluster shape: user-authz:editor is effectively * * *, so MT is the only
// deny. get nodes is Allowed via the CAR CRB.
func TestBulkSARStorage_Create_WildcardEditorClusterReality(t *testing.T) {
	editorStack := newContractStack(t, bulkSAREditorConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	superStack := newContractStack(t, bulkSARSuperConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	reqs := consoleContractRequests()

	editor := assertBoundEqualsUnbound(t, editorStack, contractEditorUser, nil, reqs)
	super := assertBoundEqualsUnbound(t, superStack, contractSuperUser, nil, reqs)
	require.Len(t, editor, len(reqs))
	require.Len(t, super, len(reqs))

	denyClusterNamespaced := map[int]struct{}{
		0: {}, 1: {}, 2: {}, 3: {}, 4: {}, 5: {}, 6: {}, 25: {},
	}
	denyOutsideNS := map[int]struct{}{
		19: {}, 20: {}, 21: {}, 22: {}, 23: {},
	}
	unknownGVR := 24
	projectsList := 12
	projectsCreate := 13
	nodesGet := 7
	podsIn := 14
	podsLogIn := 26
	nonResource := 27

	for i, r := range editor {
		got := outcomeFromResult(r)
		switch {
		case i == unknownGVR:
			assertOutcome(t, got, false, true, "making cluster-scoped requests for namespaced resources is not allowed")
		case i == projectsList:
			assert.True(t, got.Allowed, "list projects: %+v", got)
			assert.False(t, got.Denied)
		case i == projectsCreate:
			assert.True(t, got.Allowed, "wildcard Editor create projects: %+v", got)
		case i == nodesGet:
			assert.True(t, got.Allowed, "wildcard Editor get nodes: %+v", got)
			assert.False(t, got.Denied)
		case i == podsIn || i == podsLogIn:
			assert.True(t, got.Allowed, "item %d: %+v", i, got)
			assert.False(t, got.Denied)
		case i == nonResource:
			assert.True(t, got.Allowed, "non-resource: %+v", got)
		case func() bool { _, ok := denyClusterNamespaced[i]; return ok }():
			assertOutcome(t, got, false, true, "making cluster-scoped requests for namespaced resources is not allowed")
		case func() bool { _, ok := denyOutsideNS[i]; return ok }():
			assertOutcome(t, got, false, true, "either you have no access to the namespace or the namespace does not exist")
		default:
			assert.True(t, got.Allowed, "item %d should be allowed for wildcard Editor: %+v req=%+v", i, got, reqs[i])
			assert.False(t, got.Denied)
		}
	}

	for i, r := range super {
		assert.True(t, r.Allowed, "SuperAdmin item %d: %+v req=%+v", i, r, reqs[i])
		assert.False(t, r.Denied)
	}
}

func TestBulkSARStorage_Create_LimitNamespacesEqualsLabelSelector(t *testing.T) {
	objs := wildcardRBAC(contractEditorUser, contractSuperUser)
	limited := newContractStack(t, bulkSAREditorConfig, objs, contractScope())
	selected := newContractStack(t, editorLabelSelectorConfig, objs, contractScope())
	reqs := consoleContractRequests()

	fromLimit := assertBoundEqualsUnbound(t, limited, contractEditorUser, nil, reqs)
	fromSelector := assertBoundEqualsUnbound(t, selected, contractEditorUser, nil, reqs)
	for i := range reqs {
		assert.Equal(t, outcomeFromResult(fromLimit[i]), outcomeFromResult(fromSelector[i]),
			"item %d %+v", i, reqs[i])
	}
}

func TestBulkSARStorage_Create_RestrictedEditorIdentityReadAndNodes(t *testing.T) {
	stack := newContractStack(t, bulkSAREditorConfig, restrictedRBAC(contractEditorUser, contractSuperUser), contractScope())
	reqs := consoleContractRequests()
	got := assertBoundEqualsUnbound(t, stack, contractEditorUser, nil, reqs)

	assert.True(t, got[7].Allowed == false && got[7].Denied == false, "get nodes NoOpinion: %+v", got[7])
	assert.True(t, got[8].Allowed == false && got[8].Denied == false, "list nodes NoOpinion: %+v", got[8])
	assert.True(t, got[10].Allowed == false && got[10].Denied == false, "list clusterroles NoOpinion: %+v", got[10])

	assert.True(t, got[12].Allowed, "list projects via IdentityRead: %+v", got[12])
	assert.Equal(t, filteredReadReason, got[12].Reason)
	assert.False(t, got[13].Allowed, "create projects is not an identity read: %+v", got[13])
	assert.False(t, got[13].Denied)

	assert.True(t, got[0].Denied)
	assert.True(t, got[14].Allowed, "get pods ns-in: %+v", got[14])
	assert.True(t, got[19].Denied, "get pods ns-out: %+v", got[19])
	assert.True(t, got[24].Denied, "unknown GVR: %+v", got[24])
}

func TestBulkSARStorage_Create_IndependentRBACRescuesWithoutCARCRB(t *testing.T) {
	objs := append(restrictedRBAC(contractEditorUser, contractSuperUser),
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-reader", Namespace: "ns-out"},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}}},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "editor-pod-reader", Namespace: "ns-out"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: contractEditorUser}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "pod-reader"},
		},
		clusterRole("secret-viewer", []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list"}},
		}),
		userCRB("editor-secret-viewer", "secret-viewer", contractEditorUser),
	)
	stack := newContractStack(t, bulkSAREditorConfig, objs, contractScope())

	reqs := []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "", "pods", ""),
		ra("get", "", "pods", "ns-out"),
		ra("delete", "", "pods", "ns-out"),
		ra("list", "", "secrets", ""),
		ra("get", "", "secrets", "ns-out"),
		ra("get", "apps", "deployments", "ns-out"),
	}
	got := assertBoundEqualsUnbound(t, stack, contractEditorUser, nil, reqs)

	assertOutcome(t, outcomeFromResult(got[0]), false, true, "making cluster-scoped requests for namespaced resources is not allowed")
	assert.True(t, got[1].Allowed, "RoleBinding must rescue get pods in ns-out: %+v", got[1])
	assertOutcome(t, outcomeFromResult(got[2]), false, true, "either you have no access to the namespace or the namespace does not exist")
	assert.True(t, got[3].Allowed, "user CRB must rescue cluster-scoped list secrets: %+v", got[3])
	assert.True(t, got[4].Allowed, "user CRB must rescue get secrets in ns-out: %+v", got[4])
	assertOutcome(t, outcomeFromResult(got[5]), false, true, "either you have no access to the namespace or the namespace does not exist")
}

func TestBulkSARStorage_Create_SelfModeMatchesNonSelf(t *testing.T) {
	stack := newContractStack(t, bulkSAREditorConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	reqs := sec40Requests()

	nonSelf := createBulkSAR(t, stack.storage, contractEditorUser, nil, reqs)

	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: contractEditorUser})
	out, err := stack.storage.Create(ctx, &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{Requests: reqs},
	}, nil, nil)
	require.NoError(t, err)
	self := out.(*v1alpha1.BulkSubjectAccessReview).Status.Results
	require.Len(t, self, len(nonSelf))
	for i := range nonSelf {
		assert.Equal(t, outcomeFromResult(nonSelf[i]), outcomeFromResult(self[i]), "item %d", i)
	}
}

func TestBulkSARStorage_Create_GroupSubjectMatchesUserSubject(t *testing.T) {
	const groupConfig = `{
		"crds": [{
			"name": "editor-group",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "Group", "name": "editors"}]
			}
		}]
	}`
	member := "grouped@example.io"
	objs := append(labeledNS(), reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		carCRB("user-authz:grouped:editor", "user-authz:editor", member),
	)
	groupStack := newContractStack(t, groupConfig, objs, contractScope())
	userStack := newContractStack(t, bulkSAREditorConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())

	fromUser := createBulkSAR(t, userStack.storage, contractEditorUser, nil, sec40Requests())
	fromGroup := createBulkSAR(t, groupStack.storage, member, []string{"editors"}, sec40Requests())
	for i := range fromUser {
		assert.Equal(t, fromUser[i].Allowed, fromGroup[i].Allowed, "allowed item %d", i)
		assert.Equal(t, fromUser[i].Denied, fromGroup[i].Denied, "denied item %d", i)
		if fromUser[i].Denied || fromGroup[i].Denied {
			assert.Equal(t, fromUser[i].Reason, fromGroup[i].Reason, "deny reason item %d", i)
		}
	}
}

func TestBulkSARStorage_Create_UnknownScopeIsDeniedForFilteredUser(t *testing.T) {
	stack := newContractStack(t, bulkSAREditorConfig, wildcardRBAC(contractEditorUser, contractSuperUser), staticResourceScope{})
	got := createBulkSAR(t, stack.storage, contractEditorUser, nil, []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "", "pods", ""),
		ra("get", "", "nodes", ""),
	})
	assertOutcome(t, outcomeFromResult(got[0]), false, true, "making cluster-scoped requests for namespaced resources is not allowed")
	assertOutcome(t, outcomeFromResult(got[1]), false, true, "making cluster-scoped requests for namespaced resources is not allowed")
}

func TestBulkSARStorage_Create_EmptyScopeDoesNotChangeSuperAdmin(t *testing.T) {
	empty := newContractStack(t, bulkSARSuperConfig, wildcardRBAC(contractEditorUser, contractSuperUser), staticResourceScope{})
	full := newContractStack(t, bulkSARSuperConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	reqs := consoleContractRequests()

	fromEmpty := createBulkSAR(t, empty.storage, contractSuperUser, nil, reqs)
	fromFull := createBulkSAR(t, full.storage, contractSuperUser, nil, reqs)
	for i := range reqs {
		assert.Equal(t, fromFull[i].Allowed, fromEmpty[i].Allowed, "allowed item %d %+v", i, reqs[i])
		assert.Equal(t, fromFull[i].Denied, fromEmpty[i].Denied, "denied item %d %+v", i, reqs[i])
		assert.True(t, fromEmpty[i].Allowed, "SuperAdmin matchAny must Allow item %d: %+v", i, fromEmpty[i])
	}
}
