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
