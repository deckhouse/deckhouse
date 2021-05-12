package kubernetes

import (
	"fmt"
	"io/ioutil"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	"k8s.io/client-go/kubernetes"
)

// Access provides Kubernetes access
type Access struct {
	client  kube.KubernetesClient
	saToken string
}

type Config struct {
	Context     string
	Config      string
	Server      string
	ClientQps   float32
	ClientBurst int
}

func (a *Access) Init(config *Config) error {
	// Kubernetes client
	a.client = kube.NewKubernetesClient()
	a.client.WithContextName(config.Context)
	a.client.WithConfigPath(config.Config)
	a.client.WithRateLimiterSettings(config.ClientQps, config.ClientBurst)
	a.client.WithMetricStorage(metric_storage.NewMetricStorage())
	err := a.client.Init()
	if err != nil {
		return fmt.Errorf("cannot init kuberbetes client: %v", err)
	}

	// Service account token
	token, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		return fmt.Errorf("pod expected, cannot read service account token: %v", err)
	}
	a.saToken = string(token)

	return nil
}

func (a *Access) Kubernetes() kubernetes.Interface {
	return a.client
}

func (a *Access) ServiceAccountToken() string {
	return a.saToken
}
