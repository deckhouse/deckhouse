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

package requirements

import (
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	disruptionKey = "ingressNginx:hasDisruption"
)

func init() {
	disruptionCheckFunc := func(getter requirements.ValueGetter) (bool, string) {
		value, exist := getter.Get(disruptionKey)
		if !exist {
			return true, ""
		}

		hasDisruptionVersionUpdate := value.(bool)

		reason := ""
		if hasDisruptionVersionUpdate {
			reason = "Default IngressNginxController version 0.33 will be automatically changed to 1.1, this action will restart all controllers with non-specified version"
		}
		return hasDisruptionVersionUpdate, reason
	}

	requirements.RegisterDisruption("ingressNginx", disruptionCheckFunc)
}
