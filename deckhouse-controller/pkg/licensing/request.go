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

package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"sort"
	"time"
)

// RegistrationInput is everything the cluster knows about itself when it builds
// a registration request.
type RegistrationInput struct {
	ClusterID    string
	PublicDomain string
	Build        string
	DKPVersion   string
	Seq          uint64
	JTI          string
	IssuedAt     time.Time
	// Metrics are the instant readings over the licensable nodes. Plain
	// integers: there is no moving average and no projection any more.
	Metrics map[string]int64
	// Records holds the ids of the accepted records of every installed key.
	// Rejected records must not be listed: the license server would count them
	// as part of the policy.
	Records []string
	// ActiveKeys holds the jti of the installed keys that carry at least one
	// accepted record. Normally exactly one; empty means no key.
	ActiveKeys []string
	Key        ed25519.PrivateKey
	// IncludeJWK selects the header form. A request carrying jwk declares a new
	// identity and is only sent while no key has been accepted yet; afterwards
	// the key is referenced by its thumbprint in kid. Exactly one of the two is
	// present.
	IncludeJWK bool
}

// registrationPayload is the v1 schema. Field order follows the specification;
// optional fields disappear when empty, metrics, active_keys and records never
// do.
type registrationPayload struct {
	Ver          int              `json:"ver"`
	ClusterID    string           `json:"cluster_id"`
	PublicDomain string           `json:"public_domain,omitempty"`
	JTI          string           `json:"jti"`
	IAT          string           `json:"iat"`
	Seq          uint64           `json:"seq"`
	Build        string           `json:"build,omitempty"`
	DKPVersion   string           `json:"dkp_version,omitempty"`
	Metrics      map[string]int64 `json:"metrics"`
	ActiveKeys   []string         `json:"active_keys"`
	Records      []string         `json:"records"`
}

// BuildRegistrationRequest returns a signed bare compact JWT with the v1
// registration payload. Ids are sorted lexicographically so that an unchanged
// policy produces stable bytes.
func BuildRegistrationRequest(in RegistrationInput) (string, error) {
	if len(in.Key) != ed25519.PrivateKeySize {
		return "", errors.New("licensing: invalid Ed25519 private key")
	}
	pub, ok := in.Key.Public().(ed25519.PublicKey)
	if !ok {
		return "", errors.New("licensing: invalid Ed25519 private key")
	}

	header := map[string]any{"typ": TypRegistration}
	if in.IncludeJWK {
		header["jwk"] = map[string]string{
			"crv": "Ed25519",
			"kty": "OKP",
			"x":   base64.RawURLEncoding.EncodeToString(pub),
		}
	} else {
		header["kid"] = Thumbprint(pub)
	}

	metrics := in.Metrics
	if metrics == nil {
		// The schema promises the three metrics are always there, zeros
		// included: the license server must not have to special case a cluster
		// with nothing to license.
		metrics = Consumption(nil)
	}

	return Sign(header, registrationPayload{
		Ver:          SchemaVersion,
		ClusterID:    in.ClusterID,
		PublicDomain: in.PublicDomain,
		JTI:          in.JTI,
		IAT:          in.IssuedAt.UTC().Format(time.RFC3339),
		Seq:          in.Seq,
		Build:        in.Build,
		DKPVersion:   in.DKPVersion,
		Metrics:      metrics,
		ActiveKeys:   sorted(in.ActiveKeys),
		Records:      sorted(in.Records),
	}, in.Key)
}

// sorted returns a sorted copy that marshals as [] rather than null: the
// registration schema requires both arrays to be present.
func sorted(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
