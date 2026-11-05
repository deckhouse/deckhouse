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

package toy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/component-base/compatibility"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	"github.com/deckhouse/deckhouse/pkg/api-extension-sdk/scope"
)

// Options configure the toy server.
type Options struct {
	SecureServing  *genericoptions.SecureServingOptionsWithLoopback
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions

	// Seed lists namespaces to create gizmo-<ns> in at start.
	Seed []string

	// Authorizer replaces the delegated authorizer when set (tests only).
	// It is wrapped with scope.WithAuthorizer like the delegated one.
	Authorizer authorizer.Authorizer
}

func NewOptions() *Options {
	o := &Options{
		SecureServing:  genericoptions.NewSecureServingOptions().WithLoopback(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
	}
	o.SecureServing.BindPort = 8443
	o.SecureServing.ServerCert.CertDirectory = "/tmp/scope-toy-certs"
	o.SecureServing.ServerCert.PairName = "apiserver"
	return o
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	o.SecureServing.AddFlags(fs)
	o.Authentication.AddFlags(fs)
	o.Authorization.AddFlags(fs)
	fs.StringSliceVar(&o.Seed, "seed", o.Seed, "Namespaces to create gizmo-<namespace> in at start. Also read from $SCOPE_TOY_SEED.")
}

// Server is a configured toy server ready to run.
type Server struct {
	Generic *genericapiserver.GenericAPIServer
	Storage *Storage
}

// New builds the server. The scope wiring is the part an extension would copy:
//
//  1. BuildHandlerChainFunc puts scope.WithTrusted outside the default chain,
//     with the requestheader configuration the delegated authentication loaded.
//  2. The authorizer is wrapped with scope.WithAuthorizer for the opted-in
//     resources.
//  3. The storage filters List, Get and Watch by the decision.
func (o *Options) New() (*Server, error) {
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", []string{"scope-toy.scope-toy.svc"}, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("self-signed certificates: %w", err)
	}

	cfg := genericapiserver.NewConfig(Codecs)
	cfg.EffectiveVersion = compatibility.NewEffectiveVersionFromString("1.35", "", "")
	namer := openapi.NewDefinitionNamer(Scheme)
	cfg.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(GetOpenAPIDefinitions, namer)
	cfg.OpenAPIConfig.Info.Title, cfg.OpenAPIConfig.Info.Version = "scopetoy", "v1"
	cfg.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(GetOpenAPIDefinitions, namer)
	cfg.OpenAPIV3Config.Info.Title, cfg.OpenAPIV3Config.Info.Version = "scopetoy", "v1"

	if err := o.SecureServing.ApplyTo(&cfg.SecureServing, &cfg.LoopbackClientConfig); err != nil {
		return nil, err
	}
	if err := o.Authentication.ApplyTo(&cfg.Authentication, cfg.SecureServing, cfg.OpenAPIConfig); err != nil {
		return nil, err
	}
	if o.Authorizer != nil {
		cfg.Authorization.Authorizer = o.Authorizer
	} else if err := o.Authorization.ApplyTo(&cfg.Authorization); err != nil {
		return nil, err
	}

	cfg.Authorization.Authorizer = logAuthorizer(scope.WithAuthorizer(cfg.Authorization.Authorizer, GizmoResource))
	cfg.BuildHandlerChainFunc = func(h http.Handler, c *genericapiserver.Config) http.Handler {
		return scope.WithTrusted(genericapiserver.DefaultBuildHandlerChain(h, c), c.Authentication.RequestHeaderConfig)
	}
	if cfg.Authentication.RequestHeaderConfig == nil {
		klog.Warning("no requestheader configuration: scope headers will never be trusted")
	}

	generic, err := cfg.Complete(nil).New("scope-toy", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	storage := NewStorage()
	if err := storage.Seed(o.Seed); err != nil {
		return nil, err
	}
	group := genericapiserver.NewDefaultAPIGroupInfo(GroupName, Scheme, ParameterCodec, Codecs)
	group.VersionedResourcesStorageMap[SchemeGroupVersion.Version] = map[string]rest.Storage{GizmoResource.Resource: storage}
	if err := generic.InstallAPIGroup(&group); err != nil {
		return nil, err
	}
	return &Server{Generic: generic, Storage: storage}, nil
}

// Run serves until ctx is done.
func (s *Server) Run(ctx context.Context) error {
	return s.Generic.PrepareRun().RunWithContext(ctx)
}

// logAuthorizer logs every authorization decision, so a denied scoped read is
// visible in the log too (the storage logs only the reads it serves).
func logAuthorizer(a authorizer.Authorizer) authorizer.Authorizer {
	return authorizer.AuthorizerFunc(func(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
		decision, reason, err := a.Authorize(ctx, attrs)
		if !attrs.IsResourceRequest() {
			return decision, reason, err
		}
		mode, size := "none", -1
		if d, ok := scope.FromContext(ctx); ok {
			mode, size = string(d.Mode), d.Namespaces.Len()
		}
		userName := ""
		if attrs.GetUser() != nil {
			userName = attrs.GetUser().GetName()
		}
		gr := schema.GroupResource{Group: attrs.GetAPIGroup(), Resource: attrs.GetResource()}
		klog.InfoS("authorize", "user", userName, "verb", attrs.GetVerb(), "resource", gr.String(),
			"subresource", attrs.GetSubresource(), "namespace", attrs.GetNamespace(),
			"scope", mode, "setSize", size, "decision", decisionString(decision), "reason", reason, "err", err)
		return decision, reason, err
	})
}

func decisionString(d authorizer.Decision) string {
	switch d {
	case authorizer.DecisionAllow:
		return "allow"
	case authorizer.DecisionDeny:
		return "deny"
	default:
		return "no-opinion"
	}
}

// ParseSeed splits a comma-separated namespace list.
func ParseSeed(v string) []string {
	var out []string
	for _, ns := range strings.Split(v, ",") {
		if ns = strings.TrimSpace(ns); ns != "" {
			out = append(out, ns)
		}
	}
	return out
}
