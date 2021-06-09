package kubernetes

import (
	"fmt"
	"io/ioutil"

	"github.com/flant/shell-operator/pkg/kube"
	"github.com/flant/shell-operator/pkg/metric_storage"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

const DefaultAlpineImage = "alpine:3.12"

// Access provides Kubernetes access
type Access interface {
	Kubernetes() kubernetes.Interface
	ServiceAccountToken() string
	CpSchedulerImage() *ProbeImage
}

type ProbeImageConfig struct {
	Name        string
	PullSecrets []string
}

type ProbeImage struct {
	name        string
	pullSecrets []string
}

func NewProbeImage(cfg *ProbeImageConfig) *ProbeImage {
	name := cfg.Name
	if name == "" {
		name = DefaultAlpineImage
	}

	return &ProbeImage{
		name:        name,
		pullSecrets: cfg.PullSecrets,
	}
}

func (p *ProbeImage) GetImageName() string {
	return p.name
}

// Accessor provides Kubernetes access in pod
type Accessor struct {
	client          kube.KubernetesClient
	saToken         string
	cpPodProbeImage *ProbeImage
}

type Config struct {
	Context     string
	Config      string
	Server      string
	ClientQps   float32
	ClientBurst int

	CpSchedulerImage ProbeImageConfig
}

func (p *ProbeImage) GetPullSecrets() []v1.LocalObjectReference {
	// yes, always make copy
	// with copy ProbeImage always immutable
	// because slice is reference type and may changed outside ProbeImage
	pullSecrets := make([]v1.LocalObjectReference, 0)
	for _, s := range p.pullSecrets {
		pullSecrets = append(pullSecrets, v1.LocalObjectReference{
			Name: s,
		})
	}
	return pullSecrets
}

func (a *Accessor) Init(config *Config) error {
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
	a.cpPodProbeImage = NewProbeImage(&config.CpSchedulerImage)

	return nil
}

func (a *Accessor) Kubernetes() kubernetes.Interface {
	return a.client
}

func (a *Accessor) ServiceAccountToken() string {
	return a.saToken
}

func (a *Accessor) CpSchedulerImage() *ProbeImage {
	return a.cpPodProbeImage
}
