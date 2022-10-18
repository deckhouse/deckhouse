/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/go-containerregistry/pkg/authn"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	revokedCMName      = "madison-revoked-project"
	revokedCMNamespace = "d8-monitoring"
	revokedCMBinding   = revokedCMName

	globalRegistryPath     = "global.modulesImages.registry"
	dockerConfigPath       = "/etc/registrysecret/.dockerconfigjson"
	internalLicenseKeyPath = "flantIntegration.internal.licenseKey"
	licenseKeyPath         = "flantIntegration.licenseKey"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       revokedCMBinding,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{revokedCMName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{revokedCMNamespace},
				},
			},
			// Synchronization is redundant because of OnBeforeHelm.
			ExecuteHookOnSynchronization: go_hook.Bool(false),
			FilterFunc:                   filterRevokedConfigMap,
		},
	},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
}, handle)

func filterRevokedConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

// This hook discovers license key from values or docker config, and puts it into internal values to use elsewhere.
func handle(input *go_hook.HookInput) error {
	// Remove license from internal values if 'revoked' ConfigMap is present.
	if len(input.Snapshots[revokedCMBinding]) > 0 {
		input.Values.Remove(internalLicenseKeyPath)
		return nil
	}

	// Get license key from configuration.
	configLicenseKey := input.ConfigValues.Get("flantIntegration.licenseKey").String()
	if configLicenseKey != "" {
		input.Values.Set(internalLicenseKeyPath, configLicenseKey)
		return nil
	}

	// Get license key from docker registry config. License key is the password to access container registry.
	registry := input.Values.Get(globalRegistryPath).String()
	licenseKey, err := getLicenseKeyFromDockerConfig(registry, dockerConfigPath)
	if err != nil {
		return err
	}
	input.Values.Set(internalLicenseKeyPath, licenseKey)

	return nil
}

func getLicenseKeyFromDockerConfig(registryValue, dockerConfigPath string) (string, error) {
	registryHost, err := parseRegistryHost(registryValue)
	if err != nil {
		return "", fmt.Errorf("empty registry: %v", err)
	}

	cfg, err := readFile(dockerConfigPath)
	if err != nil {
		log.Warnf("cannot open %q: %v", dockerConfigPath, err)
		return "", fmt.Errorf(`cannot find license key in docker config file; set "flantIntegration.licenseKey" in deckhouse configmap`)
	}

	return parseLicenseKeyFromDockerCredentials(cfg, registryHost)
}

func parseRegistryHost(repo string) (string, error) {
	if repo == "" {
		return "", fmt.Errorf("repo is empty")
	}
	repoSegments := strings.Split(repo, "/")
	if len(repoSegments) == 0 {
		return "", fmt.Errorf("repo is empty")
	}
	registry := repoSegments[0]
	return registry, nil
}

func parseLicenseKeyFromDockerCredentials(dockerConfig []byte, registry string) (string, error) {
	var auth dockerFileConfig
	err := json.Unmarshal(dockerConfig, &auth)
	if err != nil {
		return "", fmt.Errorf("cannot decode docker config JSON: %v", err)
	}
	creds, ok := auth.Auths[registry]
	if !ok {
		return "", fmt.Errorf("no credentials for current registry")
	}

	var license string
	if creds.Password != "" {
		license = creds.Password
	} else if creds.Auth != "" {
		auth, err := base64.StdEncoding.DecodeString(creds.Auth)
		if err != nil {
			return "", fmt.Errorf(`cannot decode base64 "auth" field`)
		}
		parts := strings.Split(string(auth), ":")
		if len(parts) != 2 {
			return "", fmt.Errorf(`unexpected format of "auth" field`)
		}
		license = parts[1]
	}

	if license == "" {
		return "", fmt.Errorf("licenseKey not set in dockerconfig")
	}
	return strings.TrimSpace(license), nil
}

/*
	{ "auths":{
	        "registry.example.com":{
			"username":"oauth2",
			"password":"...",
			"auth":"...",
			"email":"...@example.com"
		}
	}}
*/
type dockerFileConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

// Define global to mock file reading in tests.
// TODO Add FS abstraction layer to dependency.Container.
var readFile = ioutil.ReadFile
