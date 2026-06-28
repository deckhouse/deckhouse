// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package meta contains DVP validation constants.
package meta

import (
	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
)

const (
	// ProviderClusterConfigKubeconfigPath is the dot path to kubeconfig in legacy ProviderClusterConfiguration.
	ProviderClusterConfigKubeconfigPath = "provider.kubeconfigDataBase64"
)

var (
	// AllowedCredentialAuthSchemes lists auth schemes supported by the DVP provider.
	AllowedCredentialAuthSchemes = []cpapi.AuthScheme{cpapi.AuthSchemeKubeconfig}
)
