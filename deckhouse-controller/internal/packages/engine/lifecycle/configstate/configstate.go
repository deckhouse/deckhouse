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

// Package configstate holds the configuration state Current and Global share: what was pushed, what is
// in force, and the checksum every configuration is identified by. What each one configures, what
// its checksum covers and what it reports stay with it.
package configstate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

// Checksum is the SHA-256 of v's JSON (map keys sorted): how a pushed configuration, settings and
// effective values are identified.
func Checksum(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("checksum: %w", err)
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:]), nil
}

// Key is Checksum for a pushed configuration or settings, which decode from JSON and so always
// encode; an unencodable value gets the key of its error, which matches no encodable one.
func Key(v any) string {
	sum, err := Checksum(v)
	if err != nil {
		sum, _ = Checksum(fmt.Sprintf("unencodable: %v", err))
	}

	return sum
}

// Accepted is the configuration in force in the object: its effective values and their checksum.
type Accepted struct {
	Values   addonutils.Values
	Checksum string
}

// Checkpoint writes a as the settings checkpoint (AppliedValues), what the next boot restores.
func (a Accepted) Checkpoint(cr *model.CurrentReport) {
	cr.AppliedValues = a.Values
	cr.AppliedChecksum = a.Checksum
}

// State is what was given to Accept and what is in force. Configurations are identified by their
// checksum: most pushes repeat the content in a new value, and they must neither reconfigure the
// object nor cancel a run. Only Accept writes it; one loop owns it.
type State struct {
	Pushed           string    // Pushed is the checksum last given to Accept, accepted or not.
	AcceptedChecksum string    // AcceptedChecksum is the one whose accept succeeded last.
	Accepted         *Accepted // Accepted is in force in the object; nil until the first accept.
}

// InForce reports whether the configuration of checksum is the one the object holds and Configured
// reports: the last accept was of it and it succeeded. Not so after another configuration was tried
// since, even if that one failed, so pushing the old one again reports Configured=true.
func (s State) InForce(checksum string) bool {
	return s.Pushed == checksum && s.AcceptedChecksum == checksum
}

// Outcome is the result of Accept.
type Outcome struct {
	// Err: not accepted; the last accepted configuration stays in force and the retry is held
	// until Until.
	Err   error
	Until time.Time
	// Changed: the effective values in force changed (the first accept included).
	Changed bool
}

// Accept records checksum as pushed and runs configure, which takes the configuration into the
// object and returns the effective values with their checksum. A failure keeps the last accepted
// configuration in force and holds its retry in backoff, keyed by checksum and counted from the
// failure (validation may take long).
func (s *State) Accept(checksum string, backoff *retry.Backoff, clock retry.Clock, configure func() (addonutils.Values, string, error)) Outcome {
	s.Pushed = checksum

	values, sum, err := configure()
	if err != nil {
		return Outcome{Err: err, Until: backoff.Failed(checksum, clock.Now())}
	}

	backoff.Succeeded()
	s.AcceptedChecksum = checksum

	changed := s.Accepted == nil || s.Accepted.Checksum != sum
	if changed {
		s.Accepted = &Accepted{Values: values, Checksum: sum}
	}

	return Outcome{Changed: changed}
}
