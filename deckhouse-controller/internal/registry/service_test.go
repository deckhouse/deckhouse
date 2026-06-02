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

package registry

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	registryhelpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
)

const existingDockerCfg = "ZXhpc3Rpbmc=" // literally "existing" string

func TestBuildRemote_PackageRepository_DockerCfgPreserved(t *testing.T) {
	pr := &v1alpha1.PackageRepository{}
	pr.Name = "test"
	pr.Spec.Registry.Repo = "dev-registry.deckhouse.io/deckhouse/foxtrot/packages"
	pr.Spec.Registry.DockerCFG = existingDockerCfg
	pr.Spec.Registry.Login = "license-token"
	pr.Spec.Registry.Password = "secret"

	got := BuildRemote(pr)
	// existing dockerCfg must not be overwritten
	assert.Equal(t, got.DockerConfig, existingDockerCfg, "explicit dockerCfg must not be overwritten")
	assert.Equal(t, got.Login, "license-token")
	assert.Equal(t, got.Password, "secret")
}

func TestBuildRemote_PackageRepository_SynthesizeFromLoginPassword(t *testing.T) {
	pr := &v1alpha1.PackageRepository{}
	pr.Name = "test"
	pr.Spec.Registry.Repo = "dev-registry.deckhouse.io/deckhouse/foxtrot/packages"
	pr.Spec.Registry.Login = "license-token"
	pr.Spec.Registry.Password = "secret"

	got := BuildRemote(pr)
	require.NotEmpty(t, got.DockerConfig, "synthesized dockerCfg must not be empty when login is set")

	raw, err := base64.StdEncoding.DecodeString(got.DockerConfig)
	require.NoError(t, err, "DockerConfig must be base64-encoded")

	username, password, err := registryhelpers.CredsFromDockerCfg(raw, "dev-registry.deckhouse.io")
	require.NoError(t, err)
	assert.Equal(t, username, "license-token")
	assert.Equal(t, password, "secret")
}

func TestBuildRemote_PackageRepository_NoCredentials(t *testing.T) {
	pr := &v1alpha1.PackageRepository{}
	pr.Name = "test"
	pr.Spec.Registry.Repo = "dev-registry.deckhouse.io/deckhouse/foxtrot/packages"

	got := BuildRemote(pr)
	assert.Empty(t, got.DockerConfig, "no credentials → no synthesized dockerCfg")
	assert.Empty(t, got.Login)
	assert.Empty(t, got.Password)
}

func TestBuildRemote_ModuleSource_LoginPasswordNotApplicable(t *testing.T) {
	ms := &v1alpha1.ModuleSource{}
	ms.Name = "source"
	ms.Spec.Registry.Repo = "dev-registry.deckhouse.io/deckhouse/modules"
	ms.Spec.Registry.DockerCFG = existingDockerCfg

	got := BuildRemote(ms)
	assert.Equal(t, existingDockerCfg, got.DockerConfig)
	assert.Empty(t, got.Login)
	assert.Empty(t, got.Password)
}
