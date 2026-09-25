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

package template_tests

import (
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"k8s.io/utils/ptr"
)

// user_authn_gateway_api_enabled reports the configured flag only, while helm_lib_module_gateway_enabled
// additionally demands a resolvable default Gateway. Authenticator routes need the former: they name
// their ListenerSet in parentRefs and never fall back to the module gateway.
func TestGatewayAPIEnabledHelper(t *testing.T) {
	library, err := loader.Load("../../../helm_lib/charts/deckhouse_lib_helm")
	require.NoError(t, err)
	helpers, err := os.ReadFile("../templates/_helpers.tpl")
	require.NoError(t, err)
	c := &chart.Chart{
		Metadata: &chart.Metadata{Name: "user-authn", Version: "0.1.0"},
		Templates: []*chart.File{
			{Name: "templates/_helpers.tpl", Data: helpers},
			{Name: "templates/result", Data: []byte(`{{ include "user_authn_gateway_api_enabled" . }}|{{ include "helm_lib_module_gateway_enabled" . }}`)},
		},
	}
	c.AddDependency(library)
	for _, tc := range []struct {
		name           string
		global, module *bool
		enabled        bool
	}{
		{name: "default", enabled: true},
		{name: "global enabled", global: ptr.To(true), enabled: true},
		{name: "global disabled", global: ptr.To(false)},
		{name: "module enabled", module: ptr.To(true), enabled: true},
		{name: "module disabled", module: ptr.To(false)},
		{name: "module overrides global disable", global: ptr.To(false), module: ptr.To(true), enabled: true},
		{name: "module overrides global enable", global: ptr.To(true), module: ptr.To(false)},
	} {
		for _, gatewayPresent := range []bool{false, true} {
			t.Run(tc.name+"/gateway="+strconv.FormatBool(gatewayPresent), func(t *testing.T) {
				globalSettings, moduleSettings := map[string]any{}, map[string]any{}
				if tc.global != nil {
					globalSettings["gatewayAPI"] = map[string]any{"enabled": *tc.global}
				}
				if tc.module != nil {
					moduleSettings["gatewayAPI"] = map[string]any{"enabled": *tc.module}
				}
				discovery := map[string]any{}
				if gatewayPresent {
					discovery["gatewayAPIDefaultGateway"] = map[string]any{"name": "shared", "namespace": "d8-alb"}
				}
				values, err := chartutil.ToRenderValues(c, map[string]any{"global": map[string]any{"modules": globalSettings, "discovery": discovery}, "userAuthn": moduleSettings}, chartutil.ReleaseOptions{Name: "test"}, nil)
				require.NoError(t, err)
				rendered, err := engine.Render(c, values)
				require.NoError(t, err)
				require.Equal(t, strconv.FormatBool(tc.enabled)+"|"+strconv.FormatBool(tc.enabled && gatewayPresent), rendered["user-authn/templates/result"])
			})
		}
	}
}
