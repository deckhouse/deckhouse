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

package kubutils

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateKubernetesClient(config *rest.Config, schema *runtime.Scheme) (kclient.Client, error) {
	var kc kclient.Client
	kc, err := kclient.New(config, kclient.Options{
		Scheme: schema,
	})
	if err != nil {
		return kc, fmt.Errorf("error create kubernetes client %w", err)
	}
	return kc, err
}

func KubernetesDefaultConfigCreate() (*rest.Config, error) {
	//todo validate empty
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	)
	// Get a config to talk to API server
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("config kubernetes error %w", err)
	}
	return config, nil
}

func GetNodeUID(ctx context.Context, kc kclient.Client, nodeName string) (string, error) {
	node := &v1.Node{}
	err := kc.Get(ctx, kclient.ObjectKey{Name: nodeName}, node)
	if err != nil {
		return "", fmt.Errorf("get node error %w", err)
	}
	return string(node.UID), nil
}
