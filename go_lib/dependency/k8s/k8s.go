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

	// Default value, used in hook config - 1.23
	DefaultFakeClusterVersion = fake.ClusterVersionV123
)

type Client interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
}

type k8sClient struct {
	*kubernetes.Clientset
	dynamicClient dynamic.Interface
}

func NewClient(options ...Option) (Client, error) {
	opts := &k8sOptions{}

	for _, opt := range options {
		opt(opts)
	}

	var config *rest.Config
	var err error
	if opts.kubeconfigPath != "" {
		config, err = clientcmd.BuildConfigFromFlags("", opts.kubeconfigPath)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
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
}

type Option func(options *k8sOptions)

// WithKubeConfig pass external kube config file to make a client
func WithKubeConfig(kubeConfigPath string) Option {
	return func(options *k8sOptions) {
		options.kubeconfigPath = kubeConfigPath
	}
}
