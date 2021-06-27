// Copyright 2021 Flant CJSC
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

package docker_registry_watcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LoadDockerConfig_Auths_Key(t *testing.T) {
	secret := `{"auths":{"registry.example.com":{"username":"user"}, "registry2.company.io":{"password":"qwer1234"}}}`

	err := LoadDockerRegistrySecret([]byte(secret))

	assert.NoError(t, err, "should not be an error")

	assert.NotEmpty(t, DockerCfgAuths)
	assert.Contains(t, DockerCfgAuths, "registry.example.com")
	assert.Equal(t, DockerCfgAuths["registry.example.com"].Username, "user")
	assert.Contains(t, DockerCfgAuths, "registry2.company.io")
	assert.Equal(t, DockerCfgAuths["registry2.company.io"].Password, "qwer1234")
}

func Test_LoadDockerConfig_One(t *testing.T) {
	secret := `{"registry.example.com":{"username":"user","password":"qwer1234"}}`

	err := LoadDockerRegistrySecret([]byte(secret))

	assert.NoError(t, err, "should not be an error")

	assert.NotEmpty(t, DockerCfgAuths)
	assert.Contains(t, DockerCfgAuths, "registry.example.com")
	assert.Equal(t, DockerCfgAuths["registry.example.com"].Username, "user")
	assert.Equal(t, DockerCfgAuths["registry.example.com"].Password, "qwer1234")
}

func Test_LoadDockerConfig_EmptyObject(t *testing.T) {
	secret := `{}`

	err := LoadDockerRegistrySecret([]byte(secret))

	assert.NoError(t, err, "should not be an error")

	assert.Empty(t, DockerCfgAuths)
}

func Test_LoadDockerConfig_Empty_IsError(t *testing.T) {
	secret := ``

	err := LoadDockerRegistrySecret([]byte(secret))

	assert.Error(t, err, "should be an error")
}
