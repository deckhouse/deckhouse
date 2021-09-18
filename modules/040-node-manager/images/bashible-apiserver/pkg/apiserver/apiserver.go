package apiserver

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"d8.io/bashible/pkg/apis/bashible"
	"d8.io/bashible/pkg/apis/bashible/install"
	bashibleregistry "d8.io/bashible/pkg/registry"
	"d8.io/bashible/pkg/template"
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	install.Install(Scheme)

	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

// ExtraConfig holds custom apiserver config
type ExtraConfig struct { // Place you custom config here.
}

// Config defines the config for the apiserver
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// BashibleServer contains state for a Kubernetes cluster master/api server.
type BashibleServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside of this package.
type CompletedConfig struct {
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return CompletedConfig{&c}
}

// New returns a new instance of BashibleServer from the given config.
func (c completedConfig) New() (*BashibleServer, error) {
	genericServer, err := c.GenericConfig.New("bashible-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	s := &BashibleServer{
		GenericAPIServer: genericServer,
	}

	// Config hardcode, could be put to `ExtraConfig`
	const (
		labelSelector    = "app=bashible-apiserver"
		namespace        = "d8-cloud-instance-manager"
		secretName       = "bashible-apiserver-context"
		secretKey        = "context.yaml"
		templatesRootDir = "/bashible/templates"
		resyncTimeout    = 30 * time.Minute
	)

	// Bashible context and its dynamic update
	factory, err := newBashibleInformerFactory(resyncTimeout, namespace, labelSelector)
	if err != nil {
		panic("cannot create informer " + err.Error())
	}

	cachesManager := bashibleregistry.NewCachesManager()
	bashibleContext := template.NewContext(factory, secretName, secretKey, cachesManager)

	// Template-based REST API
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(bashible.GroupName, Scheme, metav1.ParameterCodec, Codecs)
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = bashibleregistry.GetStorage(templatesRootDir, bashibleContext, cachesManager)

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

// newBashibleInformerFactory creates informer factory for particular namespace and label selector.
// Bashible apiserver is expected to use single namespace and only related resources.
func newBashibleInformerFactory(resync time.Duration, namespace, labelSelector string) (informers.SharedInformerFactory, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot get in-cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("cannot create kubernetes client: %v", err)
	}

	factory := informers.NewSharedInformerFactoryWithOptions(
		clientset,
		resync,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)

	return factory, nil
}
