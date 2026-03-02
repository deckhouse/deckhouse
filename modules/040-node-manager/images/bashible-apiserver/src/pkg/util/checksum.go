/*
Copyright 2025 Flant JSC

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

package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"bashible-apiserver/pkg/template"
)

const ConfigurationChecksumAnnotation = "bashible.deckhouse.io/configuration-checksum"

func SetConfigurationChecksumAnnotation(ctx template.Context, ng string, meta *metav1.ObjectMeta) {
	if checksum, ok := ctx.GetConfigurationChecksum(ng); ok {
		if meta.Annotations == nil {
			meta.Annotations = map[string]string{}
		}
		meta.Annotations[ConfigurationChecksumAnnotation] = checksum
	}
}
