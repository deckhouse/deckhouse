/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"crypto/sha256"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

// generateSafeName creates a safe name within 63 character limit with optional hash suffix
func generateSafeName(baseName, fixedPart string, isPrefix bool) string {
	var fullName string
	if isPrefix {
		fullName = fmt.Sprintf("%s-%s", fixedPart, baseName)
	} else {
		fullName = fmt.Sprintf("%s-%s", baseName, fixedPart)
	}

	if len(fullName) <= 63 {
		return fullName
	}

	// Generate hash from base name
	hasher := sha256.New()
	hasher.Write([]byte(baseName))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))[:8]

	// Calculate maximum length for the truncated name
	maxNameLength := 63 - len(fixedPart) - 1 - len(hash) - 1 // 63 - fixedPart - dash - hash - dash
	if maxNameLength < 1 {
		maxNameLength = 1
	}

	// Truncate base name if necessary
	truncatedName := baseName
	if len(baseName) > maxNameLength {
		truncatedName = baseName[:maxNameLength]
	}

	if isPrefix {
		return fmt.Sprintf("%s-%s-%s", fixedPart, truncatedName, hash)
	}
	return fmt.Sprintf("%s-%s-%s", truncatedName, hash, fixedPart)
}

// dexAuthenticatorNameWithNamespace creates safe names for DexAuthenticator objects
// following the same logic as dex_authenticator_name_with_namespace in _helpers.tpl
func dexAuthenticatorNameWithNamespace(name, namespace string) string {
	combinedName := fmt.Sprintf("%s-%s", name, namespace)
	return generateSafeName(combinedName, "dex-authenticator", false)
}

// dexAuthenticatorNameReverse creates safe names with reverse pattern (prefix-name)
// following the same logic as dex_authenticator_name_reverse in _helpers.tpl
func dexAuthenticatorNameReverse(name string) string {
	return generateSafeName(name, "dex-authenticator", true)
}

type DexAuthenticator struct {
	ID          string                 `json:"uuid"`
	EncodedName string                 `json:"encodedName"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Spec        map[string]interface{} `json:"spec"`

	AllowAccessToKubernetes bool        `json:"allowAccessToKubernetes"`
	Credentials             Credentials `json:"credentials"`
}

type Credentials struct {
	CookieSecret string `json:"cookieSecret"`
	AppDexSecret string `json:"appDexSecret"`
}

type DexAuthenticatorSecret struct {
	ID          string      `json:"uuid"`
	Name        string      `json:"name"`
	Namespace   string      `json:"namespace"`
	Credentials Credentials `json:"credentials"`
}

func applyDexAuthenticatorFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from dex authenticator: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("dex authenticator has no spec field")
	}

	name := obj.GetName()
	namespace := obj.GetNamespace()

	id := fmt.Sprintf("%s@%s", name, namespace)
	fullName := dexAuthenticatorNameWithNamespace(name, namespace)
	encodedName := encoding.ToSha256Like(fullName)

	_, allowAccessToKubernetes := obj.GetAnnotations()["dexauthenticator.deckhouse.io/allow-access-to-kubernetes"]

	return DexAuthenticator{
		ID:                      id,
		EncodedName:             encodedName,
		Name:                    name,
		Namespace:               namespace,
		Spec:                    spec,
		AllowAccessToKubernetes: allowAccessToKubernetes,
	}, nil
}

func applyDexAuthenticatorSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert dex authenticator secret to secret: %v", err)
	}

	name := obj.GetName()
	namespace := obj.GetNamespace()

	id := fmt.Sprintf("%s@%s", name, namespace)
	return DexAuthenticatorSecret{
		ID:        id,
		Name:      name,
		Namespace: namespace,
		Credentials: Credentials{
			AppDexSecret: string(secret.Data["client-secret"]),
			CookieSecret: string(secret.Data["cookie-secret"]),
		},
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authn",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "authenticators",
			ApiVersion: "deckhouse.io/v2alpha1",
			Kind:       "DexAuthenticator",
			FilterFunc: applyDexAuthenticatorFilter,
		},
		{
			Name:       "credentials",
			ApiVersion: "v1",
			Kind:       "Secret",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":  "dex-authenticator",
					"name": "credentials",
				},
			},
			FilterFunc: applyDexAuthenticatorSecretFilter,
		},
	},
}, getDexAuthenticator)

func getDexAuthenticator(input *go_hook.HookInput) error {
	authenticators := input.Snapshots["authenticators"]
	credentials := input.Snapshots["credentials"]

	credentialsByID := make(map[string]Credentials, len(credentials))

	for _, secret := range credentials {
		dexSecret, ok := secret.(DexAuthenticatorSecret)
		if !ok {
			return fmt.Errorf("cannot convert dex authenticator secret")
		}

		credentialsByID[dexSecret.ID] = dexSecret.Credentials
	}

	dexAuthenticators := make([]DexAuthenticator, 0, len(authenticators))
	for _, authenticator := range authenticators {
		dexAuthenticator, ok := authenticator.(DexAuthenticator)
		if !ok {
			return fmt.Errorf("cannot convert dex authenticaor")
		}

		// Use the same naming logic as in the template for secret name
		secretName := dexAuthenticatorNameReverse(dexAuthenticator.Name)
		secretID := fmt.Sprintf("%s@%s", secretName, dexAuthenticator.Namespace)
		existedCredentials, ok := credentialsByID[secretID]
		if !ok {
			existedCredentials = Credentials{
				AppDexSecret: pwgen.AlphaNum(20),
				CookieSecret: pwgen.AlphaNum(24),
			}
		}

		// Migrate all cookie secret from 20 bytes length to 24 bytes
		if len(existedCredentials.CookieSecret) < 24 {
			existedCredentials.CookieSecret = pwgen.AlphaNum(24)
		}

		dexAuthenticator.Credentials = existedCredentials
		dexAuthenticators = append(dexAuthenticators, dexAuthenticator)
	}

	input.Values.Set("userAuthn.internal.dexAuthenticatorCRDs", dexAuthenticators)
	return nil
}
