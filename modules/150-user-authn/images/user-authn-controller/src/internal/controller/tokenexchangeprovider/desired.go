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

package tokenexchangeprovider

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/dexclient"
	"user-authn-controller/internal/naming"
)

const (
	// connectorType is the Dex connector type of patch 420.
	connectorType = "tokenexchange"
	// grantTypeTokenExchange is the only grant the connector serves.
	grantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"

	// ProviderLabel names the TokenExchangeProvider of a connector.
	ProviderLabel = "user-authn.deckhouse.io/token-exchange-provider"
	// Finalizer holds a TokenExchangeProvider until the tokens of its connector are revoked.
	Finalizer = "user-authn.deckhouse.io/token-exchange-provider"
)

// ReservedPrefixes are the name prefixes the Dex config reserves (reservedNamePrefixes in
// templates/dex/config.yaml) apart from "system:", which no provider may use at all. A provider
// owns the reserved prefixes its own prefixes start with; the ValidatingAdmissionPolicy of the
// module lets only their owners set them.
var ReservedPrefixes = []string{"peering:"}

// Spec is the spec of a TokenExchangeProvider.
type Spec struct {
	Issuer          string      `json:"issuer"`
	Keys            Keys        `json:"keys"`
	Audiences       []Audience  `json:"audiences"`
	ClaimValidation []ClaimRule `json:"claimValidation,omitempty"`
	// SubjectTokenMaxLifetime is a duration; the schema defaults it to 24h.
	SubjectTokenMaxLifetime string            `json:"subjectTokenMaxLifetime,omitempty"`
	Identity                Identity          `json:"identity"`
	AllowedIdentities       AllowedIdentities `json:"allowedIdentities,omitempty"`
	Clients                 []ClientRef       `json:"clients"`
}

// Keys holds one source of the issuer's keys.
type Keys struct {
	Discovery *Discovery `json:"discovery,omitempty"`
	JWKS      *JWKS      `json:"jwks,omitempty"`
}

// Discovery locates the discovery document of the issuer.
type Discovery struct {
	URL    string `json:"url,omitempty"`
	CAData string `json:"caData,omitempty"`
}

// JWKS is a JWK Set of public keys, passed to Dex as it is.
type JWKS struct {
	Keys []map[string]any `json:"keys"`
}

// Audience matches aud exactly or by prefix.
type Audience struct {
	Exact  string `json:"exact,omitempty"`
	Prefix string `json:"prefix,omitempty"`
}

// ClaimRule requires a claim value.
type ClaimRule struct {
	Claim  string `json:"claim"`
	Equals string `json:"equals"`
}

// Identity names the subject.
type Identity struct {
	Username ClaimMapping   `json:"username"`
	Email    *ClaimMapping  `json:"email,omitempty"`
	Groups   []GroupMapping `json:"groups,omitempty"`
}

// ClaimMapping is a string claim with a prefix.
type ClaimMapping struct {
	Claim  string `json:"claim"`
	Prefix string `json:"prefix,omitempty"`
}

// GroupMapping takes groups from a claim.
type GroupMapping struct {
	Claim      string `json:"claim"`
	TrimPrefix string `json:"trimPrefix,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
}

// AllowedIdentities are the allow-lists of the connector (patch 200).
type AllowedIdentities struct {
	Emails       []string `json:"emails,omitempty"`
	EmailDomains []string `json:"emailDomains,omitempty"`
	Groups       []string `json:"groups,omitempty"`
}

// ClientRef names a client that may exchange.
type ClientRef struct {
	DexClient *DexClientRef `json:"dexClient,omitempty"`
}

// DexClientRef is a DexClient.
type DexClientRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// connectorConfig is the config of the Dex connector: the spec with the client IDs resolved and
// the reserved prefixes the provider owns.
type connectorConfig struct {
	Issuer                  string      `json:"issuer"`
	Keys                    Keys        `json:"keys"`
	Audiences               []Audience  `json:"audiences"`
	ClaimValidation         []ClaimRule `json:"claimValidation,omitempty"`
	SubjectTokenMaxLifetime string      `json:"subjectTokenMaxLifetime,omitempty"`
	Identity                Identity    `json:"identity"`
	ReservedPrefixes        []string    `json:"reservedPrefixes,omitempty"`
	Clients                 []string    `json:"clients"`
}

// Decode reads the spec of a TokenExchangeProvider.
func Decode(obj *unstructured.Unstructured) (Spec, error) {
	raw, ok := obj.Object["spec"].(map[string]any)
	if !ok {
		return Spec{}, errors.New("spec is missing")
	}
	var spec Spec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &spec); err != nil {
		return Spec{}, fmt.Errorf("decode spec: %w", err)
	}
	return spec, nil
}

// ConnectorID is the ID of the first Dex connector of the provider name with the UID uid.
func ConnectorID(name, uid string) string {
	return naming.TokenExchangeConnectorID(name, uid, 0)
}

// validate checks what the CRD schema cannot: Dex refuses to open a connector with them, and a
// connector that does not open refuses every exchange, without falling back to its previous
// config.
func validate(spec Spec) error {
	if spec.Keys.JWKS != nil {
		for i, key := range spec.Keys.JWKS.Keys {
			// d holds the private exponent of RSA, EC and OKP keys; k is a symmetric key.
			if _, private := key["d"]; private {
				return fmt.Errorf("spec.keys.jwks.keys[%d] is a private key: give the public key only", i)
			}
			if _, symmetric := key["k"]; symmetric {
				return fmt.Errorf("spec.keys.jwks.keys[%d] is a symmetric key: give a public key", i)
			}
			if err := publicKey(key); err != nil {
				return fmt.Errorf("spec.keys.jwks.keys[%d]: %w", i, err)
			}
		}
	}
	if d := spec.Keys.Discovery; d != nil && d.CAData != "" {
		if !x509.NewCertPool().AppendCertsFromPEM([]byte(d.CAData)) {
			return errors.New("spec.keys.discovery.caData holds no PEM certificate")
		}
	}
	for _, ref := range spec.Clients {
		if ref.DexClient == nil {
			return errors.New("spec.clients: every client must name a dexClient")
		}
	}
	if spec.SubjectTokenMaxLifetime != "" {
		d, err := time.ParseDuration(spec.SubjectTokenMaxLifetime)
		if err != nil || d <= 0 {
			return fmt.Errorf("spec.subjectTokenMaxLifetime %q is not a positive duration", spec.SubjectTokenMaxLifetime)
		}
	}
	return nil
}

// ecCurves are the curves of the EC keys Dex verifies, by the name in crv.
var ecCurves = map[string]ecdh.Curve{"P-256": ecdh.P256(), "P-384": ecdh.P384(), "P-521": ecdh.P521()}

// ecSizes are the byte lengths of the coordinates of each curve (RFC 7518 section 6.2.1.2).
var ecSizes = map[string]int{"P-256": 32, "P-384": 48, "P-521": 66}

// publicKey checks that a JWK is a public key Dex can build (RFC 7518 section 6, RFC 8037): the
// members it needs, in unpadded base64url, and for EC a point on the curve. go-jose in Dex refuses
// the same keys when the connector opens.
func publicKey(key map[string]any) error {
	kty, _ := key["kty"].(string)
	member := func(name string) ([]byte, error) {
		value, _ := key[name].(string)
		if value == "" {
			return nil, fmt.Errorf("a %s key needs %q", kty, name)
		}
		b, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("%q is not unpadded base64url: %w", name, err)
		}
		return b, nil
	}
	switch kty {
	case "RSA":
		n, err := member("n")
		if err != nil {
			return err
		}
		e, err := member("e")
		if err != nil {
			return err
		}
		if len(e) > 4 || new(big.Int).SetBytes(e).Cmp(big.NewInt(3)) < 0 || new(big.Int).SetBytes(n).Sign() == 0 {
			return errors.New("not a valid RSA public key")
		}
	case "EC":
		crv, _ := key["crv"].(string)
		curve, known := ecCurves[crv]
		if !known {
			return fmt.Errorf("crv %q is not P-256, P-384 or P-521", crv)
		}
		x, err := member("x")
		if err != nil {
			return err
		}
		y, err := member("y")
		if err != nil {
			return err
		}
		if len(x) != ecSizes[crv] || len(y) != ecSizes[crv] {
			return fmt.Errorf("the coordinates of a %s key are %d bytes long", crv, ecSizes[crv])
		}
		if _, err := curve.NewPublicKey(append(append([]byte{4}, x...), y...)); err != nil {
			return fmt.Errorf("not a point of %s", crv)
		}
	case "OKP":
		if crv, _ := key["crv"].(string); crv != "Ed25519" {
			return fmt.Errorf("crv %q is not Ed25519", crv)
		}
		x, err := member("x")
		if err != nil {
			return err
		}
		if len(x) != ed25519.PublicKeySize {
			return fmt.Errorf("an Ed25519 key is %d bytes long", ed25519.PublicKeySize)
		}
	default:
		return fmt.Errorf("kty %q is not RSA, EC or OKP", kty)
	}
	for _, name := range []string{"kid", "use", "alg"} {
		if value, set := key[name]; set {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("%q is not a string", name)
			}
		}
	}
	// Then the parser of Dex itself, for what the checks above leave out: x5c, x5t and x5u that do
	// not parse or do not match the key.
	raw, err := json.Marshal(key)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	var parsed jose.JSONWebKey
	if err := parsed.UnmarshalJSON(raw); err != nil {
		return fmt.Errorf("Dex cannot read the key: %w", err)
	}
	if !parsed.Valid() || !parsed.IsPublic() {
		return errors.New("Dex cannot read the key as a public key")
	}
	return nil
}

// ownedPrefixes returns the prefixes of the identity mapping that lie under a reserved prefix. Dex
// lets the connector give names under a reserved prefix only when they start with one of them: a
// peer owns "peering:peer:moscow:", not all of "peering:", so it cannot name the subjects of
// another peer, whatever the issuer puts into its claims.
func ownedPrefixes(identity Identity) []string {
	prefixes := []string{identity.Username.Prefix}
	if identity.Email != nil {
		prefixes = append(prefixes, identity.Email.Prefix)
	}
	for _, g := range identity.Groups {
		prefixes = append(prefixes, g.Prefix)
	}
	var owned []string
	for _, p := range prefixes {
		if slices.ContainsFunc(ReservedPrefixes, func(reserved string) bool { return strings.HasPrefix(p, reserved) }) &&
			!slices.Contains(owned, p) {
			owned = append(owned, p)
		}
	}
	return owned
}

// clientIDs resolves the clients of the provider to Dex client IDs.
func clientIDs(spec Spec) []string {
	ids := make([]string, 0, len(spec.Clients))
	for _, ref := range spec.Clients {
		ids = append(ids, dexclient.ClientID(ref.DexClient.Name, ref.DexClient.Namespace))
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// desiredConnector is the Dex Connector object of the provider. Its name is the connector ID: the
// Dex kubernetes storage finds a connector by its object name.
func desiredConnector(name, id string, spec Spec) (*unstructured.Unstructured, error) {
	config, err := json.Marshal(connectorConfig{
		Issuer:                  spec.Issuer,
		Keys:                    spec.Keys,
		Audiences:               spec.Audiences,
		ClaimValidation:         spec.ClaimValidation,
		SubjectTokenMaxLifetime: spec.SubjectTokenMaxLifetime,
		Identity:                spec.Identity,
		ReservedPrefixes:        ownedPrefixes(spec.Identity),
		Clients:                 clientIDs(spec),
	})
	if err != nil {
		return nil, fmt.Errorf("encode connector config: %w", err)
	}

	obj := controller.Object(controller.ConnectorGVK)
	obj.SetName(id)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetLabels(map[string]string{
		ProviderLabel: name,
		"heritage":    "deckhouse",
		"module":      "user-authn",
	})
	obj.Object["id"] = id
	obj.Object["type"] = connectorType
	obj.Object["name"] = name
	// The storage keeps the config as bytes, which JSON carries in base64.
	obj.Object["config"] = base64.StdEncoding.EncodeToString(config)
	obj.Object["grantTypes"] = []any{grantTypeTokenExchange}
	setList(obj, "allowedEmails", spec.AllowedIdentities.Emails)
	setList(obj, "allowedEmailDomains", spec.AllowedIdentities.EmailDomains)
	setList(obj, "allowedGroups", spec.AllowedIdentities.Groups)
	return obj, nil
}

// connectorFields are the fields of a Connector the controller owns.
var connectorFields = []string{"id", "type", "name", "config", "grantTypes", "allowedEmails", "allowedEmailDomains", "allowedGroups"}

func setList(obj *unstructured.Unstructured, field string, values []string) {
	if len(values) == 0 {
		return
	}
	items := make([]any, 0, len(values))
	for _, v := range values {
		items = append(items, v)
	}
	obj.Object[field] = items
}
