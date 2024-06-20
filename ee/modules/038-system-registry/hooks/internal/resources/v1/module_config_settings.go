/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resources

import (
	"encoding/base64"
	"fmt"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"strings"
)

// Init secret
type InitSecretData struct {
	Data *struct {
		RegistryMode            *[]byte `json:"registryMode,omitempty" yaml:"registryMode,omitempty"`
		UpstreamRegistryAddress *[]byte `json:"upstreamRegistryAddress,omitempty" yaml:"upstreamRegistryAddress,omitempty"`
		UpstreamRegistryAuth    *[]byte `json:"upstreamRegistryAuth,omitempty" yaml:"upstreamRegistryAuth,omitempty"`
		UpstreamRegistryCA      *[]byte `json:"upstreamRegistryCA,omitempty" yaml:"upstreamRegistryCA,omitempty"`
		UpstreamRegistryPath    *[]byte `json:"upstreamRegistryPath,omitempty" yaml:"upstreamRegistryPath,omitempty"`
		UpstreamRegistryScheme  *[]byte `json:"upstreamRegistryScheme,omitempty" yaml:"upstreamRegistryScheme,omitempty"`
	} `json:"data,omitempty" yaml:"data,omitempty"`
}

// Module Config Settings
type ModuleConfigSettings struct {
	RegistryMode     *string                               `json:"registryMode,omitempty" yaml:"registryMode,omitempty"`
	UpstreamRegistry *ModuleConfigSettingsUpstreamRegistry `json:"upstreamRegistry,omitempty" yaml:"upstreamRegistry,omitempty"`
}

type ModuleConfigSettingsUpstreamRegistry struct {
	UpstreamRegistryHost     *string `json:"upstreamRegistryHost,omitempty" yaml:"upstreamRegistryHost,omitempty"`
	UpstreamRegistryScheme   *string `json:"upstreamRegistryScheme,omitempty" yaml:"upstreamRegistryScheme,omitempty"`
	UpstreamRegistryCa       *string `json:"upstreamRegistryCa,omitempty" yaml:"upstreamRegistryCa,omitempty"`
	UpstreamRegistryPath     *string `json:"upstreamRegistryPath,omitempty" yaml:"upstreamRegistryPath,omitempty"`
	UpstreamRegistryUser     *string `json:"upstreamRegistryUser,omitempty" yaml:"upstreamRegistryUser,omitempty"`
	UpstreamRegistryPassword *string `json:"upstreamRegistryPassword,omitempty" yaml:"upstreamRegistryPassword,omitempty"`
}

// Module Config funcs
func NewModuleConfigByInitSecret(secretData *InitSecretData) (*v1alpha1.ModuleConfig, error) {
	settings, err := fromInitSecretDataToModuleConfigSettings(secretData)
	if err != nil {
		return nil, err
	}
	mapStructSettings, err := FromModuleConfigSettingsToUnstructured(settings)
	if err != nil {
		return nil, err
	}

	newModuleConfig := v1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleConfig",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "system-registry",
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Version:  1,
			Settings: mapStructSettings,
			Enabled:  pointer.Bool(true),
		},
	}
	return &newModuleConfig, nil
}

func fromInitSecretDataToModuleConfigSettings(initData *InitSecretData) (*ModuleConfigSettings, error) {
	if initData == nil || initData.Data == nil {
		return nil, fmt.Errorf("initData or initData.Data is nil")
	}

	decodeBase64 := func(data *[]byte) (*[]byte, error) {
		if data == nil {
			return nil, nil
		}
		if len(*data) == 0 {
			return nil, nil
		}
		decoded, err := base64.StdEncoding.DecodeString(string(*data))
		if err != nil {
			return nil, err
		}
		return &decoded, nil
	}

	toString := func(data *[]byte) *string {
		if data == nil {
			return nil
		}
		strData := string(*data)
		strData = strings.TrimSpace(strData)
		return &strData
	}

	var upstreamRegistryUser, upstreamRegistryPassword *string

	if initData.Data.UpstreamRegistryAuth != nil {
		// Double decode
		authData, err := decodeBase64(initData.Data.UpstreamRegistryAuth)
		if err != nil {
			return nil, err
		}

		authStr := toString(authData)
		if authStr != nil {
			parts := strings.Split(*authStr, ":")
			if len(parts) == 2 {
				upstreamRegistryUser = &parts[0]
				upstreamRegistryPassword = &parts[1]
			}
		}
	}

	return &ModuleConfigSettings{
		RegistryMode: toString(initData.Data.RegistryMode),
		UpstreamRegistry: &ModuleConfigSettingsUpstreamRegistry{
			UpstreamRegistryHost:     toString(initData.Data.UpstreamRegistryAddress),
			UpstreamRegistryScheme:   toString(initData.Data.UpstreamRegistryScheme),
			UpstreamRegistryCa:       toString(initData.Data.UpstreamRegistryCA),
			UpstreamRegistryPath:     toString(initData.Data.UpstreamRegistryPath),
			UpstreamRegistryUser:     upstreamRegistryUser,
			UpstreamRegistryPassword: upstreamRegistryPassword,
		},
	}, nil
}

func FromModuleConfigSettingsToUnstructured(mcSettings *ModuleConfigSettings) (map[string]interface{}, error) {
	if mcSettings == nil {
		return map[string]interface{}{}, nil
	}
	unstruct, err := sdk.ToUnstructured(mcSettings)
	if err != nil {
		return nil, err
	}
	if unstruct == nil {
		return map[string]interface{}{}, nil
	}
	return unstruct.Object, nil
}

func FromUnstructuredToModuleConfigSettings(mcSettings map[string]interface{}) (*ModuleConfigSettings, error) {
	if mcSettings == nil {
		return &ModuleConfigSettings{}, nil
	}
	var mcSettingsStruct ModuleConfigSettings
	err := sdk.FromUnstructured(&unstructured.Unstructured{Object: mcSettings}, &mcSettingsStruct)
	return &mcSettingsStruct, err
}

func PrepareModuleConfigByInitSettings(moduleConfig *v1alpha1.ModuleConfig, secretData *InitSecretData) error {
	if moduleConfig == nil {
		return fmt.Errorf("moduleConfig is nil")
	}
	if secretData == nil {
		return fmt.Errorf("secretData is nil")
	}
	settingsData, err := fromInitSecretDataToModuleConfigSettings(secretData)
	if err != nil {
		return err
	}
	moduleConfigData, err := FromUnstructuredToModuleConfigSettings(moduleConfig.Spec.Settings)
	if err != nil {
		return err
	}

	if moduleConfigData.RegistryMode == nil {
		moduleConfigData.RegistryMode = settingsData.RegistryMode
	}
	if moduleConfigData.UpstreamRegistry == nil {
		moduleConfigData.UpstreamRegistry = settingsData.UpstreamRegistry
	} else {
		if moduleConfigData.UpstreamRegistry.UpstreamRegistryHost == nil {
			moduleConfigData.UpstreamRegistry.UpstreamRegistryHost = settingsData.UpstreamRegistry.UpstreamRegistryHost
		}
		if moduleConfigData.UpstreamRegistry.UpstreamRegistryScheme == nil {
			moduleConfigData.UpstreamRegistry.UpstreamRegistryScheme = settingsData.UpstreamRegistry.UpstreamRegistryScheme
		}
		if moduleConfigData.UpstreamRegistry.UpstreamRegistryCa == nil {
			moduleConfigData.UpstreamRegistry.UpstreamRegistryCa = settingsData.UpstreamRegistry.UpstreamRegistryCa
		}
		if moduleConfigData.UpstreamRegistry.UpstreamRegistryPath == nil {
			moduleConfigData.UpstreamRegistry.UpstreamRegistryPath = settingsData.UpstreamRegistry.UpstreamRegistryPath
		}
		if moduleConfigData.UpstreamRegistry.UpstreamRegistryUser == nil {
			moduleConfigData.UpstreamRegistry.UpstreamRegistryUser = settingsData.UpstreamRegistry.UpstreamRegistryUser
		}
		if moduleConfigData.UpstreamRegistry.UpstreamRegistryPassword == nil {
			moduleConfigData.UpstreamRegistry.UpstreamRegistryPassword = settingsData.UpstreamRegistry.UpstreamRegistryPassword
		}
	}
	newModuleConfigData, err := FromModuleConfigSettingsToUnstructured(moduleConfigData)
	if err != nil {
		return err
	}

	moduleConfig.Spec.Settings = newModuleConfigData
	return nil
}
