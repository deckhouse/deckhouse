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

package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha3"
)

// A project on its way out and a virtual project have no additional namespaces to be confused with,
// so neither makes a name read like one.
func TestNameWarnings_LeavesOutProjectsWithoutNamespaces(t *testing.T) {
	now := metav1.Now()
	deleted := v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo", DeletionTimestamp: &now}}
	virtual := v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "default"}, Spec: v1alpha3.ProjectSpec{ProjectTemplateName: "virtual"}}

	assert.Empty(t, nameWarnings("foo-bar", []v1alpha3.Project{deleted}))
	assert.Empty(t, nameWarnings("foo", []v1alpha3.Project{{ObjectMeta: metav1.ObjectMeta{Name: "foo-bar", DeletionTimestamp: &now}}}))
	assert.Empty(t, nameWarnings("default-x", []v1alpha3.Project{virtual}))
}
