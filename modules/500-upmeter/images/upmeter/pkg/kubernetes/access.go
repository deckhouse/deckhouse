package kubernetes

import (
	"fmt"
	"io/ioutil"

	shapp "github.com/flant/shell-operator/pkg/app"
	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	"k8s.io/client-go/kubernetes"
)

// Access provides Kubernetes access
type Access struct {
	client  kube.KubernetesClient
	saToken string
}

func (a *Access) Init() error {
	// Kubernetes client
	a.client = kube.NewKubernetesClient()
	a.client.WithContextName(shapp.KubeContext)
	a.client.WithConfigPath(shapp.KubeConfig)
	a.client.WithRateLimiterSettings(shapp.KubeClientQps, shapp.KubeClientBurst)
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
