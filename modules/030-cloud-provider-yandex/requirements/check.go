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
	"errors"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	yandexDeprecatedZoneInConfigKey             = "yandex:hasDeprecatedZoneInConfig"
	yandexDeprecatedZoneInNodesKey              = "yandex:hasDeprecatedZoneInNodes"
	yandexDeprecatedZoneInConfigRequirementsKey = "yandexHasDeprecatedZoneInConfig"
	yandexDeprecatedZoneInNodesRequirementsKey  = "yandexHasDeprecatedZoneInNodes"
)

func init() {
	checkRequirementInConfigFunc := func(_ string, getter requirements.ValueGetter) (bool, error) {
		hasDeprecatedZone, exists := getter.Get(yandexDeprecatedZoneInConfigKey)
		if exists {
			if hasDeprecatedZone.(bool) {
				return false, errors.New("cluster use deprecated zone \"ru-central1-c\". Remove it from provider-cluster-config")
			}
		}

		return true, nil
	}
	checkRequirementInZonesFunc := func(_ string, getter requirements.ValueGetter) (bool, error) {
		hasDeprecatedZone, exists := getter.Get(yandexDeprecatedZoneInNodesKey)
		if exists {
			if hasDeprecatedZone.(bool) {
				return false, errors.New("cluster use deprecated zone \"ru-central1-c\". Remove it from NodeGroups and check nodes")
			}
		}

		return true, nil
	}

	requirements.RegisterCheck(yandexDeprecatedZoneInConfigRequirementsKey, checkRequirementInConfigFunc)
	requirements.RegisterCheck(yandexDeprecatedZoneInNodesRequirementsKey, checkRequirementInZonesFunc)
}
