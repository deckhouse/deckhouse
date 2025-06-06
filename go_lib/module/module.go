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
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/tidwall/gjson"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

func getFirstDefined(values sdkpkg.PatchableValuesCollector, keys ...string) (gjson.Result, bool) {
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

func GetValuesFirstDefined(input *go_hook.HookInput, keys ...string) (gjson.Result, bool) {
	return getFirstDefined(input.Values, keys...)
}

func GetConfigValuesFirstDefined(input *go_hook.HookInput, keys ...string) (gjson.Result, bool) {
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
		input.Logger.Warn("ERROR: https.mode must be in [CertManager, CustomCertificate, OnlyInURI], returning", slog.String("value", prefix))
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
