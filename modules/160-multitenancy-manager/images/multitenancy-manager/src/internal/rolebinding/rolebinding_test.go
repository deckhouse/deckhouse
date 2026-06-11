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
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"

	"controller/apis/deckhouse.io/v1alpha3"
)

func TestServiceNames(t *testing.T) {
	assert.Equal(t, "d8:prb:viewers", PRBServiceName("viewers"))
	assert.Equal(t, "d8:cprb:admins", CPRBServiceName("admins"))
}

func TestIsRoleAllowed(t *testing.T) {
	allowed := []string{
		"d8:project:viewer",
		"d8:namespace:admin",
		"d8:project-capability:manage-rbac",
		"d8:namespace-capability:view",
		"d8:custom:my-role",
	}
	for _, name := range allowed {
		assert.Truef(t, IsRoleAllowed(name), "expected %q to be allowed", name)
	}

	denied := []string{
		"cluster-admin",
		"d8:system:masters",
		"d8:user-authz:admin",
		"admin",
		"",
	}
	for _, name := range denied {
		assert.Falsef(t, IsRoleAllowed(name), "expected %q to be denied", name)
	}
}

func TestProjectNamespaceNames(t *testing.T) {
	// no status: only the main namespace
	p := &v1alpha3.Project{}
	p.Name = "foo"
	assert.Equal(t, []string{"foo"}, ProjectNamespaceNames(p))

	// status without the main namespace: it is appended
	p.Status.Namespaces = []v1alpha3.NamespaceStatus{{Name: "foo-extra"}}
	assert.ElementsMatch(t, []string{"foo-extra", "foo"}, ProjectNamespaceNames(p))

	// status with the main namespace: not duplicated
	p.Status.Namespaces = []v1alpha3.NamespaceStatus{{Name: "foo"}, {Name: "foo-extra"}}
	got := ProjectNamespaceNames(p)
	assert.ElementsMatch(t, []string{"foo", "foo-extra"}, got)
	assert.Len(t, got, 2)
}

func TestCopySubjects(t *testing.T) {
	assert.Nil(t, CopySubjects(nil))

	in := []rbacv1.Subject{{Kind: "User", Name: "alice"}}
	out := CopySubjects(in)
	assert.Equal(t, in, out)

	// mutating the copy must not affect the source
	out[0].Name = "bob"
	assert.Equal(t, "alice", in[0].Name)
}
