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

package v1alpha2

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestStaticInstance_validateAddressUnlessAdopting(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		annotations map[string]string
		address     string
		nodes       []*corev1.Node
		errContains string
	}{
		"free address without annotations is allowed": {
			address: testAddress,
			nodes:   []*corev1.Node{nodeWithAddresses("other", "192.168.199.11")},
		},
		"taken address without annotations is rejected": {
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			errContains: "already exists on node",
		},
		"rejection hints at the declarative annotation": {
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
			errContains: AdoptIfNodeExistsAnnotation,
		},
		"taken address is allowed with adopt-if-node-exists": {
			annotations: map[string]string{AdoptIfNodeExistsAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
		},
		"taken address is allowed with skip-bootstrap-phase": {
			annotations: map[string]string{SkipBootstrapPhaseAnnotation: ""},
			address:     testAddress,
			nodes:       []*corev1.Node{nodeWithAddresses("existing", testAddress)},
		},
		"empty address without annotations is rejected": {
			address:     "",
			nodes:       nil,
			errContains: "must not be empty",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			instance := staticInstanceWithAnnotations(tt.address, tt.annotations)

			err := instance.validateAddressUnlessAdopting(t.Context(), fakeClientWithNodes(t, tt.nodes...))

			if tt.errContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.errContains)
			}
		})
	}
}
