/*
Copyright 2021 Flant JSC

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

package tls_certificate

import (
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/require"
)

func testGetClusterDomainValues(t *testing.T, domain string) *go_hook.PatchableValues {
	patchableValues, err := go_hook.NewPatchableValues(map[string]interface{}{
		"global": map[string]interface{}{
			"discovery": map[string]interface{}{
				"clusterDomain": domain,
			},
		},
	})
	require.NoError(t, err)
	return patchableValues
}

func TestDefaultSANs(t *testing.T) {
	orig := []string{
		"conversion-webhook-handler.d8-system.svc",
		ClusterDomainSAN("conversion-webhook-handler.d8-system.svc"),
	}
	f := DefaultSANs(orig)

	patchableValues1 := testGetClusterDomainValues(t, "example1.com")
	res1 := f(&go_hook.HookInput{Values: patchableValues1})

	require.Equal(t, []string{
		"conversion-webhook-handler.d8-system.svc",
		"conversion-webhook-handler.d8-system.svc.example1.com",
	}, res1)

	patchableValues2 := testGetClusterDomainValues(t, "example2.com")
	res2 := f(&go_hook.HookInput{Values: patchableValues2})

	require.Equal(t, []string{
		"conversion-webhook-handler.d8-system.svc",
		"conversion-webhook-handler.d8-system.svc.example2.com",
	}, res2)
}
