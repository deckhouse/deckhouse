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

package checksum

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

// componentChecksumDeps sets the dependencies of the component's checksum on the keys of the control_plane_config secret.
// The map is based on the control_plane_config template in daemonset.yaml.
var componentChecksumDeps = map[string]componentFieldMap{
	"kube-apiserver": {
		checksumDependsOn: []string{
			"kube-apiserver-full.yaml.tpl",
			"extra-file-admission-control-config.yaml",
			"extra-file-audit-policy.yaml",
			"extra-file-authn-webhook-config.yaml",
			"extra-file-event-rate-limit-config.yaml",
			"extra-file-oidc-ca.crt",
			"extra-file-secret-encryption-config.yaml",
			"extra-file-webhook-config.yaml",
		},
	},
	"etcd": {
		checksumDependsOn: []string{
			"etcd-full.yaml.tpl",
		},
	},
	"kube-controller-manager": {
		checksumDependsOn: []string{
			"kube-controller-manager-full.yaml.tpl",
		},
	},
	"kube-scheduler": {
		checksumDependsOn: []string{
			"kube-scheduler-full.yaml.tpl",
			"extra-file-scheduler-config.yaml",
		},
	},
}

var hotReloadChecksumDependsOn = []string{
	"extra-file-authentication-config.yaml",
	"extra-file-authorization-config.yaml",
}

type componentFieldMap struct {
	// secret keys of d8-control-plane-manager-config, on which the component's checksum depends.
	checksumDependsOn []string
}

// sortedKeysFromMap returns a sorted slice of keys from the map.
func sortedKeysFromMap(data map[string][]byte) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedKeysFromSlice returns a sorted slice of keys from candidates that exist in data.
func sortedKeysFromSlice(candidates []string, data map[string][]byte) []string {
	keys := make([]string, 0, len(candidates))
	for _, k := range candidates {
		if _, has := data[k]; has {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

// hashKeys returns SHA256 hex of the concatenated values for the given keys.
func hashKeys(secretData map[string][]byte, keys []string) string {
	h := sha256.New()
	for _, k := range keys {
		h.Write(secretData[k])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// ComponentChecksum calculates the checksum of the component according to the d8-control-plane-manager-config secret.
// Inside, it hashes the data from componentChecksumDeps[component] keys in sorted order.
func ComponentChecksum(secretData map[string][]byte, component string) (string, error) {
	keys, err := collectDependencyData(secretData, component)
	if err != nil {
		return "", err
	}
	return hashKeys(secretData, keys), nil
}

// collectDependencyData returns sorted keys from componentChecksumDeps[component] that exist in secretData.
// Missing keys are skipped (conditional files may not be present in the secret).
func collectDependencyData(secretData map[string][]byte, component string) ([]string, error) {
	fieldMap, ok := componentChecksumDeps[component]
	if !ok {
		return nil, fmt.Errorf("unknown component %q", component)
	}
	return sortedKeysFromSlice(fieldMap.checksumDependsOn, secretData), nil
}

// PKIChecksum calculates the total checksum of all the keys of the pki secret based only on the values in the secret.
// Keys names are ignored for the checksum calculation.
func PKIChecksum(pkiSecretData map[string][]byte) (string, error) {
	return hashKeys(pkiSecretData, sortedKeysFromMap(pkiSecretData)), nil
}

func HotReloadChecksum(secretData map[string][]byte) (string, error) {
	return hashKeys(secretData, sortedKeysFromSlice(hotReloadChecksumDependsOn, secretData)), nil
}
