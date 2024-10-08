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

package module

import (
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

type DeckhouseModule struct {
	basic *modules.BasicModule

	description string
	stage       string
	labels      map[string]string
}

func NewDeckhouseModule(def DeckhouseModuleDefinition, staticValues utils.Values, configBytes, valuesBytes []byte) (*DeckhouseModule, error) {
	basic, err := modules.NewBasicModule(def.Name, def.Path, def.Weight, staticValues, configBytes, valuesBytes)
	if err != nil {
		return nil, fmt.Errorf("new basic module: %w", err)
	}

	labels := make(map[string]string, len(def.Tags))
	for _, tag := range def.Tags {
		labels["module.deckhouse.io/"+tag] = ""
	}

	if len(def.Tags) == 0 {
		labels = calculateLabels(def.Name)
	}

	return &DeckhouseModule{
		basic:       basic,
		labels:      labels,
		description: def.Description,
		stage:       def.Stage,
	}, nil
}

func (dm DeckhouseModule) GetBasicModule() *modules.BasicModule {
	return dm.basic
}

func (dm DeckhouseModule) AsKubeObject(source string) *v1alpha1.Module {
	if source == "" {
		source = "Embedded"
	}
	return &v1alpha1.Module{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha1.ModuleGVK.Kind,
			APIVersion: v1alpha1.ModuleGVK.Group + "/" + v1alpha1.ModuleGVK.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   dm.basic.Name,
			Labels: dm.labels,
		},
		Properties: v1alpha1.ModuleProperties{
			Weight:      dm.basic.Order,
			Source:      source,
			State:       "Disabled",
			Stage:       dm.stage,
			Description: dm.description,
		},
	}
}

func calculateLabels(name string) map[string]string {
	// could be removed when we will ready properties from the module.yaml file
	labels := make(map[string]string, 0)

	if strings.HasPrefix(name, "cni-") {
		labels["module.deckhouse.io/cni"] = ""
	}

	if strings.HasPrefix(name, "cloud-provider-") {
		labels["module.deckhouse.io/cloud-provider"] = ""
	}

	if strings.HasSuffix(name, "-crd") {
		labels["module.deckhouse.io/crd"] = ""
	}

	return labels
}

func getFirstDefined(values *go_hook.PatchableValues, keys ...string) (gjson.Result, bool) {
	var (
		v  gjson.Result
		ok bool
	)

	for i := range keys {
		v, ok = values.GetOk(keys[i])
		if ok {
			return v, ok
		}
	}

	return v, ok
}

func GetValuesFirstDefined(input *go_hook.HookInput, keys ...string) (v gjson.Result, ok bool) {
	return getFirstDefined(input.Values, keys...)
}

func GetConfigValuesFirstDefined(input *go_hook.HookInput, keys ...string) (v gjson.Result, ok bool) {
	return getFirstDefined(input.ConfigValues, keys...)
}

func GetHTTPSMode(moduleName string, input *go_hook.HookInput) string {
	var (
		modulePath = moduleName + ".https.mode"
		globalPath = "global.modules.https.mode"
	)

	v, ok := GetValuesFirstDefined(input, modulePath, globalPath)

	if ok {
		return v.String()
	}

	panic("https mode is not defined")
}

// IsEnabled check module on enable. moduleName should be in `kebab-case` without order prefix
func IsEnabled(moduleName string, input *go_hook.HookInput) bool {
	return set.NewFromValues(input.Values, "global.enabledModules").Has(moduleName)
}

func GetPublicDomain(moduleName string, input *go_hook.HookInput) string {
	template := input.ConfigValues.Get("global.modules.publicDomainTemplate").String()

	if len(strings.Split(template, "%s")) == 2 {
		return fmt.Sprintf(template, moduleName)
	}
	panic("ERROR: global.modules.publicDomainTemplate must contain '%s'.")
}

func GetIngressClass(moduleName string, input *go_hook.HookInput) string {
	var (
		modulePath = moduleName + ".ingressClass"
		globalPath = "global.modules.ingressClass"
	)

	v, ok := GetValuesFirstDefined(input, modulePath, globalPath)

	if ok {
		return v.String()
	}

	panic("ingress class is not defined")
}

func GetHTTPSSecretName(prefix string, moduleName string, input *go_hook.HookInput) string {
	var (
		modulePath = moduleName + ".https.mode"
		globalPath = "global.modules.https.mode"
	)
	httpsMode, _ := GetValuesFirstDefined(input, modulePath, globalPath)
	switch httpsMode.String() {
	case "CustomCertificate":
		return fmt.Sprintf("%s-customcertificate", prefix)
	case "CertManager":
		return prefix
	case "OnlyInURI":
		return ""
	default:
		input.LogEntry.Warnf("ERROR: https.mode must be in [CertManager, CustomCertificate, OnlyInURI], returning %s", prefix)
		return prefix
	}
}

func GetCertificateIssuerName(moduleName string, input *go_hook.HookInput) string {
	var (
		modulePath = moduleName + ".https.certManager.clusterIssuerName"
		globalPath = "global.modules.https.certManager.clusterIssuerName"
	)

	v, ok := GetValuesFirstDefined(input, modulePath, globalPath)

	if ok {
		return v.String()
	}

	panic("certmanager clusterIssuerName is not defined")
}
