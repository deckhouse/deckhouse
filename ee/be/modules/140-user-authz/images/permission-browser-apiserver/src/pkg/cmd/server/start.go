/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package server

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	basecompatibility "k8s.io/component-base/compatibility"
	baseversion "k8s.io/component-base/version"

	"permission-browser-apiserver/pkg/apis/authorization/v1alpha1"
	"permission-browser-apiserver/pkg/apiserver"
	generatedopenapi "permission-browser-apiserver/pkg/generated/openapi"
)

// PermissionBrowserServerOptions contains state for master/api server
type PermissionBrowserServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions
	ConfigPath         string

	StdOut io.Writer
	StdErr io.Writer
}

// NewPermissionBrowserServerOptions returns a new PermissionBrowserServerOptions
func NewPermissionBrowserServerOptions(out, errOut io.Writer) *PermissionBrowserServerOptions {
	o := &PermissionBrowserServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			"",
			apiserver.Codecs.LegacyCodec(v1alpha1.SchemeGroupVersion),
		),
		ConfigPath: "/etc/user-authz-webhook/config.json",
		StdOut:     out,
		StdErr:     errOut,
	}
	// No etcd - ephemeral resources only
	o.RecommendedOptions.Etcd = nil
	o.RecommendedOptions.Features.EnableProfiling = true

	return o
}

// NewCommandStartPermissionBrowserServer provides a CLI handler for 'start' command
func NewCommandStartPermissionBrowserServer(defaults *PermissionBrowserServerOptions, stopCh <-chan struct{}) *cobra.Command {
	o := *defaults
	cmd := &cobra.Command{
		Short: "Launch a permission browser API server",
		Long:  "Launch a permission browser API server for bulk subject access reviews",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunPermissionBrowserServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)
	flags.StringVar(&o.ConfigPath, "user-authz-config", o.ConfigPath, "Path to the user-authz webhook configuration file")

	return cmd
}

// Validate validates PermissionBrowserServerOptions
func (o PermissionBrowserServerOptions) Validate(args []string) error {
	errors := make([]error, 0)
	errors = append(errors, o.RecommendedOptions.Validate()...)
	return utilerrors.NewAggregate(errors)
}

// Complete fills in fields required to have valid data and applies defaults.
// This implements the standard Kubernetes Complete -> Validate -> Run pattern.
func (o *PermissionBrowserServerOptions) Complete() error {
	// Set default config path if not provided
	if o.ConfigPath == "" {
		o.ConfigPath = "/etc/user-authz-webhook/config.json"
	}
	return nil
}

// Config returns config for the api server
func (o *PermissionBrowserServerOptions) Config(stopCh <-chan struct{}) (*apiserver.Config, error) {
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(apiserver.Codecs)

	// Set EffectiveVersion explicitly for k8s.io/apiserver v0.34.x Config.Complete() expectations.
	// Use current binary version as the default emulation/min-compat versions.
	{
		bin := baseversion.Get().String()
		serverConfig.EffectiveVersion = basecompatibility.NewEffectiveVersionFromString(bin, "", "")
	}

	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(
		generatedopenapi.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(apiserver.Scheme))
	serverConfig.OpenAPIV3Config.Info.Title = "PermissionBrowser"
	serverConfig.OpenAPIV3Config.Info.Version = "0.1"

	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		generatedopenapi.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(apiserver.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "PermissionBrowser"
	serverConfig.OpenAPIConfig.Info.Version = "0.1"

	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	config := &apiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: apiserver.ExtraConfig{
			ConfigPath: o.ConfigPath,
		},
	}
	return config, nil
}

// RunPermissionBrowserServer starts a new PermissionBrowserServer
func (o PermissionBrowserServerOptions) RunPermissionBrowserServer(stopCh <-chan struct{}) error {
	config, err := o.Config(stopCh)
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	server.GenericAPIServer.AddPostStartHookOrDie(
		"start-permission-browser-apiserver-informers",
		func(hookCtx genericapiserver.PostStartHookContext) error {
			config.GenericConfig.SharedInformerFactory.Start(hookCtx.Done())
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-stopCh
		cancel()
	}()

	return server.GenericAPIServer.PrepareRun().Run(ctx.Done())
}
