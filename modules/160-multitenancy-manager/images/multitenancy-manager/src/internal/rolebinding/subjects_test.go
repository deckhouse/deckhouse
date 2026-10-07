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

package rolebinding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
)

func serviceAccount(namespace, name string) rbacv1.Subject {
	return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: name, Namespace: namespace}
}

func user(name string) rbacv1.Subject {
	return rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: name}
}

func group(name string) rbacv1.Subject {
	return rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: name}
}

func namespaceOf(name, project string) *corev1.Namespace {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if project != "" {
		ns.Labels = map[string]string{v1alpha3.ResourceLabelProject: project}
	}
	return ns
}

func TestServiceAccountNamespace(t *testing.T) {
	tests := []struct {
		name      string
		subject   rbacv1.Subject
		namespace string
		isAccount bool
	}{
		{name: "a service account", subject: serviceAccount("proj-extra", "deployer"), namespace: "proj-extra", isAccount: true},
		{name: "a service account without a namespace", subject: serviceAccount("", "deployer"), isAccount: true},
		{name: "the user of a service account", subject: user("system:serviceaccount:proj-extra:deployer"), namespace: "proj-extra", isAccount: true},
		{name: "the group of the service accounts of a namespace", subject: group("system:serviceaccounts:proj-extra"), namespace: "proj-extra", isAccount: true},
		{name: "a user", subject: user("alice")},
		{name: "a group", subject: group("developers")},
		{name: "the group of every service account names no namespace", subject: group("system:serviceaccounts")},
		{name: "a user without the account name", subject: user("system:serviceaccount:proj-extra")},
		{name: "a user with an empty namespace", subject: user("system:serviceaccount::deployer")},
		{name: "a user with an extra part", subject: user("system:serviceaccount:proj-extra:deployer:x")},
		{name: "a group with an empty namespace", subject: group("system:serviceaccounts:")},
		{name: "another kind", subject: rbacv1.Subject{Kind: "Robot", Name: "system:serviceaccount:proj-extra:deployer"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace, isAccount := ServiceAccountNamespace(tt.subject)
			assert.Equal(t, tt.isAccount, isAccount)
			assert.Equal(t, tt.namespace, namespace)
		})
	}
}

func TestServiceAccountNamespaces(t *testing.T) {
	subjects := []rbacv1.Subject{
		serviceAccount("proj-b", "x"),
		user("alice"),
		user("system:serviceaccount:proj-a:y"),
		group("system:serviceaccounts:proj-b"),
		serviceAccount("proj", "main"),
		serviceAccount("", "anywhere"),
	}
	assert.Equal(t, []string{"proj-a", "proj-b"}, ServiceAccountNamespaces(subjects, "proj"))
	assert.Empty(t, ServiceAccountNamespaces([]rbacv1.Subject{user("alice")}, "proj"))
}

// The fan-out judges a ServiceAccount by the namespace it is in now. Whatever the webhook accepted
// when the subject was added, a namespace that is gone, is going or belongs to another project gives
// its ServiceAccounts no rights in this one, in whichever form the subject names them.
func TestSplitProjectSubjects(t *testing.T) {
	leaving := namespaceOf("proj-leaving", "proj")
	leaving.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
	leaving.Finalizers = []string{"test"}
	// The main namespace is not read at all, so it is left out of the cache here.
	c := newClient(t,
		namespaceOf("proj-extra", "proj"),
		namespaceOf("proj-other", "proj-other"),
		namespaceOf("proj-bare", ""),
		leaving,
	)

	subjects := []rbacv1.Subject{
		user("alice"),
		serviceAccount("proj", "main"),
		serviceAccount("proj-extra", "deployer"),
		serviceAccount("proj-other", "deployer"),
		user("system:serviceaccount:proj-other:ci"),
		group("system:serviceaccounts:proj-gone"),
		serviceAccount("proj-leaving", "sa"),
		serviceAccount("proj-bare", "sa"),
		group("system:serviceaccounts:proj-extra"),
		group("system:authenticated"),
		serviceAccount("", "local"),
	}
	kept, leftOut, err := SplitProjectSubjects(context.Background(), c, "proj", subjects, nil)
	require.NoError(t, err)
	assert.Equal(t, []rbacv1.Subject{
		user("alice"),
		serviceAccount("proj", "main"),
		serviceAccount("proj-extra", "deployer"),
		group("system:serviceaccounts:proj-extra"),
		group("system:authenticated"),
		serviceAccount("", "local"),
	}, kept)
	assert.Equal(t, []rbacv1.Subject{
		serviceAccount("proj-other", "deployer"),
		user("system:serviceaccount:proj-other:ci"),
		group("system:serviceaccounts:proj-gone"),
		serviceAccount("proj-leaving", "sa"),
		serviceAccount("proj-bare", "sa"),
	}, leftOut)
}

func TestSplitProjectSubjects_OnlyUsersAndGroups(t *testing.T) {
	subjects := []rbacv1.Subject{user("alice"), group("developers")}
	kept, leftOut, err := SplitProjectSubjects(context.Background(), newClient(t), "proj", subjects, nil)
	require.NoError(t, err)
	assert.Equal(t, subjects, kept)
	assert.Empty(t, leftOut)
}

// Only the d8-administrators binding with the managed-by label of the controller is the one the
// controller writes from the administrators of its project.
func TestIsAdministratorsBinding(t *testing.T) {
	tests := []struct {
		name      string
		binding   string
		managedBy string
		expected  bool
	}{
		{name: "the binding of the controller", binding: v1alpha3.ProjectAdministratorsBinding, managedBy: v1alpha3.ManagedByController, expected: true},
		{name: "the name without the label", binding: v1alpha3.ProjectAdministratorsBinding},
		{name: "another value of the label", binding: v1alpha3.ProjectAdministratorsBinding, managedBy: "somebody-else"},
		{name: "the label on another binding", binding: "ci", managedBy: v1alpha3.ManagedByController},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsAdministratorsBinding(tt.binding, tt.managedBy))
		})
	}
}

// The administrators of the project are carried whatever namespace their ServiceAccounts are in, the
// way the previous release granted them through an AuthorizationRule. A subject that is no
// administrator is judged by its namespace as before, the ServiceAccount ci/runner too, and so is
// every subject of a binding that gets no administrators.
func TestSplitProjectSubjects_Administrators(t *testing.T) {
	c := newClient(t, namespaceOf("ci", "ci"))
	administrators := []v1alpha3.Administrator{
		{Kind: rbacv1.UserKind, Name: "alice"},
		{Kind: rbacv1.UserKind, Name: "system:serviceaccount:ci:runner"},
		{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:ci"},
	}
	subjects := []rbacv1.Subject{
		user("alice"),
		user("system:serviceaccount:ci:runner"),
		group("system:serviceaccounts:ci"),
		user("system:serviceaccount:ci:deployer"),
		serviceAccount("ci", "runner"),
	}

	kept, leftOut, err := SplitProjectSubjects(context.Background(), c, "proj", subjects, administrators)
	require.NoError(t, err)
	assert.Equal(t, subjects[:3], kept)
	assert.Equal(t, subjects[3:], leftOut)

	kept, leftOut, err = SplitProjectSubjects(context.Background(), c, "proj", subjects, nil)
	require.NoError(t, err)
	assert.Equal(t, subjects[:1], kept)
	assert.Equal(t, subjects[1:], leftOut)
}

// A stored binding that names the ServiceAccounts of a namespace keeps a new namespace under that
// name from being made for someone else, and the reason names neither the binding nor its project.
func TestServiceAccountGrantConflict(t *testing.T) {
	cprb := func(subjects ...rbacv1.Subject) *v1alpha3.ClusterProjectRoleBinding {
		return &v1alpha3.ClusterProjectRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "hidden-binding"},
			Spec:       v1alpha3.ClusterProjectRoleBindingSpec{Subjects: subjects},
		}
	}
	prb := func(project string, subjects ...rbacv1.Subject) *v1alpha3.ProjectRoleBinding {
		return &v1alpha3.ProjectRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "hidden-binding", Namespace: project},
			Spec:       v1alpha3.ProjectRoleBindingSpec{Subjects: subjects},
		}
	}
	const (
		byCluster = "a ClusterProjectRoleBinding still grants rights to ServiceAccounts of a namespace with this name"
		byProject = "a ProjectRoleBinding of another project still grants rights to ServiceAccounts of a namespace with this name"
	)

	tests := []struct {
		name     string
		objects  []client.Object
		lookup   string
		project  string
		expected string
	}{
		{name: "no bindings", lookup: "proj-extra", project: "proj-extra"},
		{
			name:    "bindings of other namespaces",
			objects: []client.Object{cprb(serviceAccount("tools-ci", "runner")), prb("proj", serviceAccount("proj-queue", "sa"))},
			lookup:  "proj-extra", project: "proj-extra",
		},
		{
			name:    "a cluster binding",
			objects: []client.Object{cprb(user("alice"), serviceAccount("proj-extra", "runner"))},
			lookup:  "proj-extra", project: "proj-extra", expected: byCluster,
		},
		{
			name:    "a cluster binding with the group of the namespace",
			objects: []client.Object{cprb(group("system:serviceaccounts:proj-extra"))},
			lookup:  "proj-extra", project: "proj", expected: byCluster,
		},
		{
			name:    "a binding of the project the name extends",
			objects: []client.Object{prb("proj", serviceAccount("proj-extra", "deployer"))},
			lookup:  "proj-extra", project: "proj-extra", expected: byProject,
		},
		{
			name:    "a binding of a project the name extends at a later dash, with the user of the account",
			objects: []client.Object{prb("proj-a", user("system:serviceaccount:proj-a-x:sa"))},
			lookup:  "proj-a-x", project: "proj", expected: byProject,
		},
		{
			// A ProjectNamespace that brings back a namespace of its own project returns the
			// rights to where they belong.
			name:    "a binding of the project itself",
			objects: []client.Object{prb("proj", serviceAccount("proj-extra", "deployer"))},
			lookup:  "proj-extra", project: "proj",
		},
		{
			name:    "a binding of a project the name does not extend is not read",
			objects: []client.Object{prb("other", user("system:serviceaccount:proj-extra:deployer"))},
			lookup:  "proj-extra", project: "proj-extra",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, err := ServiceAccountGrantConflict(context.Background(), newClient(t, tt.objects...), tt.lookup, tt.project)
			require.NoError(t, err)
			if tt.expected == "" {
				assert.Empty(t, reason)
				return
			}
			assert.Contains(t, reason, tt.expected)
			assert.Contains(t, reason, "have to be removed from that binding first")
			assert.NotContains(t, reason, "hidden-binding")
		})
	}
}
