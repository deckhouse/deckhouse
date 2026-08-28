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

package app_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
)

type EmbeddedPackageVersionSuite struct {
	suite.Suite
}

func TestEmbeddedPackageVersion(t *testing.T) {
	suite.Run(t, new(EmbeddedPackageVersionSuite))
}

func (s *EmbeddedPackageVersionSuite) TestReducesToMajorMinorPatch() {
	cases := map[string]string{
		"v1.78.0":                 "v1.78.0",
		"1.78.0":                  "v1.78.0",
		"v1.78.0-pr22189+8776a42": "v1.78.0",
		"v1.0.0-RC1":              "v1.0.0",
		"v1.78.0+8776a42":         "v1.78.0",
	}

	for version, want := range cases {
		s.Equal(want, app.EmbeddedPackageVersion(version), version)
	}
}

func (s *EmbeddedPackageVersionSuite) TestDevCountsAsV2() {
	s.Equal("v2.0.0", app.EmbeddedPackageVersion("dev"))
}

func (s *EmbeddedPackageVersionSuite) TestPassesNonSemverThrough() {
	// every caller must still name the same version, even one no object can be named after
	for _, version := range []string{"latest", "Latest_Build", ""} {
		s.Equal(version, app.EmbeddedPackageVersion(version), version)
	}
}

func (s *EmbeddedPackageVersionSuite) TestIsStableAcrossBuildsOfOneRelease() {
	s.Equal(
		app.EmbeddedPackageVersion("v1.78.0-pr22189+8776a42"),
		app.EmbeddedPackageVersion("v1.78.0-pr22190+1234567"),
	)
}
