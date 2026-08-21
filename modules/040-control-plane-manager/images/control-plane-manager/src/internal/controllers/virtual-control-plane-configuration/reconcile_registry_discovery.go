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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	registryConfigSecretNS   = "d8-system"
	registryConfigSecretName = "registry-config"
	registryModeLocal        = "Local"
)

// errRegistryLocalNoUpstream is returned when the parent runs the registry
// module in Local mode: there is no external upstream reachable from tenant
// worker nodes, so direct pull cannot work (this needs in-cluster routing).
var errRegistryLocalNoUpstream = errors.New(
	"registry mode Local has no external upstream reachable from tenant nodes; direct tenant pull is unsupported",
)

// tenantRegistry is the external upstream registry the tenant cluster and its
// worker nodes pull Deckhouse images from directly, bypassing the parent's in-cluster registry proxy (module 038).
//
// It is discovered from the operator's registry input Secret d8-system/registry-config (rendered by the deckhouse module 002 from ModuleConfig/deckhouse .spec.settings.registry).
// That Secret holds the real external upstream even when module 038 is Direct/Proxy, unlike global.modulesImages.registry, which points at the in-cluster proxy.
type tenantRegistry struct {
	Address          string // host[:port]
	Path             string // "/deckhouse/ee"
	Scheme           string // "https"/"http"
	CA               string // PEM, optional
	DockerConfigJSON []byte
}

// Base returns "address/path", the image reference base (== imagesRepo).
func (t *tenantRegistry) Base() string { return t.Address + t.Path }

// registrySecretData renders the tenant deckhouse-registry Secret .data.
// Keys mirror what the global discovery hook expects (see
// global-hooks/discovery/deckhouse_registry.go).
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

// discoverTenantRegistry reads d8-system/registry-config and returns the external upstream.
// It returns (nil, nil) when the Secret is absent or does not describe an external upstream (deprecated non-configurable Unmanaged) so callers fall
// back to cloning the parent deckhouse-registry Secret, which already points at the real registry in that case. Local mode returns errRegistryLocalNoUpstream.
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
		// Non-configurable Unmanaged (no imagesRepo): fall back to parent clone.
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

// resolveTenantRegistryData returns the deckhouse-registry Secret .data for the tenant:
// the discovered external upstream when available, otherwise a clone of the parent Secret.
// Discovery errors (including Local mode) are logged and fall back to the parent clone rather than failing the reconcile.
func (r *reconciler) resolveTenantRegistryData(ctx context.Context, parent *corev1.Secret) map[string][]byte {
	tr, err := r.discoverTenantRegistry(ctx)
	if err != nil {
		logf.FromContext(ctx).Error(err, "discover tenant registry; falling back to parent registry secret (tenant node image pull may fail)")
		return maps.Clone(parent.Data)
	}
	if tr == nil {
		return maps.Clone(parent.Data)
	}
	return tr.registrySecretData()
}

// splitAddressAndPath splits "registry.example.com/deckhouse/ee" into
// ("registry.example.com", "/deckhouse/ee").
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

// rebaseImageRef re-points a "<base>@<digest>" reference at newBase, preserving the digest.
// Used to move VCP-baked tenant-node image refs off the parent in-cluster registry onto the discovered external upstream.
func rebaseImageRef(ref, newBase string) string {
	at := strings.LastIndex(ref, "@")
	if at < 0 || newBase == "" {
		return ref
	}
	return newBase + ref[at:]
}
