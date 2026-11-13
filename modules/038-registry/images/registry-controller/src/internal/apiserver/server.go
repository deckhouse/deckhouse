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

// Package apiserver runs the aggregated API server of registry.deckhouse.io/v1alpha1: resources that
// are assembled on every request and never stored. For now RegistryImage, the images of the cache.
//
// Built the way node-controller builds templates.internal.deckhouse.io: delegated authentication and
// authorization against the cluster's API server, so that RBAC and the audit log apply as to any
// resource, on every replica of the controller rather than the leader alone, since the Service the
// APIService points at balances over all of them.
package apiserver

import (
	"context"
	"fmt"
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

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"
)

const serverName = "registry-controller-apiserver"

// servingHookName names the post-start hook that reports the server answering.
const servingHookName = "registry-controller-serving"

var (
	// Scheme holds the types served by the aggregated API server.
	Scheme = runtime.NewScheme()
	// Codecs serializes and deserializes the served types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

func init() {
	utilruntime.Must(registryimagev1alpha1.AddToScheme(Scheme))
	metav1.AddToGroupVersion(Scheme, registryimagev1alpha1.GroupVersion)

	// The generic list handler passes every field selector through the scheme before the storage
	// sees it, and the default conversion knows metadata.name and metadata.namespace alone: without
	// this, a selector on state or inSet is refused before List can honour it.
	utilruntime.Must(Scheme.AddFieldLabelConversionFunc(
		registryimagev1alpha1.GroupVersion.WithKind(registryimagev1alpha1.RegistryImageKind),
		convertFieldLabel,
	))

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
	// BindPort is the port the server listens on. The APIService points at it through the
	// registry-controller Service.
	BindPort int
	// CertFile and KeyFile are the PEM paths of the serving certificate.
	CertFile string
	KeyFile  string
	// Images is what RegistryImage is served from.
	Images rest.Storage
	// Serving is closed once the server answers on BindPort. The pod's readiness reads it: the
	// APIService routes to every Endpoint, and a replica still waiting for the kube-apiserver fails
	// discovery of the group.
	Serving chan<- struct{}
}

// configRetryInterval is how long a failed startup lookup waits before the next attempt.
var configRetryInterval = 5 * time.Second

// Run starts the aggregated API server and blocks until ctx is done.
func Run(ctx context.Context, opts Options) error {
	cfg, err := newConfig(ctx, opts)
	if err != nil {
		return fmt.Errorf("build apiserver config: %w", err)
	}

	srv, err := newServer(cfg, opts.Images)
	if err != nil {
		return fmt.Errorf("create apiserver: %w", err)
	}

	// A post-start hook, not a point in this function: the listener is bound long before it is
	// served, so only the hook means "answering".
	if opts.Serving != nil {
		if err := srv.AddPostStartHook(servingHookName, func(genericapiserver.PostStartHookContext) error {
			close(opts.Serving)
			return nil
		}); err != nil {
			return fmt.Errorf("register the serving hook: %w", err)
		}
	}

	return srv.PrepareRun().RunWithContext(ctx)
}

// newConfig builds a serving config with delegated authentication and authorization against the
// cluster's kube-apiserver. Both look their configuration up in the cluster once, and the
// kube-apiserver may not answer yet while the pod starts, so a failed attempt is retried until ctx is
// done.
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
	// ApplyTo keeps the listener it opens on serving, so the next attempt takes that one over instead
	// of asking for the port again.
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

// newServer builds the aggregated server. Both OpenAPI configs are required: InstallAPIGroup refuses
// without the v3 one, and kube-aggregator downloads /openapi/v2 from every APIService — a 404 there
// leaves the kube-apiserver re-queueing the download forever.
func newServer(cfg *genericapiserver.RecommendedConfig, images rest.Storage) (*genericapiserver.GenericAPIServer, error) {
	namer := endpointsopenapi.NewDefinitionNamer(Scheme)
	cfg.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openAPIDefinitions(), namer)
	cfg.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(openAPIDefinitions(), namer)

	srv, err := cfg.Complete().New(serverName, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, fmt.Errorf("create generic apiserver: %w", err)
	}

	info := genericapiserver.NewDefaultAPIGroupInfo(registryimagev1alpha1.GroupName, Scheme, metav1.ParameterCodec, Codecs)
	info.VersionedResourcesStorageMap[registryimagev1alpha1.GroupVersion.Version] = map[string]rest.Storage{
		registryimagev1alpha1.RegistryImageResource: images,
	}
	if err := srv.InstallAPIGroup(&info); err != nil {
		return nil, fmt.Errorf("install %s: %w", registryimagev1alpha1.GroupVersion, err)
	}
	return srv, nil
}
