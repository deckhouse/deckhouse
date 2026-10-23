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

package controller

import (
	"fmt"
	"strconv"
)

const (
	// DexClientAllowAccessToKubernetesAnnotation and DexAuthenticatorAllowAccessToKubernetesAnnotation
	// put the client into trustedPeers of the kubernetes OAuth2Client, so its tokens are accepted
	// by kube-apiserver.
	DexClientAllowAccessToKubernetesAnnotation        = "dexclient.deckhouse.io/allow-access-to-kubernetes"
	DexAuthenticatorAllowAccessToKubernetesAnnotation = "dexauthenticator.deckhouse.io/allow-access-to-kubernetes"
)

// AllowsAccessToKubernetes reads the annotation key of annotations. A value that is not a boolean
// denies access and is returned as an error: it must not hand out tokens the kube-apiserver accepts.
func AllowsAccessToKubernetes(annotations map[string]string, key string) (bool, error) {
	raw, ok := annotations[key]
	if !ok {
		return false, nil
	}
	allowed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("annotation %s=%q is not a boolean, access to Kubernetes is denied", key, raw)
	}
	return allowed, nil
}
