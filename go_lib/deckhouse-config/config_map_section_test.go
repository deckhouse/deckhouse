/*
Copyright 2022 Flant JSC

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

package deckhouse_config

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

func TestConfigMapSectionToConfigMapData(t *testing.T) {
	const (
		ExpectNoError = false
		ExpectError   = true
	)

	tests := []struct {
		name        string
		in          *configMapSection
		out         map[string]string
		expectError bool
	}{
		{
			"no values, no enabled",
			&configMapSection{
				name:      "module-one",
				valuesKey: "moduleOne",
				values:    nil,
				enabled:   nil,
			},
			map[string]string{},
			ExpectNoError,
		},
		{
			"no values",
			&configMapSection{
				name:      "module-one",
				valuesKey: "moduleOne",
				values:    nil,
				enabled:   pointer.BoolPtr(true),
			},
			map[string]string{
				"moduleOneEnabled": "true",
			},
			ExpectNoError,
		},
		{
			"empty values",
			&configMapSection{
				name:      "module-one",
				valuesKey: "moduleOne",
				values: map[string]interface{}{
					"moduleOne": nil,
				},
				enabled: pointer.BoolPtr(true),
			},
			map[string]string{
				"moduleOneEnabled": "true",
			},
			ExpectNoError,
		},
		{
			"no enabled",
			&configMapSection{
				name:      "module-one",
				valuesKey: "moduleOne",
				values: map[string]interface{}{
					"moduleOne": map[string]interface{}{
						"paramGroup": map[string]interface{}{
							"param1": "value1",
						},
					},
				},
				enabled: nil,
			},
			map[string]string{
				"moduleOne": "paramGroup:\n  param1: value1\n",
			},
			ExpectNoError,
		},
		{
			"values and enabled",
			&configMapSection{
				name:      "module-one",
				valuesKey: "moduleOne",
				values: map[string]interface{}{
					"moduleOne": map[string]interface{}{
						"paramGroup": map[string]interface{}{
							"param1": "value1",
						},
					},
				},
				enabled: pointer.BoolPtr(true),
			},
			map[string]string{
				"moduleOne":        "paramGroup:\n  param1: value1\n",
				"moduleOneEnabled": "true",
			},
			ExpectNoError,
		},
		{
			"wrong values",
			&configMapSection{
				name:      "module-one",
				valuesKey: "moduleOne",
				values: map[string]interface{}{
					"moduleOne": map[interface{}]interface{}{
						1000: "value1",
					},
				},
				enabled: pointer.BoolPtr(true),
			},
			nil,
			ExpectError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			data, err := tt.in.getConfigMapData()

			if tt.expectError == ExpectError {
				g.Expect(err).Should(HaveOccurred())
				return
			}

			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(data).ShouldNot(BeNil(), "data should not be nil")
			g.Expect(data).Should(HaveLen(len(tt.out)))
			for k, v := range tt.out {
				g.Expect(data).Should(HaveKey(k), "result should have %s key, got %+v", k, data)
				g.Expect(data[k]).Should(BeEquivalentTo(v), "%s field should be equal to %v, got %+v", k, v, data)
			}
		})
	}
}
