// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	registry_types "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/types"
)

func (s *PreflightChecksTestSuite) TestEditionBad() {
	t := s.Require()

	app.AppVersion = "dev"
	app.AppEdition = "test"
	app.PreflightSkipDeckhouseEditionCheck = false
	image := s.checker.installConfig.GetRemoteImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.metaConfig = &config.MetaConfig{
		Registry: registry_config.Config{
			Mode: &registry_config.UnmanagedMode{
				Remote: registry_types.Data{
					Scheme:     "https",
					ImagesRepo: "test.registry.io/test",
					CA:         "",
				},
			},
		},
	}

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.ConfigFile{
			Config: v1.Config{Labels: map[string]string{
				"io.deckhouse.edition": "BAD",
			}}}, nil)

	err = s.checker.CheckDhctlEdition(context.Background())
	t.Error(err)
}

func (s *PreflightChecksTestSuite) TestOk() {
	t := s.Require()

	app.AppVersion = "dev"
	app.AppEdition = "test"
	app.PreflightSkipDeckhouseEditionCheck = false
	image := s.checker.installConfig.GetRemoteImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.metaConfig = &config.MetaConfig{
		Registry: registry_config.Config{
			Mode: &registry_config.UnmanagedMode{
				Remote: registry_types.Data{
					Scheme:     "https",
					ImagesRepo: "test.registry.io/test",
					CA:         "",
				},
			},
		},
	}

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.ConfigFile{
			Config: v1.Config{Labels: map[string]string{
				"io.deckhouse.edition": "test",
			}}}, nil)

	err = s.checker.CheckDhctlEdition(context.Background())
	t.NoError(err)
}

func (s *PreflightChecksTestSuite) TestCheckDisable() {
	t := s.Require()

	app.AppVersion = "dev"
	app.AppEdition = "test"
	app.PreflightSkipDeckhouseEditionCheck = true
	image := s.checker.installConfig.GetRemoteImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.metaConfig = &config.MetaConfig{
		Registry: registry_config.Config{
			Mode: &registry_config.UnmanagedMode{
				Remote: registry_types.Data{
					Scheme:     "https",
					ImagesRepo: "test.registry.io/test",
					CA:         "",
				},
			},
		},
	}

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.ConfigFile{
			Config: v1.Config{Labels: map[string]string{
				"io.deckhouse.edition": "BAD",
			}}}, nil)

	err = s.checker.CheckDhctlEdition(context.Background())
	t.NoError(err)
}
