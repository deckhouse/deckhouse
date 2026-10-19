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

package configstate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
)

var t0 = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

// clock is fixed at t0; Accept only reads Now.
type clock struct{}

func (clock) Now() time.Time                     { return t0 }
func (clock) NewTimer(time.Duration) retry.Timer { return nil }

// configured returns a configure that yields values with checksum sum.
func configured(sum string) func() (addonutils.Values, string, error) {
	return func() (addonutils.Values, string, error) { return addonutils.Values{"sum": sum}, sum, nil }
}

type configstateSuite struct {
	suite.Suite

	state   configstate.State
	backoff retry.Backoff
}

func TestConfigState(t *testing.T) {
	suite.Run(t, new(configstateSuite))
}

func (s *configstateSuite) SetupTest() {
	s.state = configstate.State{}
	s.backoff = retry.Backoff{}
}

func (s *configstateSuite) TestFirstAcceptIsInForceAndChanged() {
	out := s.state.Accept("k1", &s.backoff, clock{}, configured("c1"))

	s.NoError(out.Err)
	s.True(out.Changed)
	s.True(s.state.InForce("k1"))
	s.Equal("c1", s.state.Accepted.Checksum)
}

func (s *configstateSuite) TestSameValuesAreNotAChange() {
	s.state.Accept("k1", &s.backoff, clock{}, configured("c1"))

	out := s.state.Accept("k2", &s.backoff, clock{}, configured("c1"))

	s.False(out.Changed, "another configuration with the same effective values")
	s.True(s.state.InForce("k2"))
}

func (s *configstateSuite) TestFailureKeepsTheLastAcceptedInForceAndHoldsTheRetry() {
	s.state.Accept("k1", &s.backoff, clock{}, configured("c1"))

	out := s.state.Accept("k2", &s.backoff, clock{}, func() (addonutils.Values, string, error) {
		return nil, "", errors.New("invalid")
	})

	s.EqualError(out.Err, "invalid")
	s.Equal(t0.Add(retry.Min), out.Until)
	s.True(s.backoff.Holds("k2", t0))
	s.Equal("c1", s.state.Accepted.Checksum, "the last accepted stays")
	s.False(s.state.InForce("k1"), "another configuration was tried since")
	s.False(s.state.InForce("k2"))
}

func (s *configstateSuite) TestPushingTheAcceptedAgainIsInForce() {
	s.state.Accept("k1", &s.backoff, clock{}, configured("c1"))
	s.state.Accept("k2", &s.backoff, clock{}, func() (addonutils.Values, string, error) {
		return nil, "", errors.New("invalid")
	})

	out := s.state.Accept("k1", &s.backoff, clock{}, configured("c1"))

	s.NoError(out.Err)
	s.True(s.state.InForce("k1"))
	s.False(s.backoff.Holds("k2", t0), "a success clears the backoff")
}

func (s *configstateSuite) TestCheckpointWritesTheAcceptedValues() {
	var cr model.CurrentReport

	configstate.Accepted{Values: addonutils.Values{"a": 1}, Checksum: "c1"}.Checkpoint(&cr)

	s.Equal(1, cr.AppliedValues["a"])
	s.Equal("c1", cr.AppliedChecksum)
}

func (s *configstateSuite) TestChecksumIgnoresMapOrder() {
	a, err := configstate.Checksum(map[string]int{"x": 1, "y": 2})
	s.Require().NoError(err)

	b, err := configstate.Checksum(map[string]int{"y": 2, "x": 1})
	s.Require().NoError(err)

	s.Equal(a, b)

	_, err = configstate.Checksum(func() {})
	s.Error(err, "an unencodable value has no checksum")
}
