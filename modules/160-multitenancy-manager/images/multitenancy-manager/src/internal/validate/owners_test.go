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

package validate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestAddedOwnerReference(t *testing.T) {
	a := metav1.OwnerReference{APIVersion: "v1", Kind: "Namespace", Name: "a", UID: "1"}
	b := metav1.OwnerReference{APIVersion: "v1", Kind: "Namespace", Name: "b", UID: "2"}
	controller := a
	controller.Controller = ptr.To(true)

	assert.Empty(t, AddedOwnerReference("Project", "p", nil, nil))
	assert.Empty(t, AddedOwnerReference("Project", "p", []metav1.OwnerReference{a, b}, []metav1.OwnerReference{b, a}), "the order does not matter")
	assert.Empty(t, AddedOwnerReference("Project", "p", []metav1.OwnerReference{a, b}, []metav1.OwnerReference{a}), "a removal passes")
	assert.Equal(t, `the ownerReferences of the "p" Project cannot be added or changed: the v1 Namespace "b" is not among them`,
		AddedOwnerReference("Project", "p", []metav1.OwnerReference{a}, []metav1.OwnerReference{a, b}))
	assert.NotEmpty(t, AddedOwnerReference("Project", "p", []metav1.OwnerReference{a}, []metav1.OwnerReference{controller}), "a changed entry is refused")
}
