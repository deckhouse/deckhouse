package docker_registry_manager

import (
	"encoding/json"
	"fmt"
)

// DockerConfigJSON represents a local docker auth config file
// for pulling images.
type DockerConfigJSON struct {
	Auths DockerConfig `json:"auths"`

	XXX map[string]interface{} `yaml:",inline"`
}

// DockerConfig represents the config file used by the docker CLI.
// This config that represents the credentials that should be used
// when pulling images from specific image repositories.
type DockerConfig map[string]DockerConfigEntry

type DockerConfigEntry struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Email    string `json:"email,omitempty"`
	Auth     string `json:"auth,omitempty"`

	XXX map[string]interface{} `yaml:",inline"`
}

// DockerCfgAuths stores all available registries and their auths
var DockerCfgAuths = DockerConfig{}

// Anonymous auth for unknown registries
var AnonymousAuth = DockerConfigEntry{}

func LoadDockerRegistrySecret(bytes []byte) error {
	//
	var tmpVar interface{}
	err := json.Unmarshal(bytes, &tmpVar)
	if err != nil {
		return err
	}

	isAuths := false
	if tmpMap, ok := tmpVar.(map[string]interface{}); ok {
		if _, hasKey := tmpMap["auths"]; hasKey {
			isAuths = hasKey
		}
	} else {
		return fmt.Errorf("bad JSON structure: should be an object.")
	}

	if isAuths {
		// unmarshal as DockerConfigJson
		tmpConfigJson := DockerConfigJSON{}
		err := json.Unmarshal(bytes, &tmpConfigJson)
		if err != nil {
			return err
		}
		DockerCfgAuths = tmpConfigJson.Auths
	} else {
		// copy all from top keys
		err := json.Unmarshal(bytes, &DockerCfgAuths)
		if err != nil {
			return err
		}
	}

	return nil
}
