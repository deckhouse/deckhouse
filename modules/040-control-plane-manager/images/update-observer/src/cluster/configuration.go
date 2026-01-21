package cluster

import (
	"fmt"
	"strings"
	"update-observer/common"

	"github.com/stretchr/testify/assert/yaml"
	corev1 "k8s.io/api/core/v1"
)

type UpdateMode string

const (
	UpdateModeAutomatic UpdateMode = "Automatic"
	UpdateModeManual    UpdateMode = "Manual"
)

type Configuration struct {
	KubernetesVersion string `yaml:"kubernetesVersion"`
	DesiredVersion    string `yaml:"desiredVersion"`
	UpdateMode        UpdateMode
}

func GetConfiguration(secret *corev1.Secret) (*Configuration, error) {
	rawCfg, ok := secret.Data[clusterConfigurationYAML]
	if !ok {
		return nil, fmt.Errorf("'%s' is not found", clusterConfigurationYAML)
	}

	var cfg *Configuration
	if err := yaml.Unmarshal(rawCfg, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal '%s': %w", clusterConfigurationYAML, err)
	}

	var err error
	if cfg.KubernetesVersion == string(UpdateModeAutomatic) {
		cfg.UpdateMode = UpdateModeAutomatic

		rawDefault, ok := secret.Data[defaultKubernetesVersion]
		if !ok {
			return nil, fmt.Errorf("'%s' is not found", rawDefault)
		}

		desiredVersion := strings.TrimSpace(string(rawDefault))
		if desiredVersion == "" {
			return nil, fmt.Errorf("'%s' is empty", defaultKubernetesVersion)
		}

		cfg.DesiredVersion, err = common.NormalizeVersion(desiredVersion)
		if err != nil {
			return nil, fmt.Errorf("'%s' is not valid: %w", defaultKubernetesVersion, err)
		}
	} else {
		cfg.UpdateMode = UpdateModeManual
		cfg.DesiredVersion, err = common.NormalizeVersion(cfg.KubernetesVersion)
		if err != nil {
			return nil, fmt.Errorf("kubernetesVersion is not valid: %w", err)
		}
	}

	return cfg, nil
}
