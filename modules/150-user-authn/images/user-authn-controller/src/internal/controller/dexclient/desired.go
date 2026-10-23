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

package dexclient

import (
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/internal/adoption"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// SecretPrefix is the prefix of the Secret name and of both client IDs.
	SecretPrefix = "dex-client-"
	// SecretKey is the Secret data key holding the client secret.
	SecretKey = "clientSecret"

	LabelApp      = "app"
	AppValue      = "dex-client"
	LabelName     = "name"
	NameValue     = "credentials"
	LabelHeritage = "heritage"
	HeritageValue = "deckhouse"
	LabelModule   = "module"
	ModuleValue   = "user-authn"

	// AllowAccessToKubernetesAnnotation is copied from the DexClient to its Secret; the kubernetes
	// client reconciler reads it from the DexClient.
	AllowAccessToKubernetesAnnotation = controller.DexClientAllowAccessToKubernetesAnnotation

	// clientSecretLength and clientSecretAlphabet match the secrets the module hook generated.
	clientSecretLength   = 20
	clientSecretAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// Spec is the part of the DexClient spec the controller renders.
type Spec struct {
	RedirectURIs   []string       `json:"redirectURIs,omitempty"`
	TrustedPeers   []string       `json:"trustedPeers,omitempty"`
	AllowedEmails  []string       `json:"allowedEmails,omitempty"`
	AllowedGroups  []string       `json:"allowedGroups,omitempty"`
	SecretMetadata SecretMetadata `json:"secretMetadata,omitempty"`

	PostLogoutRedirectURIs []string `json:"postLogoutRedirectURIs,omitempty"`
	BackchannelLogoutURI   string   `json:"backchannelLogoutURI,omitempty"`
	RefreshTokenLifetime   string   `json:"refreshTokenLifetime,omitempty"`
}

// SecretMetadata is spec.secretMetadata of a DexClient.
type SecretMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Client is a decoded DexClient.
type Client struct {
	Name        string
	Namespace   string
	UID         types.UID
	Annotations map[string]string
	Spec        Spec
}

// Decode reads a DexClient.
func Decode(obj *unstructured.Unstructured) (Client, error) {
	var decoded struct {
		Spec Spec `json:"spec"`
	}
	if err := controller.DecodeInto(obj, &decoded); err != nil {
		return Client{}, fmt.Errorf("decode DexClient %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return Client{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		UID:         obj.GetUID(),
		Annotations: obj.GetAnnotations(),
		Spec:        decoded.Spec,
	}, nil
}

// ClientID is the Dex client ID of a DexClient.
func ClientID(name, namespace string) string {
	return SecretPrefix + name + "@" + namespace
}

// LegacyClientID is the old client ID with a colon, which cannot be sent in basic auth credentials.
// Its OAuth2Client is still kept for the applications configured with it.
func LegacyClientID(name, namespace string) string {
	return SecretPrefix + name + ":" + namespace
}

// SecretName is the name of the credentials Secret in the namespace of the DexClient.
func SecretName(name string) string {
	return SecretPrefix + name
}

// OAuth2ClientName is the object name Dex stores the client with ID id under.
func OAuth2ClientName(id string) string {
	return naming.ToFnvLikeDex(id)
}

// ParseClientID returns the DexClient of a current or legacy client ID. Neither a name nor a
// namespace can contain '@' or ':', so the last one separates them.
func ParseClientID(id string) (types.NamespacedName, bool) {
	rest, found := strings.CutPrefix(id, SecretPrefix)
	if !found {
		return types.NamespacedName{}, false
	}
	i := strings.LastIndexAny(rest, "@:")
	if i <= 0 || i == len(rest)-1 {
		return types.NamespacedName{}, false
	}
	return types.NamespacedName{Namespace: rest[i+1:], Name: rest[:i]}, true
}

// GenerateSecret returns a new random client secret.
func GenerateSecret() (string, error) {
	limit := big.NewInt(int64(len(clientSecretAlphabet)))
	out := make([]byte, clientSecretLength)
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate client secret: %w", err)
		}
		out[i] = clientSecretAlphabet[n.Int64()]
	}
	return string(out), nil
}

// moduleLabels are the labels every object of the controller carries.
func moduleLabels(app string) map[string]string {
	return map[string]string{
		LabelHeritage: HeritageValue,
		LabelModule:   ModuleValue,
		LabelApp:      app,
	}
}

// secretMetadata is the desired metadata of the Secret: the user's from secretMetadata, and the
// module labels, which win, so that the controller always finds its Secret by them. The chart let
// secretMetadata override them, which made the module lose such a Secret.
func secretMetadata(c Client) adoption.Metadata {
	fixed := moduleLabels(AppValue)
	fixed[LabelName] = NameValue

	annotations := maps.Clone(c.Spec.SecretMetadata.Annotations)
	if value, ok := c.Annotations[AllowAccessToKubernetesAnnotation]; ok {
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations[AllowAccessToKubernetesAnnotation] = value
	}
	return adoption.Metadata{
		Fixed:       fixed,
		Labels:      c.Spec.SecretMetadata.Labels,
		Annotations: annotations,
		Track:       true,
	}
}

func oauth2ClientMetadata() adoption.Metadata {
	return adoption.Metadata{Fixed: moduleLabels(AppValue), Track: true}
}

// oauth2ClientFields are the top-level fields of an OAuth2Client the controller owns.
var oauth2ClientFields = []string{
	"id", "name", "secret", "redirectURIs", "trustedPeers", "allowedEmails", "allowedGroups",
	"postLogoutRedirectURIs", "backchannelLogoutURI", "refreshTokenLifetime",
}

// oauth2ClientContent is the desired content of the OAuth2Client with ID id. Empty lists and
// strings are left out, as the chart did. Dex takes refreshTokenLifetime in lower case.
func oauth2ClientContent(c Client, id, secret string) map[string]any {
	out := map[string]any{
		"id":     id,
		"name":   id,
		"secret": secret,
	}
	lists := map[string][]string{
		"redirectURIs":  c.Spec.RedirectURIs,
		"trustedPeers":  c.Spec.TrustedPeers,
		"allowedEmails": c.Spec.AllowedEmails,
		"allowedGroups": c.Spec.AllowedGroups,

		"postLogoutRedirectURIs": c.Spec.PostLogoutRedirectURIs,
	}
	for key, values := range lists {
		if len(values) == 0 {
			continue
		}
		items := make([]any, 0, len(values))
		for _, v := range values {
			items = append(items, v)
		}
		out[key] = items
	}
	if c.Spec.BackchannelLogoutURI != "" {
		out["backchannelLogoutURI"] = c.Spec.BackchannelLogoutURI
	}
	if c.Spec.RefreshTokenLifetime != "" {
		out["refreshTokenLifetime"] = strings.ToLower(c.Spec.RefreshTokenLifetime)
	}
	return out
}
