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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/go_lib/pwgen"
)

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
	encodedName := encoding.ToFnvLikeDex(fmt.Sprintf("%s-%s-dex-authenticator", name, namespace))

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

func getDexAuthenticator(_ context.Context, input *go_hook.HookInput) error {
	authenticators := input.Snapshots.Get("authenticators")
	credentials := input.Snapshots.Get("credentials")

	credentialsByID := make(map[string]Credentials, len(credentials))

	for dexSecret, err := range sdkobjectpatch.SnapshotIter[DexAuthenticatorSecret](credentials) {
		if err != nil {
			return fmt.Errorf("cannot convert dex authenticator secret: failed to iterate over 'credentials' snapshot: %w", err)
		}

		credentialsByID[dexSecret.ID] = dexSecret.Credentials
	}

	dexAuthenticators := make([]DexAuthenticator, 0, len(authenticators))
	// Build computed names map: key "<name>@<namespace>" => {name, truncated, hash}
	namesMap := make(map[string]interface{})

	for dexAuthenticator, err := range sdkobjectpatch.SnapshotIter[DexAuthenticator](authenticators) {
		if err != nil {
			return fmt.Errorf("cannot convert dex authenticaor: failed to iterate over 'authenticators' snapshot: %w", err)
		}

		existedCredentials, ok := credentialsByID[fmt.Sprintf("dex-authenticator-%s", dexAuthenticator.ID)]
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

		// Compute safe base resource name
		full := fmt.Sprintf("%s-dex-authenticator", dexAuthenticator.Name)
		safeName, truncated, hash5 := SafeDNS1123Name(full)

		// Compute safe secret name
		fullSecretName := fmt.Sprintf("dex-authenticator-%s", dexAuthenticator.Name)
		safeSecretName, secretTruncated, secretHash := SafeDNS1123Name(fullSecretName)

		ingressNames := make(map[string]map[string]interface{})
		signOutIngressNames := make(map[string]map[string]interface{})

		if applications, found, _ := unstructured.NestedSlice(dexAuthenticator.Spec, "applications"); found {
			for i, app := range applications {
				appMap, ok := app.(map[string]interface{})
				if !ok {
					continue
				}
				domain, ok := appMap["domain"].(string)
				if !ok {
					continue
				}

				var nameSuffix string
				if i > 0 {
					h := sha256.Sum256([]byte(domain))
					hashedDomain := hex.EncodeToString(h[:])[:8]
					nameSuffix = fmt.Sprintf("-%s", hashedDomain)
				}

				// Main ingress
				fullIngressName := fmt.Sprintf("%s%s-dex-authenticator", dexAuthenticator.Name, nameSuffix)
				safeIngressName, ingTruncated, ingHash := SafeDNS1123Name(fullIngressName)
				ingressNames[fmt.Sprintf("%d", i)] = map[string]interface{}{
					"name":      safeIngressName,
					"truncated": ingTruncated,
					"hash":      ingHash,
				}

				// SignOut ingress
				if signOutURL, found, _ := unstructured.NestedString(appMap, "signOutURL"); found && signOutURL != "" {
					fullSignOutIngressName := fmt.Sprintf("%s-sign-out", fullIngressName)
					safeSignOutIngressName, signOutTruncated, signOutHash := SafeDNS1123Name(fullSignOutIngressName)
					signOutIngressNames[fmt.Sprintf("%d", i)] = map[string]interface{}{
						"name":      safeSignOutIngressName,
						"truncated": signOutTruncated,
						"hash":      signOutHash,
					}
				}
			}
		}

		namesMap[dexAuthenticator.ID] = map[string]interface{}{
			"name":                safeName,
			"truncated":           truncated,
			"hash":                hash5,
			"secretName":          safeSecretName,
			"secretTruncated":     secretTruncated,
			"secretHash":          secretHash,
			"ingressNames":        ingressNames,
			"signOutIngressNames": signOutIngressNames,
		}
	}

	input.Values.Set("userAuthn.internal.dexAuthenticatorCRDs", dexAuthenticators)
	input.Values.Set("userAuthn.internal.dexAuthenticatorNames", namesMap)
	return nil
}

var (
	notAllowedRegexp = regexp.MustCompile("[^a-z0-9-]+")
	multiDashRegexp  = regexp.MustCompile("-+")
)

// SafeDNS1123Name normalizes and truncates name to DNS-1123 and length <=63.
// If truncation happens, "-<hash5>" is appended, where hash5 is first 5 hex of sha256(original).
func SafeDNS1123Name(fullOriginalName string) (string, bool, string) {
	// If base name length is within limit, keep it exactly as is.
	if len(fullOriginalName) <= 63 {
		return fullOriginalName, false, ""
	}

	// Normalize only when we have to truncate.
	normalized := strings.ToLower(fullOriginalName)
	normalized = notAllowedRegexp.ReplaceAllString(normalized, "-")
	normalized = multiDashRegexp.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")

	// Compute hash only if needed later.
	if len(normalized) <= 63 {
		return normalized, false, ""
	}

	fullHash := stringsutil.Sha256Encode(fullOriginalName)
	hash5 := fullHash[:5]

	base := normalized[:57]
	base = strings.TrimRight(base, "-")
	safe := base + "-" + hash5
	return safe, true, hash5
}
