/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
)

func extraContractNamespaces() []runtime.Object {
	return []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-b"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-prod"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-monitoring"}},
	}
}

func narrowReviewerObjects() []runtime.Object {
	return []runtime.Object{
		clusterRole("pba-reviewer", []rbacv1.PolicyRule{{
			APIGroups: []string{"authorization.deckhouse.io"},
			Resources: []string{"bulksubjectaccessreviews/nonself"},
			Verbs:     []string{"create"},
		}}),
		userCRB("pba-reviewer", "pba-reviewer", contractAdminUser),
	}
}

func carCRBSA(name, role, saNS, saName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: carLabels},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: saName, Namespace: saNS}},
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: role},
	}
}

const dualEditorCARConfig = `{
	"crds": [
		{
			"name": "editor-a",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-a"],
				"subjects": [{"kind": "User", "name": "dual@example.io"}]
			}
		},
		{
			"name": "editor-b",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-b"],
				"subjects": [{"kind": "User", "name": "dual@example.io"}]
			}
		}
	]
}`

const userLimitPlusGroupMatchAnyConfig = `{
	"crds": [
		{
			"name": "editor",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "User", "name": "union@example.io"}]
			}
		},
		{
			"name": "ops",
			"spec": {
				"accessLevel": "SuperAdmin",
				"namespaceSelector": {"matchAny": true},
				"subjects": [{"kind": "Group", "name": "ops"}]
			}
		}
	]
}`

const selectorIgnoresLimitConfig = `{
	"crds": [{
		"name": "editor-both",
		"spec": {
			"accessLevel": "Editor",
			"limitNamespaces": ["ns-a"],
			"namespaceSelector": {"labelSelector": {"matchLabels": {"pba-perf": "true"}}},
			"subjects": [{"kind": "User", "name": "editor@example.io"}]
		}
	}]
}`

const editorRegexCARConfig = `{
	"crds": [{
		"name": "editor-re",
		"spec": {
			"accessLevel": "Editor",
			"limitNamespaces": ["app-.*"],
			"subjects": [{"kind": "User", "name": "regex@example.io"}]
		}
	}]
}`

const editorSACARConfig = `{
	"crds": [{
		"name": "editor-sa",
		"spec": {
			"accessLevel": "Editor",
			"limitNamespaces": ["ns-in"],
			"subjects": [{"kind": "ServiceAccount", "name": "app", "namespace": "ns-in"}]
		}
	}]
}`

const bothSubjectsConfig = `{
	"crds": [
		{
			"name": "editor",
			"spec": {
				"accessLevel": "Editor",
				"limitNamespaces": ["ns-in"],
				"subjects": [{"kind": "User", "name": "editor@example.io"}]
			}
		},
		{
			"name": "super",
			"spec": {
				"accessLevel": "SuperAdmin",
				"allowAccessToSystemNamespaces": true,
				"namespaceSelector": {"matchAny": true},
				"subjects": [{"kind": "User", "name": "super@example.io"}]
			}
		}
	]
}`

// TestBulkSARStorage_Create_NarrowReviewerBindsSubject locks the non-self
// gate: the caller may only create .../nonself. Item answers must come from
// the subject's CAR CRB, not from the caller. If BindSubject is skipped or
// applied before the gate, get pods in the CAR ns stops being Allowed.
func TestBulkSARStorage_Create_NarrowReviewerBindsSubject(t *testing.T) {
	objs := append(labeledNS(), narrowReviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		carCRB("user-authz:editor:editor", "user-authz:editor", contractEditorUser),
	)
	stack := newContractStack(t, bulkSAREditorConfig, objs, contractScope())
	got := assertBoundEqualsUnbound(t, stack, contractEditorUser, nil, sec40Requests())

	assertOutcome(t, outcomeFromResult(got[0]), false, true, "making cluster-scoped requests for namespaced resources is not allowed")
	assert.True(t, got[1].Allowed, "subject Editor must still get pods in ns-in: %+v", got[1])
	assert.True(t, got[2].Denied, "subject Editor must not get pods in ns-out: %+v", got[2])
	assert.True(t, got[3].Allowed, "wildcard Editor get nodes: %+v", got[3])
}

func TestBulkSARStorage_Create_RestrictedEditorCannotReviewOthers(t *testing.T) {
	stack := newContractStack(t, bothSubjectsConfig, restrictedRBAC(contractEditorUser, contractSuperUser), contractScope())
	ctx := request.WithUser(context.Background(), &user.DefaultInfo{Name: contractEditorUser})
	_, err := stack.storage.Create(ctx, &v1alpha1.BulkSubjectAccessReview{
		Spec: v1alpha1.BulkSubjectAccessReviewSpec{
			User:     contractSuperUser,
			Requests: sec40Requests(),
		},
	}, nil, nil)
	require.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err), "restricted Editor must not non-self review SuperAdmin: %v", err)
}

func TestBulkSARStorage_Create_TwoCARsUnionNamespaces(t *testing.T) {
	const user = "dual@example.io"
	objs := append(labeledNS(), extraContractNamespaces()...)
	objs = append(objs, reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		carCRB("user-authz:editor-a:editor", "user-authz:editor", user),
		carCRB("user-authz:editor-b:editor", "user-authz:editor", user),
	)
	stack := newContractStack(t, dualEditorCARConfig, objs, contractScope())
	reqs := []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "", "pods", ""),
		ra("get", "", "pods", "ns-a"),
		ra("get", "", "pods", "ns-b"),
		ra("get", "", "pods", "ns-out"),
		ra("get", "", "nodes", ""),
	}
	got := assertBoundEqualsUnbound(t, stack, user, nil, reqs)
	assert.True(t, got[0].Denied)
	assert.True(t, got[1].Allowed, "ns-a from first CAR: %+v", got[1])
	assert.True(t, got[2].Allowed, "ns-b from second CAR: %+v", got[2])
	assert.True(t, got[3].Denied, "union must not leak ns-out: %+v", got[3])
	assert.True(t, got[4].Allowed)
}

func TestBulkSARStorage_Create_MatchAnyGroupOverridesUserLimit(t *testing.T) {
	const user = "union@example.io"
	objs := append(labeledNS(), reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		clusterRole("user-authz:super-admin", wildcardRules()),
		carCRB("user-authz:union:editor", "user-authz:editor", user),
	)
	stack := newContractStack(t, userLimitPlusGroupMatchAnyConfig, objs, contractScope())

	limited := createBulkSAR(t, stack.storage, user, nil, sec40Requests())
	assert.True(t, limited[0].Denied, "user CAR alone stays filtered")
	assert.True(t, limited[2].Denied)

	withOps := createBulkSAR(t, stack.storage, user, []string{"ops"}, sec40Requests())
	for i, r := range withOps {
		assert.True(t, r.Allowed, "MatchAny on a group must clear filters, item %d: %+v", i, r)
	}
}

func TestBulkSARStorage_Create_SelectorIgnoresSiblingLimitNamespaces(t *testing.T) {
	objs := append(labeledNS(), extraContractNamespaces()...)
	objs = append(objs, reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		carCRB("user-authz:editor:editor", "user-authz:editor", contractEditorUser),
	)
	stack := newContractStack(t, selectorIgnoresLimitConfig, objs, contractScope())
	reqs := []v1alpha1.SubjectAccessReviewRequest{
		ra("get", "", "pods", "ns-in"),
		ra("get", "", "pods", "ns-a"),
		ra("get", "", "pods", "ns-out"),
	}
	got := assertBoundEqualsUnbound(t, stack, contractEditorUser, nil, reqs)
	assert.True(t, got[0].Allowed, "labelSelector ns-in: %+v", got[0])
	assert.True(t, got[1].Denied, "limitNamespaces is ignored when namespaceSelector is set: %+v", got[1])
	assert.True(t, got[2].Denied)
}

func TestBulkSARStorage_Create_LimitNamespacesRegex(t *testing.T) {
	const user = "regex@example.io"
	objs := append(labeledNS(), extraContractNamespaces()...)
	objs = append(objs, reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		carCRB("user-authz:regex:editor", "user-authz:editor", user),
	)
	stack := newContractStack(t, editorRegexCARConfig, objs, contractScope())
	reqs := []v1alpha1.SubjectAccessReviewRequest{
		ra("get", "", "pods", "app-prod"),
		ra("get", "", "pods", "ns-in"),
		ra("list", "", "pods", ""),
	}
	got := assertBoundEqualsUnbound(t, stack, user, nil, reqs)
	assert.True(t, got[0].Allowed, "app-.* must match app-prod: %+v", got[0])
	assert.True(t, got[1].Denied, "ns-in is outside app-.*: %+v", got[1])
	assert.True(t, got[2].Denied)
}

func TestBulkSARStorage_Create_ServiceAccountSubject(t *testing.T) {
	sa := "system:serviceaccount:ns-in:app"
	objs := append(labeledNS(), reviewerObjects()...)
	objs = append(objs,
		clusterRole("user-authz:editor", wildcardRules()),
		carCRBSA("user-authz:editor-sa:editor", "user-authz:editor", "ns-in", "app"),
	)
	stack := newContractStack(t, editorSACARConfig, objs, contractScope())
	got := assertBoundEqualsUnbound(t, stack, sa, nil, sec40Requests())
	assert.True(t, got[0].Denied)
	assert.True(t, got[1].Allowed, "SA in CAR ns: %+v", got[1])
	assert.True(t, got[2].Denied)
	assert.True(t, got[3].Allowed)
}

func TestBulkSARStorage_Create_SystemNamespacesAndDefault(t *testing.T) {
	objs := append(wildcardRBAC(contractEditorUser, contractSuperUser), extraContractNamespaces()...)
	stack := newContractStack(t, bothSubjectsConfig, objs, contractScope())
	reqs := []v1alpha1.SubjectAccessReviewRequest{
		ra("get", "", "pods", "default"),
		ra("get", "", "pods", "kube-public"),
		ra("get", "", "pods", "d8-monitoring"),
		ra("get", "", "pods", "kube-system"),
	}
	editor := createBulkSAR(t, stack.storage, contractEditorUser, nil, reqs)
	for i, r := range editor {
		assert.True(t, r.Denied, "Editor must be denied system ns item %d: %+v", i, r)
		assert.Equal(t, "either you have no access to the namespace or the namespace does not exist", r.Reason)
	}
	super := createBulkSAR(t, stack.storage, contractSuperUser, nil, reqs)
	for i, r := range super {
		assert.True(t, r.Allowed, "SuperAdmin matchAny item %d: %+v", i, r)
	}
}

func TestBulkSARStorage_Create_VersionAndNameDoNotChangeDecision(t *testing.T) {
	stack := newContractStack(t, bulkSAREditorConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	base := []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "", "pods", ""),
		ra("get", "", "pods", "ns-in"),
		ra("get", "", "pods", "ns-out"),
		ra("get", "", "nodes", ""),
	}
	varied := []v1alpha1.SubjectAccessReviewRequest{
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "list", Version: "v1", Resource: "pods"}},
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Version: "v1beta1", Resource: "pods", Namespace: "ns-in", Name: "web"}},
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Version: "v1", Resource: "pods", Namespace: "ns-out", Name: "hidden"}},
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Version: "v1", Resource: "nodes", Name: "node-1"}},
	}
	fromBase := createBulkSAR(t, stack.storage, contractEditorUser, nil, base)
	fromVaried := createBulkSAR(t, stack.storage, contractEditorUser, nil, varied)
	for i := range base {
		assert.Equal(t, fromBase[i].Allowed, fromVaried[i].Allowed, "allowed item %d", i)
		assert.Equal(t, fromBase[i].Denied, fromVaried[i].Denied, "denied item %d", i)
		if fromBase[i].Denied {
			assert.Equal(t, fromBase[i].Reason, fromVaried[i].Reason, "a named/versioned deny must keep the webhook reason")
		}
	}
}

func TestBulkSARStorage_Create_VerbsDoNotChangeMTFilter(t *testing.T) {
	stack := newContractStack(t, bulkSAREditorConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}
	var reqs []v1alpha1.SubjectAccessReviewRequest
	for _, v := range verbs {
		reqs = append(reqs, ra(v, "", "pods", ""), ra(v, "", "pods", "ns-out"), ra(v, "", "nodes", ""))
	}
	got := createBulkSAR(t, stack.storage, contractEditorUser, nil, reqs)
	for i, v := range verbs {
		assert.True(t, got[i*3].Denied, "%s pods cluster-scoped: %+v", v, got[i*3])
		assert.True(t, got[i*3+1].Denied, "%s pods ns-out: %+v", v, got[i*3+1])
		assert.True(t, got[i*3+2].Allowed, "%s nodes: %+v", v, got[i*3+2])
	}
}

func TestBulkSARStorage_Create_ProjectsNeedTheCRD(t *testing.T) {
	stack := newContractStackWithRegistry(t, bulkSAREditorConfig, restrictedRBAC(contractEditorUser, contractSuperUser), contractScope(), servedResources{})
	got := createBulkSAR(t, stack.storage, contractEditorUser, nil, []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "deckhouse.io", "projects", ""),
	})
	assert.False(t, got[0].Allowed, "no Project CRD: %+v", got[0])
	assert.False(t, got[0].Denied)
}

func TestBulkSARStorage_Create_ResourceNamesDoNotRescueList(t *testing.T) {
	objs := append(restrictedRBAC(contractEditorUser, contractSuperUser),
		clusterRole("named-secret", []rbacv1.PolicyRule{{
			APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list"}, ResourceNames: []string{"only-this"},
		}}),
		userCRB("editor-named-secret", "named-secret", contractEditorUser),
	)
	stack := newContractStack(t, bulkSAREditorConfig, objs, contractScope())
	got := createBulkSAR(t, stack.storage, contractEditorUser, nil, []v1alpha1.SubjectAccessReviewRequest{
		ra("list", "", "secrets", ""),
		{ResourceAttributes: &v1alpha1.ResourceAttributes{Verb: "get", Resource: "secrets", Name: "only-this"}},
	})
	assert.True(t, got[0].Denied, "resourceNames must not grant an unscoped list: %+v", got[0])
	assert.True(t, got[1].Allowed, "named get via user CRB: %+v", got[1])
}

func TestBulkSARStorage_Create_GeneratedMatrixWildcardEditor(t *testing.T) {
	stack := newContractStack(t, bothSubjectsConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	reqs, expectEditorAllow := generatedContractMatrix()
	editor := createBulkSAR(t, stack.storage, contractEditorUser, nil, reqs)
	super := createBulkSAR(t, stack.storage, contractSuperUser, nil, reqs)
	require.Len(t, editor, len(reqs))
	for i, req := range reqs {
		assert.Equal(t, expectEditorAllow[i], editor[i].Allowed, "Editor allowed item %d %+v got=%+v", i, req, editor[i])
		if !expectEditorAllow[i] {
			assert.True(t, editor[i].Denied, "Editor should Deny item %d %+v", i, req)
		}
		assert.True(t, super[i].Allowed, "SuperAdmin item %d %+v: %+v", i, req, super[i])
		assert.False(t, super[i].Denied)
	}
}

func generatedContractMatrix() ([]v1alpha1.SubjectAccessReviewRequest, []bool) {
	type gvr struct {
		group, resource string
		namespaced      bool
	}
	resources := []gvr{
		{resource: "pods", namespaced: true},
		{resource: "services", namespaced: true},
		{resource: "secrets", namespaced: true},
		{resource: "configmaps", namespaced: true},
		{group: "apps", resource: "deployments", namespaced: true},
		{group: "batch", resource: "jobs", namespaced: true},
		{group: "networking.k8s.io", resource: "ingresses", namespaced: true},
		{resource: "nodes", namespaced: false},
		{resource: "namespaces", namespaced: false},
		{group: "rbac.authorization.k8s.io", resource: "clusterroles", namespaced: false},
		{group: "deckhouse.io", resource: "projects", namespaced: false},
	}
	nss := []string{"", "ns-in", "ns-out", "kube-system", "default"}
	verbs := []string{"get", "list", "watch", "create"}

	var reqs []v1alpha1.SubjectAccessReviewRequest
	var allow []bool
	for _, res := range resources {
		for _, ns := range nss {
			for _, verb := range verbs {
				reqs = append(reqs, ra(verb, res.group, res.resource, ns))
				switch {
				case ns == "ns-in":
					allow = append(allow, true)
				case ns != "":
					allow = append(allow, false)
				case res.namespaced:
					allow = append(allow, false)
				default:
					allow = append(allow, true)
				}
			}
		}
	}
	return reqs, allow
}

func TestBulkSARStorage_Create_ConcurrentEditorAndSuperAdmin(t *testing.T) {
	stack := newContractStack(t, bothSubjectsConfig, wildcardRBAC(contractEditorUser, contractSuperUser), contractScope())
	wantEditor := createBulkSAR(t, stack.storage, contractEditorUser, nil, sec40Requests())
	wantSuper := createBulkSAR(t, stack.storage, contractSuperUser, nil, sec40Requests())

	const workers = 16
	var wg sync.WaitGroup
	errCh := make(chan string, workers*2)
	wg.Add(workers * 2)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			got, err := createBulkSARResult(stack.storage, contractEditorUser, nil, sec40Requests())
			if err != nil {
				errCh <- err.Error()
				return
			}
			if len(got) != len(wantEditor) {
				errCh <- "editor result length drifted"
				return
			}
			for j := range wantEditor {
				if got[j].Allowed != wantEditor[j].Allowed || got[j].Denied != wantEditor[j].Denied {
					errCh <- "editor result drifted"
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			got, err := createBulkSARResult(stack.storage, contractSuperUser, nil, sec40Requests())
			if err != nil {
				errCh <- err.Error()
				return
			}
			if len(got) != len(wantSuper) {
				errCh <- "superadmin result length drifted"
				return
			}
			for j := range wantSuper {
				if !got[j].Allowed || got[j].Denied {
					errCh <- "superadmin result drifted"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for msg := range errCh {
		t.Error(msg)
	}
}
