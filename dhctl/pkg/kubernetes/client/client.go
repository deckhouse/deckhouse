// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"
	"reflect"
	"time"

	klient "github.com/flant/kube-client/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	// oidc allows using oidc provider in kubeconfig
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/local"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type KubeClient interface {
	kubernetes.Interface
	Dynamic() dynamic.Interface
	APIResourceList(apiVersion string) ([]*metav1.APIResourceList, error)
	APIResource(apiVersion, kind string) (*metav1.APIResource, error)
	GroupVersionResource(apiVersion, kind string) (schema.GroupVersionResource, error)
	InvalidateDiscoveryCache()
}

// KubernetesClient connects to kubernetes API server through ssh tunnel and kubectl proxy.
type KubernetesClient struct {
	KubeClient
	NodeInterface node.Interface
	KubeProxy     node.KubeProxy
}

type KubernetesInitParams struct {
	KubeConfig        string
	KubeConfigContext string

	KubeConfigInCluster bool
}

func NewKubernetesClient() *KubernetesClient {
	return &KubernetesClient{}
}

func NewFakeKubernetesClient() *KubernetesClient {
	return &KubernetesClient{KubeClient: klient.NewFake(nil)}
}

func NewFakeKubernetesClientWithListGVR(gvr map[schema.GroupVersionResource]string) *KubernetesClient {
	return &KubernetesClient{KubeClient: klient.NewFake(gvr)}
}

func (k *KubernetesClient) WithNodeInterface(client node.Interface) *KubernetesClient {
	if client != nil && !reflect.ValueOf(client).IsNil() {
		k.NodeInterface = client
	}
	return k
}

func (k *KubernetesClient) NodeInterfaceAsSSHClient() node.SSHClient {
	if k.NodeInterface == nil || reflect.ValueOf(k.NodeInterface).IsNil() {
		return nil
	}

	cl, ok := k.NodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil
	}

	return cl.Client()
}

// Init initializes kubernetes client
func (k *KubernetesClient) Init(params *KubernetesInitParams) error {
	return k.InitContext(context.Background(), params)
}

func (k *KubernetesClient) InitContext(ctx context.Context, params *KubernetesInitParams) error {
	return k.initContext(ctx, params)
}

func (k *KubernetesClient) initContext(ctx context.Context, params *KubernetesInitParams) error {
	kubeClient := klient.New()
	kubeClient.WithRateLimiterSettings(30, 60)
	_, isLocalRun := k.NodeInterface.(*local.NodeInterface)

	switch {
	case params.KubeConfigInCluster:
	case params.KubeConfig != "":
		kubeClient.WithContextName(params.KubeConfigContext)
		kubeClient.WithConfigPath(params.KubeConfig)
	case isLocalRun:
		_, err := k.StartKubernetesProxy(ctx)
		if err != nil {
			return err
		}
	default:
		port, err := k.StartKubernetesProxy(ctx)
		if err != nil {
			return err
		}
		kubeClient.WithServer("http://localhost:" + port)
	}

	// Initialize kube client for kube events hooks.
	err := kubeClient.Init()
	if err != nil {
		return fmt.Errorf("initialize kube client: %s", err)
	}

	k.KubeClient = kubeClient
	return nil
}

// StartKubernetesProxy initializes kubectl-proxy on remote host and establishes ssh tunnel to it
func (k *KubernetesClient) StartKubernetesProxy(ctx context.Context) (port string, err error) {
	if wrapper, ok := k.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
		if port, err = k.startRemoteKubeProxy(ctx, wrapper.Client()); err != nil {
			return "", fmt.Errorf("start kube proxy: %s", err)
		}
		return port, nil
	}

	return "6445", nil
}

func (k *KubernetesClient) startRemoteKubeProxy(ctx context.Context, sshCl node.SSHClient) (port string, err error) {
	err = retry.NewLoop("Starting kube proxy", sshCl.Session().CountHosts(), 1*time.Second).
		RunContext(ctx, func() error {
			log.InfoF("Using host %s\n", sshCl.Session().Host())

			k.KubeProxy = sshCl.KubeProxy()
			port, err = k.KubeProxy.Start(-1)

			if err != nil {
				sshCl.Session().ChoiceNewHost()
				return fmt.Errorf("start kube proxy: %v", err)
			}

			return nil
		})

	if err != nil {
		return "", err
	}

	log.InfoF("Proxy started on port %s\n", port)
	return port, nil
}

func AppKubernetesInitParams() *KubernetesInitParams {
	return &KubernetesInitParams{
		KubeConfig:          app.KubeConfig,
		KubeConfigContext:   app.KubeConfigContext,
		KubeConfigInCluster: app.KubeConfigInCluster,
	}
}
