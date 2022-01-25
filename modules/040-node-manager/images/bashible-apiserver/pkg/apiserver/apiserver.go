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

package apiserver

import (
	"context"
	"log"
	"time"

	"github.com/flant/kube-client/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

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
	ctx, cancel := context.WithCancel(context.Background())
	genericServer, err := c.GenericConfig.New("bashible-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	err = genericServer.AddPreShutdownHook("cancel-builder-context", func() error {
		cancel()
		return nil
	})
	if err != nil {
		return nil, err
	}

	s := &BashibleServer{
		GenericAPIServer: genericServer,
	}

	// Config hardcode, could be put to `ExtraConfig`
	const (
		templatesRootDir = "/bashible/templates"
		resyncTimeout    = 30 * time.Minute
	)

	kubeClient, err := initializeClientset()
	if err != nil {
		return nil, err
	}

	ngConfigFactory := newNodeGroupConfigurationInformerFactory(kubeClient, resyncTimeout)
	stepsStorage := template.NewStepsStorage(ctx, templatesRootDir, ngConfigFactory)

	cachesManager := bashibleregistry.NewCachesManager()
	secretUpdater := checksumSecretUpdater{client: kubeClient}
	bashibleContext := template.NewContext(ctx, stepsStorage, kubeClient, resyncTimeout, secretUpdater, cachesManager)

	// Template-based REST API
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(bashible.GroupName, Scheme, metav1.ParameterCodec, Codecs)
	apiGroupInfo.VersionedResourcesStorageMap["v1alpha1"] = bashibleregistry.GetStorage(templatesRootDir, bashibleContext, stepsStorage, cachesManager)

	if err := s.GenericAPIServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return nil, err
	}

	return s, nil
}

// newBashibleInformerFactory creates informer factory for particular namespace and label selector.
// Bashible apiserver is expected to use single namespace and only related resources.
func newBashibleInformerFactory(kubeClient kubernetes.Interface, resync time.Duration, namespace, labelSelector string) (informers.SharedInformerFactory, error) {
	factory := informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resync,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
			opts.LabelSelector = labelSelector
		}),
	)

	return factory, nil
}

func newNodeGroupConfigurationInformerFactory(kubeClient client.Client, resync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(
		kubeClient.Dynamic(),
		resync,
	)

	return factory
}
func initializeClientset() (client.Client, error) {
	kcli := client.New()
	err := kcli.Init()

	return kcli, err
}

const (
	configurationsSecretName      = "configuration-checksums"
	configurationsSecretNamespace = "d8-cloud-instance-manager"
)

type checksumSecretUpdater struct {
	client client.Client
}

func (cs checksumSecretUpdater) OnChecksumUpdate(ngmap map[string][]byte) {
	secretStruct := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configurationsSecretName,
			Namespace: configurationsSecretNamespace,
			Labels: map[string]string{
				"app": "bashible-apiserver",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: ngmap,
	}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := cs.client.CoreV1().Secrets(configurationsSecretNamespace).Get(context.Background(), configurationsSecretName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				_, err := cs.client.CoreV1().Secrets(configurationsSecretNamespace).Create(context.Background(), &secretStruct, metav1.CreateOptions{})
				if err != nil {
					log.Printf("create '%s' secret failed: %s", configurationsSecretName, err)
					return err
				}
				return nil
			}

			return err
		}

		_, err = cs.client.CoreV1().Secrets(configurationsSecretNamespace).Update(context.Background(), &secretStruct, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("update '%s' secret failed: %s", configurationsSecretName, err)
			return err
		}

		return nil
	})
	if err != nil {
		klog.Errorf("configuration-checksum upgrade failed: %s", err)
	}
}
