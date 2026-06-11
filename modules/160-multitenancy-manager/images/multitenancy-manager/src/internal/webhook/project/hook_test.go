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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"controller/apis/deckhouse.io/v1alpha3"
)

func TestValidateStandardFields(t *testing.T) {
	cases := []struct {
		name    string
		project *v1alpha3.Project
		denied  bool
	}{
		{
			name:    "empty is valid",
			project: &v1alpha3.Project{},
		},
		{
			name: "valid administrators and quota",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "Group", Name: "team"}},
				Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("2")},
			}},
		},
		{
			name: "invalid administrator kind",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "ServiceAccount", Name: "robot"}},
			}},
			denied: true,
		},
		{
			name: "empty administrator name",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: ""}},
			}},
			denied: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := validateStandardFields(tc.project)
			if tc.denied {
				assert.NotEmpty(t, msg)
			} else {
				assert.Empty(t, msg)
			}
		})
	}
}
