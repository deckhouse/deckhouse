/*
Copyright 2023 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"fmt"
	"io/ioutil"

	kube "github.com/flant/kube-client/client"
	v1 "k8s.io/api/core/v1"
)

const DefaultAlpineImage = "alpine:3.12"

// Access provides Kubernetes access
type Access interface {
	Kubernetes() kube.Client
	ServiceAccountToken() string
	UserAgent() string

	// probe-specific

	SchedulerProbeImage() *ProbeImage
	SchedulerProbeNode() string

	CloudControllerManagerNamespace() string

	ClusterDomain() string
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
	return &ProbeImage{
		name:        cfg.Name,
		pullSecrets: cfg.PullSecrets,
	}
}

func (p *ProbeImage) Name() string {
	return p.name
}

func (p *ProbeImage) PullSecrets() []v1.LocalObjectReference {
	// Copy to guarantee immutability
	secrets := make([]v1.LocalObjectReference, len(p.pullSecrets))
	for i, name := range p.pullSecrets {
		secrets[i] = v1.LocalObjectReference{Name: name}
	}
	return secrets
}

type Config struct {
	Context     string
	Config      string
	Server      string
	ClientQps   float32
	ClientBurst int

	SchedulerProbeImage ProbeImageConfig
	SchedulerProbeNode  string

	CloudControllerManagerNamespace string

	ClusterDomain string
}

// Accessor provides Kubernetes access in pod
type Accessor struct {
	client    kube.Client
	saToken   string
	userAgent string

	schedulerProbeImage *ProbeImage
	schedulerProbeNode  string

	cloudControllerManagerNamespace string

	kubernetesDomain string
}

func (a *Accessor) Init(config *Config, userAgent string) error {
	// Kubernetes client
	a.client = kube.New()
	a.client.WithContextName(config.Context)
	a.client.WithConfigPath(config.Config)
	a.client.WithRateLimiterSettings(config.ClientQps, config.ClientBurst)
	// TODO(nabokihms): add kubernetes client metrics
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

	a.schedulerProbeImage = NewProbeImage(&config.SchedulerProbeImage)
	a.schedulerProbeNode = config.SchedulerProbeNode

	a.cloudControllerManagerNamespace = config.CloudControllerManagerNamespace

	a.kubernetesDomain = "kubernetes.default.svc." + config.ClusterDomain + "." // Trailing dot to avoid domain search
	a.userAgent = userAgent

	return nil
}

func (a *Accessor) Kubernetes() kube.Client {
	return a.client
}

func (a *Accessor) ServiceAccountToken() string {
	return a.saToken
}

func (a *Accessor) UserAgent() string {
	return a.userAgent
}

func (a *Accessor) SchedulerProbeImage() *ProbeImage {
	return a.schedulerProbeImage
}

func (a *Accessor) SchedulerProbeNode() string {
	return a.schedulerProbeNode
}

func (a *Accessor) CloudControllerManagerNamespace() string {
	return a.cloudControllerManagerNamespace
}

func (a *Accessor) ClusterDomain() string {
	return a.kubernetesDomain
}

func FakeAccessor() *Accessor {
	return &Accessor{
		client: kube.NewFake(nil),
	}
}
