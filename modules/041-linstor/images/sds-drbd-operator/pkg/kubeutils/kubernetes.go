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
