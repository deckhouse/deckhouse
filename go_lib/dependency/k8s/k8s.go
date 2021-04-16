package k8s

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
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
