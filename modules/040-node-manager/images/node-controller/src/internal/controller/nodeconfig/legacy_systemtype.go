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

package nodeconfig

import (
	"cmp"
	"slices"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

var legacySystemTypes = map[internalv1alpha1.SystemType]internalv1alpha1.SystemType{
	internalv1alpha1.SystemTypeDeckhouseEngine: internalv1alpha1.SystemTypeLegacyImmutable,
	internalv1alpha1.SystemTypeGenericOS:       internalv1alpha1.SystemTypeLegacyMutable,
}

func publishedSystemType(desired internalv1alpha1.NodeSpec, existing *internalv1alpha1.NodeConfig) internalv1alpha1.SystemType {
	if existing != nil && (keepsNewSpelling(desired, existing) || runsShippedNodelet(desired, existing)) {
		return desired.SystemType
	}
	return cmp.Or(legacySystemTypes[desired.SystemType], desired.SystemType)
}

func keepsNewSpelling(desired internalv1alpha1.NodeSpec, existing *internalv1alpha1.NodeConfig) bool {
	if existing.Spec.SystemType != desired.SystemType {
		return false
	}
	_, refused := configRefused(existing, []string{"Rejected"})
	return !refused
}

func runsShippedNodelet(desired internalv1alpha1.NodeSpec, existing *internalv1alpha1.NodeConfig) bool {
	if !applied(existing) {
		return false
	}
	i := slices.IndexFunc(desired.Extensions, func(ext internalv1alpha1.Extension) bool { return ext.Name == nodeletExtension })
	if i < 0 {
		return false
	}
	return slices.ContainsFunc(existing.Status.Extensions, func(status internalv1alpha1.ExtensionStatus) bool {
		return status.Name == nodeletExtension && status.Digest == desired.Extensions[i].Digest && status.State == extensionStateReady
	})
}
