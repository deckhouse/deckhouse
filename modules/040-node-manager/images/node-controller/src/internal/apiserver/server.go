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
// (not persisted) resources of templates.internal.deckhouse.io/v1alpha1.
package apiserver

import (
	"context"
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	endpointsopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilcompatibility "k8s.io/apiserver/pkg/util/compatibility"
	"k8s.io/klog/v2"
	openapicommon "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/util"
	"k8s.io/kube-openapi/pkg/validation/spec"

	templatesv1alpha1 "github.com/deckhouse/node-controller/api/templates.internal.deckhouse.io/v1alpha1"
)

const serverName = "node-controller-apiserver"

var (
	// Scheme holds the types served by the aggregated API server.
	Scheme = runtime.NewScheme()
	// Codecs serializes and deserializes the served types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	utilruntime.Must(templatesv1alpha1.AddToScheme(Scheme))
	metav1.AddToGroupVersion(Scheme, templatesv1alpha1.GroupVersion)

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
	// Storage is the REST implementation the resource is served from.
	Storage rest.Storage
}

// configRetryInterval is how long a failed startup lookup waits before the next
// attempt. A variable so the test does not have to sit through it.
var configRetryInterval = 5 * time.Second

// Run starts the aggregated API server for templates.internal.deckhouse.io/v1alpha1 and
// blocks until ctx is done.
func Run(ctx context.Context, opts Options) error {
	cfg, err := newConfig(ctx, opts)
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
// authorization against the cluster kube-apiserver. Both look their
// configuration up in the cluster once, and the kube-apiserver is not answering
// yet while the pod starts, so a failed attempt is retried until ctx is done.
func newConfig(ctx context.Context, opts Options) (*genericapiserver.RecommendedConfig, error) {
	serving := genericoptions.NewSecureServingOptions().WithLoopback()
	serving.BindPort = opts.BindPort
	serving.ServerCert.CertKey = genericoptions.CertKey{
		CertFile: opts.CertFile,
		KeyFile:  opts.KeyFile,
	}

	var (
		cfg     *genericapiserver.RecommendedConfig
		lastErr error
	)
	// ApplyTo keeps the listener it opens on serving, so the next attempt takes
	// that one over instead of asking for the port again.
	err := wait.PollUntilContextCancel(ctx, configRetryInterval, true, func(context.Context) (bool, error) {
		cfg, lastErr = applyOptions(serving)
		if lastErr != nil {
			klog.ErrorS(lastErr, "the aggregated API server cannot build its config yet; retrying")
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, err
	}
	return cfg, nil
}

func applyOptions(serving *genericoptions.SecureServingOptionsWithLoopback) (*genericapiserver.RecommendedConfig, error) {
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

// newServer builds the aggregated server. The v3 OpenAPI config is required —
// InstallAPIGroup refuses without one — and it is built per group from the
// definitions below. The v2 config is deliberately left nil: PrepareRun builds a
// single spec covering every route the generic server carries, /version and
// /apis included, and calls klog.Fatal on the first model it has no definition
// for. Filling that in means carrying the whole generated apimachinery set for a
// pair of virtual resources nobody runs `kubectl explain` against.
func newServer(cfg *genericapiserver.RecommendedConfig, storage rest.Storage) (*genericapiserver.GenericAPIServer, error) {
	cfg.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(
		openAPIDefinitions(), endpointsopenapi.NewDefinitionNamer(Scheme))

	srv, err := cfg.Complete().New(serverName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, fmt.Errorf("create generic apiserver: %w", err)
	}

	info := genericapiserver.NewDefaultAPIGroupInfo(templatesv1alpha1.GroupVersion.Group, Scheme, metav1.ParameterCodec, Codecs)
	info.VersionedResourcesStorageMap[templatesv1alpha1.GroupVersion.Version] = map[string]rest.Storage{templatesv1alpha1.NodeConfigTemplateResource: storage}

	if err := srv.InstallAPIGroup(&info); err != nil {
		return nil, fmt.Errorf("install %s: %w", templatesv1alpha1.GroupVersion, err)
	}

	return srv, nil
}

// openAPIDefinitions describes every type this server can serve as a free-form
// object: the resources are virtual and have no generated openapi definitions.
// The list, the Status and the discovery types are served too — a route whose
// model has no definition leaves the whole group's spec empty.
func openAPIDefinitions() openapicommon.GetOpenAPIDefinitions {
	freeForm := openapicommon.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{Type: spec.StringOrArray{"object"}},
			VendorExtensible: spec.VendorExtensible{
				Extensions: spec.Extensions{"x-kubernetes-preserve-unknown-fields": true},
			},
		},
	}

	definitions := map[string]openapicommon.OpenAPIDefinition{}
	for _, known := range Scheme.AllKnownTypes() {
		definitions[util.GetCanonicalTypeName(reflect.New(known).Interface())] = freeForm
	}

	return func(openapicommon.ReferenceCallback) map[string]openapicommon.OpenAPIDefinition {
		return definitions
	}
}
