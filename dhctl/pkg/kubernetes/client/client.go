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

//nolint:gci
import (
	"context"
	"fmt"
	"reflect"

	klient "github.com/flant/kube-client/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	// oidc allows using oidc provider in kubeconfig
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/local"
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
	RestConfig          *rest.Config
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
	case params.RestConfig != nil:
		kubeClient.WithRestConfig(params.RestConfig)
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

	// allow only accept json for prevent
	// return protobuf from server
	// because we log all requests/responses to log
	// debug log is "broken" because protobuf response
	// output as formatted byte array
	kubeClient.WithAcceptOnlyJSONContentType(true)

	// Initialize kube client for kube events hooks.
	err := kubeClient.Init()
	if err != nil {
		return fmt.Errorf("initialize kube client: %s", err)
	}

	k.KubeClient = kubeClient
	return nil
}

// StartKubernetesProxy returns the local port the in-cluster kube-proxy
// listens on. The legacy SSH-tunneled remote-proxy fallback was removed
// together with dhctl's old SSH packages — modern callers build the
// KubeClient through libcon (providerinitializer.GetProviders) before
// reaching Init, so the default-port branch is the only path left here.
func (k *KubernetesClient) StartKubernetesProxy(_ context.Context) (string, error) {
	return "6445", nil
}

// AppKubernetesInitParams builds *KubernetesInitParams from the supplied
// kube options. Returns zero values when kube is nil.
func AppKubernetesInitParams(kube *options.KubeOptions) *KubernetesInitParams {
	if kube == nil {
		return &KubernetesInitParams{}
	}
	return &KubernetesInitParams{
		KubeConfig:          kube.Config,
		KubeConfigContext:   kube.ConfigContext,
		KubeConfigInCluster: kube.InCluster,
	}
}
