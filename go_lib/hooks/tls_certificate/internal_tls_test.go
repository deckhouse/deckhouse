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
	"context"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkpatchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"
)

func testGetClusterDomainValues(t *testing.T, domain string) sdkpkg.PatchableValuesCollector {
	patchableValues, err := sdkpatchablevalues.NewPatchableValues(map[string]interface{}{
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
	res1 := f(context.TODO(), &go_hook.HookInput{Values: patchableValues1})

	require.Equal(t, []string{
		"conversion-webhook-handler.d8-system.svc",
		"conversion-webhook-handler.d8-system.svc.example1.com",
	}, res1)

	patchableValues2 := testGetClusterDomainValues(t, "example2.com")
	res2 := f(context.TODO(), &go_hook.HookInput{Values: patchableValues2})

	require.Equal(t, []string{
		"conversion-webhook-handler.d8-system.svc",
		"conversion-webhook-handler.d8-system.svc.example2.com",
	}, res2)
}

func TestCACommonNameDefaultsToCN(t *testing.T) {
	require.Equal(t, "module-name", GenSelfSignedTLSHookConf{CN: "module-name"}.caCommonName())
	require.Equal(t, "module-name-ca",
		GenSelfSignedTLSHookConf{CN: "module-name", CACN: "module-name-ca"}.caCommonName())
}

func TestGenerateNewSelfSignedTLSSignsWithNamedCA(t *testing.T) {
	// A CA with a name of its own makes cfssl add the authority key identifier,
	// which is what an OpenSSL client needs to build the chain.
	input := &go_hook.HookInput{Logger: log.NewNop()}

	cert, err := generateNewSelfSignedTLS(input, "module-name-ca", "module-name",
		[]string{"127.0.0.1", "module.d8-module.svc"},
		[]string{"signing", "key encipherment", "server auth"})
	require.NoError(t, err)

	ca, err := certificate.ParseCertificate(cert.CA)
	require.NoError(t, err)
	require.Equal(t, "module-name-ca", ca.Subject.CommonName)

	leaf, err := certificate.ParseCertificate(cert.Cert)
	require.NoError(t, err)
	require.Equal(t, "module-name", leaf.Subject.CommonName)
	require.NotEmpty(t, leaf.AuthorityKeyId, "an OpenSSL client cannot build the chain without it")
}

func TestIsCAWithOtherCommonName(t *testing.T) {
	input := &go_hook.HookInput{Logger: log.NewNop()}

	cert, err := generateNewSelfSignedTLS(input, "module-name-ca", "module-name",
		[]string{"127.0.0.1"}, []string{"signing", "key encipherment", "server auth"})
	require.NoError(t, err)

	renamed, err := isCAWithOtherCommonName(cert.CA, "module-name-ca")
	require.NoError(t, err)
	require.False(t, renamed, "the configured name is unchanged")

	renamed, err = isCAWithOtherCommonName(cert.CA, "module-name")
	require.NoError(t, err)
	require.True(t, renamed, "a CA named otherwise is replaced")

	renamed, err = isCAWithOtherCommonName("", "module-name-ca")
	require.NoError(t, err)
	require.False(t, renamed, "an empty CA is already handled by isOutdatedCA")
}
