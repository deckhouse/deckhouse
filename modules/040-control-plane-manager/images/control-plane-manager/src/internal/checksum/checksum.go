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
	"encoding/binary"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
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

// CalculateComponentChecksum calculates the checksum of the component according to the control_plane_config secret.
// Inside, it collects a manifest from componentChecksumDeps[component] keys and hashes it.
func CalculateComponentChecksum(secretData map[string][]byte, component string) (string, error) {
	manifest, err := buildComponentManifest(secretData, component)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := h.Write(manifest); err != nil {
		return "", fmt.Errorf("failed to hash manifest: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// buildComponentManifest collects bytes to calculate the component's checksum from the secret data.
// Takes only keys from componentChecksumDeps[component], sorts and concatenates the values.
// Missing keys are skipped (conditional files may not be present in the secret).
func buildComponentManifest(secretData map[string][]byte, component string) ([]byte, error) {
	fieldMap, ok := componentChecksumDeps[component]
	if !ok {
		return nil, fmt.Errorf("unknown component %q", component)
	}
	keys := make([]string, 0, len(fieldMap.checksumDependsOn))
	for _, k := range fieldMap.checksumDependsOn {
		if _, has := secretData[k]; has {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var manifest []byte
	for _, k := range keys {
		manifest = append(manifest, secretData[k]...)
	}
	return manifest, nil
}

// calculatePKIChecksum calculates the total checksum of all the keys of the pki secret.
func CalculatePKIChecksum(pkiSecret *corev1.Secret) (string, error) {
	h := sha256.New()

	keys := make([]string, 0, len(pkiSecret.Data))
	for key := range pkiSecret.Data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		h.Write([]byte(key))
		h.Write(pkiSecret.Data[key])
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func BuildHotReloadChecksum(secretData map[string][]byte) (string, error) {
	keys := make([]string, 0, len(hotReloadChecksumDependsOn))
	for _, k := range hotReloadChecksumDependsOn {
		if _, has := secretData[k]; has {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var manifest []byte
	for _, k := range keys {
		manifest = append(manifest, secretData[k]...)
	}
	h := sha256.New()
	if _, err := h.Write(manifest); err != nil {
		return "", fmt.Errorf("failed to hash manifest: %w", err)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// componentOrder is the fixed order for including component checksums into ConfigurationGeneration.
var componentOrder = []string{"etcd", "kube-apiserver", "kube-controller-manager", "kube-scheduler"}

// CalculateConfigurationGeneration returns int64 generation from hashes of cmpSecret (via component + hotReload checksums) and pkiSecret (pkiChecksum).
// Any change in any of these checksums produces a different generation.
func CalculateConfigurationGeneration(pkiChecksum string, componentChecksums map[string]string, hotReloadChecksum string) int64 {
	h := sha256.New()
	h.Write([]byte(pkiChecksum))
	for _, component := range componentOrder {
		h.Write([]byte(componentChecksums[component]))
	}
	h.Write([]byte(hotReloadChecksum))
	sum := h.Sum(nil)
	// First 8 bytes as uint64, then mask to positive int64 range
	u := binary.BigEndian.Uint64(sum[:8])
	return int64(u & 0x7FFFFFFFFFFFFFFF)
}
