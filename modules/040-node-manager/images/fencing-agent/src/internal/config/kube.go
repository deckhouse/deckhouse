package fencingconfig

import (
	"fencing-agent/internal/config/validators"
	"time"
)

type KubeConfig struct {
	KubeAPIRPS                 int           `env:"KUBERNETES_API_RPS" env-default:"10"`
	KubeAPIBurst               int           `env:"KUBERNETES_API_BURST" env-default:"100"`
	KubeConfigPath             string        `env:"KUBECONFIG" env-required:"false"`
	KubernetesAPICheckInterval time.Duration `env:"KUBERNETES_API_CHECK_INTERVAL" env-default:"5s"`
	KubernetesAPITimeout       time.Duration `env:"KUBERNETES_API_TIMEOUT" env-default:"10s"`
}

func (kc *KubeConfig) validate() error {
	if unaryErr := validators.ValidateRateLimit(kc.KubeAPIRPS, kc.KubeAPIBurst, "kubernetesAPI"); unaryErr != nil {
		return unaryErr
	}
	return nil
}
