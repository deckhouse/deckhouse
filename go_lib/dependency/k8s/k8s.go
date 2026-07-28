/*
Copyright 2021 Flant JSC

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

package k8s

import (
	"fmt"

	"github.com/flant/kube-client/fake"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type FakeClusterVersion = fake.ClusterVersion

const (
	V116 FakeClusterVersion = fake.ClusterVersionV116
	V117 FakeClusterVersion = fake.ClusterVersionV117
	V118 FakeClusterVersion = fake.ClusterVersionV118
	V119 FakeClusterVersion = fake.ClusterVersionV119
	V120 FakeClusterVersion = fake.ClusterVersionV120
	V121 FakeClusterVersion = fake.ClusterVersionV121
	V122 FakeClusterVersion = fake.ClusterVersionV122
	V123 FakeClusterVersion = fake.ClusterVersionV123
	V124 FakeClusterVersion = fake.ClusterVersionV124
	V125 FakeClusterVersion = fake.ClusterVersionV125

	// Default value, used in hook config - 1.25
	DefaultFakeClusterVersion = fake.ClusterVersionV125
)

type Client interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
}

type k8sClient struct {
	*kubernetes.Clientset
	dynamicClient dynamic.Interface
}

// defaultKubeConfigPath and defaultKubeContext point the clients built by this
// package at an explicit kubeconfig instead of the in-cluster service account.
// They are set once at startup via SetDefaultKubeConfig and read-only afterwards.
var (
	defaultKubeConfigPath string
	defaultKubeContext    string
)

// SetDefaultKubeConfig makes NewClient and RESTConfig build their rest.Config
// from the given kubeconfig instead of the in-cluster service account token. An
// empty path keeps the in-cluster behavior. It must be called once during
// startup, before any client is created, so deckhouse-controller can drive a
// remote cluster through the same --kube-config the operator's own client uses.
func SetDefaultKubeConfig(path, context string) {
	defaultKubeConfigPath = path
	defaultKubeContext = context
}

// RESTConfig resolves a *rest.Config honoring the kubeconfig set by
// SetDefaultKubeConfig (or overridden per call via WithKubeConfig/WithKubeContext),
// and falls back to the in-cluster config when no kubeconfig is configured.
func RESTConfig(options ...Option) (*rest.Config, error) {
	opts := &k8sOptions{
		kubeconfigPath: defaultKubeConfigPath,
		kubeContext:    defaultKubeContext,
	}

	for _, opt := range options {
		opt(opts)
	}

	if opts.kubeconfigPath == "" {
		return rest.InClusterConfig()
	}

	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: opts.kubeconfigPath}
	overrides := &clientcmd.ConfigOverrides{}
	if opts.kubeContext != "" {
		overrides.CurrentContext = opts.kubeContext
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig %q: %w", opts.kubeconfigPath, err)
	}

	return config, nil
}

func NewClient(options ...Option) (Client, error) {
	config, err := RESTConfig(options...)
	if err != nil {
		return nil, fmt.Errorf("rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	d, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	return &k8sClient{clientset, d}, nil
}

func (k k8sClient) Dynamic() dynamic.Interface {
	return k.dynamicClient
}

type k8sOptions struct {
	kubeconfigPath string
	kubeContext    string
}

type Option func(options *k8sOptions)

// WithKubeConfig pass external kube config file to make a client
func WithKubeConfig(kubeConfigPath string) Option {
	return func(options *k8sOptions) {
		options.kubeconfigPath = kubeConfigPath
	}
}

// WithKubeContext selects a context from the kubeconfig passed via WithKubeConfig
// or SetDefaultKubeConfig. It is ignored when no kubeconfig is configured.
func WithKubeContext(kubeContext string) Option {
	return func(options *k8sOptions) {
		options.kubeContext = kubeContext
	}
}
