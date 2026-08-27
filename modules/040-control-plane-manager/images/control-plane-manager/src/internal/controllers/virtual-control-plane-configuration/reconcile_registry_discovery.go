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

package virtualcontrolplaneconfiguration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	registryConfigSecretNS   = "d8-system"
	registryConfigSecretName = "registry-config"
	registryModeLocal        = "Local"
)

// Local mode has no external upstream reachable from tenant nodes; direct pull impossible.
var errRegistryLocalNoUpstream = errors.New(
	"registry mode Local has no external upstream reachable from tenant nodes; direct tenant pull is unsupported",
)

// tenantRegistry is the external upstream tenant nodes pull from directly, bypassing the parent in-cluster proxy (module 038).
// Discovered from d8-system/registry-config (operator input); holds the real external repo even when 038 is Direct/Proxy,
// unlike global.modulesImages.registry, which points at the in-cluster proxy.
type tenantRegistry struct {
	Address          string // host[:port]
	Path             string // "/deckhouse/ee"
	Scheme           string // "https"/"http"
	CA               string // PEM, optional
	DockerConfigJSON []byte
}

// Base = "address/path", the image ref base (== imagesRepo).
func (t *tenantRegistry) Base() string { return t.Address + t.Path }

// registrySecretData renders deckhouse-registry .data (keys per global-hooks/discovery/deckhouse_registry.go).
func (t *tenantRegistry) registrySecretData() map[string][]byte {
	data := map[string][]byte{
		".dockerconfigjson": t.DockerConfigJSON,
		"address":           []byte(t.Address),
		"path":              []byte(t.Path),
		"scheme":            []byte(t.Scheme),
	}
	if t.CA != "" {
		data["ca"] = []byte(t.CA)
	}
	return data
}

// discoverTenantRegistry reads d8-system/registry-config for the external upstream.
// (nil, nil) when absent or no external repo (non-configurable Unmanaged) so callers clone the parent secret; Local returns errRegistryLocalNoUpstream.
func (r *reconciler) discoverTenantRegistry(ctx context.Context) (*tenantRegistry, error) {
	sec, err := r.getSecret(ctx, registryConfigSecretNS, registryConfigSecretName)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", registryConfigSecretNS, registryConfigSecretName, err)
	}

	if string(sec.Data["mode"]) == registryModeLocal {
		return nil, errRegistryLocalNoUpstream
	}

	imagesRepo := string(sec.Data["imagesRepo"])
	if imagesRepo == "" {
		// non-configurable Unmanaged: no imagesRepo -> fall back to parent clone.
		return nil, nil
	}

	address, path := splitAddressAndPath(imagesRepo)
	scheme := strings.ToLower(string(sec.Data["scheme"]))
	if scheme == "" {
		scheme = "https"
	}

	dockerCfg, err := buildDockerConfigJSON(address, string(sec.Data["username"]), string(sec.Data["password"]))
	if err != nil {
		return nil, fmt.Errorf("build dockerconfigjson: %w", err)
	}

	return &tenantRegistry{
		Address:          address,
		Path:             path,
		Scheme:           scheme,
		CA:               string(sec.Data["ca"]),
		DockerConfigJSON: dockerCfg,
	}, nil
}

// resolveTenantRegistryData returns deckhouse-registry .data: discovered external upstream when tr is set, else a clone of the parent secret.
func resolveTenantRegistryData(parent *corev1.Secret, tr *tenantRegistry) map[string][]byte {
	if tr == nil {
		return maps.Clone(parent.Data)
	}
	return tr.registrySecretData()
}

// splitAddressAndPath: "reg.io/a/b" -> ("reg.io", "/a/b").
func splitAddressAndPath(ref string) (address, path string) {
	parts := strings.SplitN(ref, "/", 2)
	if len(parts) < 2 {
		return parts[0], ""
	}
	return parts[0], "/" + parts[1]
}

func buildDockerConfigJSON(address, username, password string) ([]byte, error) {
	type authEntry struct {
		Username string `json:"username,omitempty"`
		Password string `json:"password,omitempty"`
		Auth     string `json:"auth,omitempty"`
	}

	entry := authEntry{}
	if username != "" || password != "" {
		entry.Username = username
		entry.Password = password
		entry.Auth = base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	}

	return json.Marshal(map[string]map[string]authEntry{"auths": {address: entry}})
}

// rebaseImageRef swaps the base of "<base>@<digest>", keeping the digest. Moves VCP-baked tenant-node refs off the in-cluster registry onto the external upstream.
func rebaseImageRef(ref, newBase string) string {
	at := strings.LastIndex(ref, "@")
	if at < 0 || newBase == "" {
		return ref
	}
	return newBase + ref[at:]
}
