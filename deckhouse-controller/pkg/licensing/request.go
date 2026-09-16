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

// MetricValue carries the three views of a consumption metric required by the
// registration schema: the current reading, the seven day moving average and
// the linear extrapolation to the end of the term. All three are always
// serialized, zero values included.
type MetricValue struct {
	Instant      float64 `json:"instant"`
	Avg7d        float64 `json:"avg_7d"`
	Extrapolated float64 `json:"extrapolated"`
}

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
	Metrics      map[string]MetricValue
	// Records holds the ids of the records of every accepted package. Rejected
	// records must not be listed: the license server would count them as part
	// of the policy.
	Records []string
	Key     ed25519.PrivateKey
	// IncludeJWK selects the header form. A request carrying jwk declares a new
	// identity and is only sent while no package has been accepted yet;
	// afterwards the key is referenced by its thumbprint in kid. Exactly one of
	// the two is present.
	IncludeJWK bool
}

// registrationPayload is the v1 schema. Field order follows the specification;
// optional fields disappear when empty, metrics and records never do.
type registrationPayload struct {
	Ver          int                    `json:"ver"`
	ClusterID    string                 `json:"cluster_id"`
	PublicDomain string                 `json:"public_domain,omitempty"`
	JTI          string                 `json:"jti"`
	IAT          string                 `json:"iat"`
	Seq          uint64                 `json:"seq"`
	Build        string                 `json:"build,omitempty"`
	DKPVersion   string                 `json:"dkp_version,omitempty"`
	Metrics      map[string]MetricValue `json:"metrics"`
	Records      []string               `json:"records"`
}

// BuildRegistrationRequest returns a signed bare compact JWT with the v1
// registration payload. Record ids are sorted lexicographically so that an
// unchanged policy produces stable bytes.
func BuildRegistrationRequest(in RegistrationInput) (string, error) {
	if len(in.Key) != ed25519.PrivateKeySize {
		return "", errors.New("licensing: invalid Ed25519 private key")
	}
	pub, ok := in.Key.Public().(ed25519.PublicKey)
	if !ok {
		return "", errors.New("licensing: invalid Ed25519 private key")
	}

	records := make([]string, len(in.Records))
	copy(records, in.Records)
	sort.Strings(records)

	metrics := in.Metrics
	if metrics == nil {
		metrics = map[string]MetricValue{}
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
		Records:      records,
	}, in.Key)
}
