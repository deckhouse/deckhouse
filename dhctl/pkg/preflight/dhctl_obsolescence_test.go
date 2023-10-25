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
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

func (s *PreflightChecksTestSuite) Test_PreflightCheck_CheckDhctlVersionObsolescence_Success_ReleaseChannel() {
	t := s.Require()

	image := s.checker.installConfig.GetImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.Descriptor{
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
			}}, nil)

	s.checker.buildDigestProvider = NewFakeBuildDigestProvider(s.T()).
		Return(
			v1.Hash{
				Algorithm: "sha256",
				Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
			}, nil)

	err = s.checker.CheckDhctlVersionObsolescence()
	t.NoError(err)
}

func (s *PreflightChecksTestSuite) Test_PreflightCheck_CheckDhctlVersionObsolescence_Success_DevBranch() {
	t := s.Require()

	s.checker.installConfig.DevBranch = "pr1234"

	image := s.checker.installConfig.GetImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.Descriptor{
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
			}}, nil)

	s.checker.buildDigestProvider = NewFakeBuildDigestProvider(s.T()).
		Return(
			v1.Hash{
				Algorithm: "sha256",
				Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
			}, nil)

	err = s.checker.CheckDhctlVersionObsolescence()
	t.NoError(err)
}

func (s *PreflightChecksTestSuite) Test_PreflightCheck_CheckDhctlVersionObsolescence_VersionMismatch_ReleaseChannel() {
	t := s.Require()

	image := s.checker.installConfig.GetImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.Descriptor{
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
			}}, nil)

	s.checker.buildDigestProvider = NewFakeBuildDigestProvider(s.T()).
		Return(
			v1.Hash{
				Algorithm: "sha256",
				Hex:       "a66bcd004c1c83c1cfb118f7652a30c784cad66ce976249aa64d60219ec5b199",
			}, nil)

	err = s.checker.CheckDhctlVersionObsolescence()
	t.ErrorIs(err, ErrInstallerVersionMismatch)
}

func (s *PreflightChecksTestSuite) Test_PreflightCheck_CheckDhctlVersionObsolescence_VersionMismatch_DevBranch() {
	t := s.Require()

	s.checker.installConfig.DevBranch = "pr1234"

	image := s.checker.installConfig.GetImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.Descriptor{
			Digest: v1.Hash{
				Algorithm: "sha256",
				Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
			}}, nil)

	s.checker.buildDigestProvider = NewFakeBuildDigestProvider(s.T()).
		Return(
			v1.Hash{
				Algorithm: "sha256",
				Hex:       "a66bcd004c1c83c1cfb118f7652a30c784cad66ce976249aa64d60219ec5b199",
			}, nil)

	err = s.checker.CheckDhctlVersionObsolescence()
	t.ErrorIs(err, ErrInstallerVersionMismatch)
}

func (s *PreflightChecksTestSuite) Test_PreflightCheck_CheckDhctlVersionObsolescence_VersionOverride_ReleaseChannel() {
	t := s.Require()

	app.PreflightSkipDeckhouseVersionCheck = true

	image := s.checker.installConfig.GetImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.Descriptor{Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
		}}, nil)

	s.checker.buildDigestProvider = NewFakeBuildDigestProvider(s.T()).Return(v1.Hash{
		Algorithm: "sha256",
		Hex:       "3490720937602946739407683046730486738046346037406374068347",
	}, nil)

	err = s.checker.CheckDhctlVersionObsolescence()
	t.NoError(err)
}

func (s *PreflightChecksTestSuite) Test_PreflightCheck_CheckDhctlVersionObsolescence_VersionOverride_DevBranch() {
	t := s.Require()

	app.PreflightSkipDeckhouseVersionCheck = true
	s.checker.installConfig.DevBranch = "pr1234"

	image := s.checker.installConfig.GetImage(false)
	ref, err := name.ParseReference(image)
	t.NoError(err)

	s.checker.imageDescriptorProvider = NewFakeImageDescriptorProvider(s.T()).
		ExpectReference(ref).
		Return(&v1.Descriptor{Digest: v1.Hash{
			Algorithm: "sha256",
			Hex:       "95693712d292a6d2e1de6052a0b2189210501393f162616f5d21f2c9b5152129",
		}}, nil)

	s.checker.buildDigestProvider = NewFakeBuildDigestProvider(s.T()).Return(v1.Hash{
		Algorithm: "sha256",
		Hex:       "3490720937602946739407683046730486738046346037406374068347",
	}, nil)

	err = s.checker.CheckDhctlVersionObsolescence()
	t.NoError(err)
}
