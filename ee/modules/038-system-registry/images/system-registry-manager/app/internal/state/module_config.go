/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package state

import (
	"context"
	"encoding/json"
	"fmt"

	utiltime "embeded-registry-manager/internal/utils/time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ModuleConfigApiVersion = "deckhouse.io/v1alpha1"
	ModuleConfigKind       = "ModuleConfig"
)

type ModuleConfig struct {
	Enabled  bool           `json:"enabled"`
	Settings RegistryConfig `json:"settings"`
}

type RegistryMode string // enum

const (
	RegistryModeDirect   RegistryMode = "Direct"
	RegistryModeProxy    RegistryMode = "Proxy"
	RegistryModeDetached RegistryMode = "Detached"
)

type RegistryConfig struct {
	Mode           RegistryMode    `json:"mode"` // enum: Direct, Proxy, Detached
	Proxy          *ProxyConfig    `json:"proxy,omitempty"`
	Detached       *DetachedConfig `json:"detached,omitempty"`
	ImagesOverride ImagesOverride  `json:"imagesOverride,omitempty"`
}

type ImagesOverride struct {
	RegistryManager string `json:"registryManager,omitempty"`
	Mirrorer        string `json:"mirrorer,omitempty"`
}

type StorageMode string // enum: S3, Fs

const (
	StorageModeFS StorageMode = "Fs"
	StorageModeS3 StorageMode = "S3"
)

type DetachedConfig struct {
	StorageMode StorageMode `json:"storageMode"`
}

type ProxyConfig struct {
	Host        string             `json:"host"`
	Scheme      string             `json:"scheme"`
	CA          string             `json:"ca"`
	Path        string             `json:"path"`
	User        string             `json:"user"`
	Password    string             `json:"password"`
	StorageMode StorageMode        `json:"storageMode"`
	TTL         *utiltime.Duration `json:"ttl"`
}

func GetModuleConfigObject() unstructured.Unstructured {
	ret := unstructured.Unstructured{}
	ret.SetAPIVersion(ModuleConfigApiVersion)
	ret.SetKind(ModuleConfigKind)
	ret.SetName(RegistryModuleName)

	return ret
}

func LoadModuleConfig(ctx context.Context, cli client.Client) (config ModuleConfig, err error) {
	key := types.NamespacedName{
		Name: RegistryModuleName,
	}

	configObject := GetModuleConfigObject()

	if err = cli.Get(ctx, key, &configObject); err != nil {
		err = fmt.Errorf("cannot get k8s object: %w", err)
		return
	}

	configSpec, ok, err := unstructured.NestedMap(configObject.Object, "spec")
	if err != nil || !ok {
		err = fmt.Errorf("cannot extract spec: %w", err)
		return
	}

	err = jsonRecode(configSpec, &config)
	if err != nil {
		err = fmt.Errorf("recode error: %w", err)
	}

	return
}

func jsonRecode(input any, output any) error {
	buf, err := json.Marshal(input)

	if err != nil {
		return fmt.Errorf("cannot marshal JSON: %w", err)
	}

	err = json.Unmarshal(buf, output)

	if err != nil {
		return fmt.Errorf("cannot marshal JSON: %w", err)
	}

	return nil
}
