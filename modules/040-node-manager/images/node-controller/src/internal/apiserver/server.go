/*
Copyright 2026 Flant JSC

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

// Package apiserver runs the aggregated API server that serves the virtual
// (not persisted) resources of internal.deckhouse.io/v1alpha1.
package apiserver

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	endpointsopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilcompatibility "k8s.io/apiserver/pkg/util/compatibility"
	openapicommon "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/util"
	"k8s.io/kube-openapi/pkg/validation/spec"

	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
)

const serverName = "node-controller-apiserver"

var (
	// Scheme holds the types served by the aggregated API server.
	Scheme = runtime.NewScheme()
	// Codecs serializes and deserializes the served types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	utilruntime.Must(internalv1alpha1.AddToScheme(Scheme))
	metav1.AddToGroupVersion(Scheme, internalv1alpha1.GroupVersion)

	// The generic server negotiates request options and Status in the unversioned "v1".
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	metav1.AddToGroupVersion(Scheme, unversioned)
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

// Options configures the aggregated API server.
type Options struct {
	// BindPort is the port the aggregated API server listens on. The APIService
	// points at this port of the node-controller Service.
	BindPort int
	// CertFile and KeyFile are the PEM paths of the serving certificate.
	CertFile string
	KeyFile  string
	// Storage maps a plural resource name to its REST implementation.
	Storage map[string]rest.Storage
}

// Run starts the aggregated API server for internal.deckhouse.io/v1alpha1 and
// blocks until ctx is done.
func Run(ctx context.Context, opts Options) error {
	if len(opts.Storage) == 0 {
		return fmt.Errorf("no storage registered for %s", internalv1alpha1.GroupVersion)
	}

	cfg, err := newConfig(opts)
	if err != nil {
		return fmt.Errorf("build apiserver config: %w", err)
	}

	srv, err := newServer(cfg, opts.Storage)
	if err != nil {
		return fmt.Errorf("create apiserver: %w", err)
	}

	return srv.PrepareRun().RunWithContext(ctx)
}

// newConfig builds a serving config with delegated authentication and
// authorization against the cluster kube-apiserver.
func newConfig(opts Options) (*genericapiserver.RecommendedConfig, error) {
	serving := genericoptions.NewSecureServingOptions().WithLoopback()
	serving.BindPort = opts.BindPort
	serving.ServerCert.CertKey = genericoptions.CertKey{
		CertFile: opts.CertFile,
		KeyFile:  opts.KeyFile,
	}

	cfg := genericapiserver.NewRecommendedConfig(Codecs)
	cfg.EffectiveVersion = utilcompatibility.DefaultBuildEffectiveVersion()

	if err := serving.ApplyTo(&cfg.SecureServing, &cfg.LoopbackClientConfig); err != nil {
		return nil, fmt.Errorf("apply serving options: %w", err)
	}
	if err := genericoptions.NewDelegatingAuthenticationOptions().ApplyTo(&cfg.Authentication, cfg.SecureServing, cfg.OpenAPIConfig); err != nil {
		return nil, fmt.Errorf("apply authentication options: %w", err)
	}
	if err := genericoptions.NewDelegatingAuthorizationOptions().ApplyTo(&cfg.Authorization); err != nil {
		return nil, fmt.Errorf("apply authorization options: %w", err)
	}

	return cfg, nil
}

func newServer(cfg *genericapiserver.RecommendedConfig, storage map[string]rest.Storage) (*genericapiserver.GenericAPIServer, error) {
	definitions := openAPIDefinitions(storage)
	namer := endpointsopenapi.NewDefinitionNamer(Scheme)
	cfg.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(definitions, namer)
	cfg.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(definitions, namer)

	srv, err := cfg.Complete().New(serverName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, fmt.Errorf("create generic apiserver: %w", err)
	}

	info := genericapiserver.NewDefaultAPIGroupInfo(internalv1alpha1.GroupVersion.Group, Scheme, metav1.ParameterCodec, Codecs)
	info.VersionedResourcesStorageMap[internalv1alpha1.GroupVersion.Version] = storage

	if err := srv.InstallAPIGroup(&info); err != nil {
		return nil, fmt.Errorf("install %s: %w", internalv1alpha1.GroupVersion, err)
	}

	return srv, nil
}

// openAPIDefinitions describes every served type as a free-form object: the
// resources are virtual and have no generated openapi definitions. Running
// openapi-gen over the api package would give kubectl explain a real schema.
func openAPIDefinitions(storage map[string]rest.Storage) openapicommon.GetOpenAPIDefinitions {
	definitions := make(map[string]openapicommon.OpenAPIDefinition, len(storage))
	for _, s := range storage {
		definitions[util.GetCanonicalTypeName(s.New())] = openapicommon.OpenAPIDefinition{
			Schema: spec.Schema{
				SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"object"}},
				VendorExtensible: spec.VendorExtensible{
					Extensions: spec.Extensions{"x-kubernetes-preserve-unknown-fields": true},
				},
			},
		}
	}

	return func(openapicommon.ReferenceCallback) map[string]openapicommon.OpenAPIDefinition {
		return definitions
	}
}
