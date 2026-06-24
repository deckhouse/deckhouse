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

package registryaddress

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	deckhouseregistry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
)

const (
	// constantAddress/constantPath are the new-arch registry address: the agent
	// intercepts registry.d8-system.svc:5001 and the primary entry serves the
	// system/deckhouse repo (LocalPathAlias) from the real DKP upstream.
	constantAddress = "registry.d8-system.svc:5001"
	constantPath    = "/system/deckhouse"
)

// buildLocalDockerCfg returns a docker config JSON authenticating to the local
// registry svc with the given (ReadOnly) user.
func buildLocalDockerCfg(user, pass string) ([]byte, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
	cfg := map[string]any{
		"auths": map[string]any{
			constantAddress: map[string]string{
				"username": user,
				"password": pass,
				"auth":     auth,
			},
		},
	}
	return json.Marshal(cfg)
}

// buildConstantConfig builds the new-arch deckhouse-registry Config: the constant
// local address + module CA + local ReadOnly creds.
func buildConstantConfig(moduleCA, roUser, roPass string) (deckhouseregistry.Config, error) {
	dcfg, err := buildLocalDockerCfg(roUser, roPass)
	if err != nil {
		return deckhouseregistry.Config{}, fmt.Errorf("build local dockercfg: %w", err)
	}
	return deckhouseregistry.Config{
		Address: constantAddress,
		Path:    constantPath,
		// Lowercase "https" matches what the legacy orchestrator writes to this
		// secret and what the containerd fallback (bashible step 030) expects
		// (it does `eq $scheme "https"` and `printf "%s://"`). Uppercase would
		// emit HTTPS:// and skip the CA block on that path.
		Scheme:       "https",
		CA:           moduleCA,
		DockerConfig: dcfg,
	}, nil
}
