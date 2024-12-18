/*
Copyright 2024 Flant JSC

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

package deckhouse_release

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestEventFilter(t *testing.T) {
	eventFilter := newEventFilter()

	t.Run("Create Event", func(t *testing.T) {
		tests := []struct {
			name     string
			arg      event.CreateEvent
			expected bool
		}{
			{
				name: "Release without annotations",
				arg: event.CreateEvent{
					Object: &v1alpha1.DeckhouseRelease{
						TypeMeta: metav1.TypeMeta{
							Kind:       v1alpha1.DeckhouseReleaseKind,
							APIVersion: v1alpha1.DeckhouseReleaseGVK.GroupVersion().String(),
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "v1.63.9",
						},
						Spec: v1alpha1.DeckhouseReleaseSpec{
							Version: "v1.63.9",
						},
					},
				},
				expected: true,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				got := eventFilter.Create(test.arg)
				if got != test.expected {
					t.Errorf("Event:\n%v\ngot %t\nexpected %t", string(must(yaml.Marshal(test.arg))), got, test.expected)
				}
			})
		}
	})
}

func must[T any](val T, err error) T {
	if err != nil {
		panic(err)
	}

	return val
}
