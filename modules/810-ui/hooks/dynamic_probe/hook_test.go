/*
Copyright 2023 Flant JSC

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

package dynamic_probe

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: upmeter :: hooks :: dynamic_probes ::", func() {
	var (
		valuesPrefix = "upmeter.internal.dynamicProbes"
		initValues   = `{"upmeter":{"internal":{ "dynamicProbes":{} }}}`

		ingressControllerValuesPath = valuesPrefix + "." + "ingressControllerNames"
		nodegroupValuesPath         = valuesPrefix + "." + "cloudEphemeralNodeGroupNames"
		zonesValuesPath             = valuesPrefix + "." + "zones"
		zonePrefixValuesPath        = valuesPrefix + "." + "zonePrefix"

		f = HookExecutionConfigInit(initValues, `{}`)
	)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	DescribeTable("Objects discovery for dynamic probes",
		func(objectsYAMLs []string, want *names) {
			yamlState := strings.Join(objectsYAMLs, "\n---\n")
			f.BindingContexts.Set(f.KubeStateSet(yamlState))

			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())

			got := &names{
				IngressControllerNames:       f.ValuesGet(ingressControllerValuesPath).AsStringSlice(),
				CloudEphemeralNodeGroupNames: f.ValuesGet(nodegroupValuesPath).AsStringSlice(),
				Zones:                        f.ValuesGet(zonesValuesPath).AsStringSlice(),
				ZonePrefix:                   f.ValuesGet(zonePrefixValuesPath).String(),
			}

			Expect(got).To(Equal(want))
		},
		Entry("No object, no cloud", []string{}, emptyNames()),
		Entry(
			"Single ingress controller",
			[]string{
				discoveredIngressControllers("main"),
			},
			emptyNames().WithIngressControllers("main"),
		),
		Entry(
			"Two ingress controllers",
			[]string{
				discoveredIngressControllers("main", "backup"),
			},
			emptyNames().WithIngressControllers("backup", "main"),
		),
		Entry(
			"Single nodegroup",
			[]string{
				clodProviderSecretWithZonesYAML("zone"),
				discoveredNodeGroups("worker"),
			},
			emptyNames().WithNodeGroups("worker").WithZones("zone"),
		),
		Entry(
			"Two nodegroups",
			[]string{
				clodProviderSecretWithZonesYAML("zone"),
				discoveredNodeGroups("frontend", "system"),
			},
			emptyNames().WithNodeGroups("frontend", "system").WithZones("zone"),
		),
		Entry(
			"Nodegroups are ignored when zones absent",
			[]string{
				discoveredNodeGroups("frontend", "system"),
			},
			emptyNames(),
		),
		Entry(
			"All names are sorted",
			[]string{
				discoveredIngressControllers("ing-b", "ing-a"),
				discoveredNodeGroups("ng-b", "ng-a"),
				clodProviderSecretWithZonesYAML("aaa", "ccc", "bbb"),
			},
			emptyNames().
				WithIngressControllers("ing-a", "ing-b").
				WithNodeGroups("ng-a", "ng-b").
				WithZones("aaa", "bbb", "ccc"),
		),
		Entry(
			"Region is empty in case of non-Azure cloud",
			[]string{
				clodProviderSecretWithRegionAndZonesYAML("gcp", "west", "aaa", "bbb"),
			},
			emptyNames().
				WithZones("aaa", "bbb"),
		),
		Entry(
			"Region is parsed and appended to zones in case of Azure cloud",
			[]string{
				clodProviderSecretWithRegionAndZonesYAML("azure", "west", "aaa", "bbb"),
			},
			emptyNames().
				WithZones("west-aaa", "west-bbb").
				WithZonePrefix("west"),
		),
	)
})

func discoveredNodeGroups(names ...string) string {
	tpl := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: upmeter-discovery-cloud-ephemeral-nodegroups
  namespace: d8-cloud-instance-manager
data:
  names: %q
`
	namesJSON, err := json.Marshal(names)
	if err != nil {
		panic("marshalling names to YAML: " + err.Error())
	}
	return fmt.Sprintf(tpl, namesJSON)
}

func discoveredIngressControllers(names ...string) string {
	tpl := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: upmeter-discovery-ingress-controllers
  namespace: d8-ingress-nginx
data:
  names: %q
`
	namesJSON, err := json.Marshal(names)
	if err != nil {
		panic("marshalling names to YAML: " + err.Error())
	}
	return fmt.Sprintf(tpl, namesJSON)
}

func clodProviderSecretWithZonesYAML(zones ...string) string {
	tpl := `
apiVersion: v1
data:
  zones: %s
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
type: Opaque
`
	zz, err := json.Marshal(zones)
	if err != nil {
		panic("marshalling zones to YAML: " + err.Error())
	}
	b64 := base64.StdEncoding.EncodeToString(zz)
	return fmt.Sprintf(tpl, b64)
}

func clodProviderSecretWithRegionAndZonesYAML(provider, region string, zones ...string) string {
	tpl := `
apiVersion: v1
data:
  type: %s
  region: %s
  zones: %s
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
type: Opaque
`
	zz, err := json.Marshal(zones)
	if err != nil {
		panic("marshalling zones to YAML: " + err.Error())
	}
	return fmt.Sprintf(tpl,
		base64.StdEncoding.EncodeToString([]byte(provider)),
		base64.StdEncoding.EncodeToString([]byte(region)),
		base64.StdEncoding.EncodeToString(zz),
	)
}
