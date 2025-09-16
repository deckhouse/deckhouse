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

package server

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"bashible-apiserver/pkg/apis/bashible/v1alpha1"
	"bashible-apiserver/pkg/apiserver"
	"bashible-apiserver/pkg/apiserver/readyz"
	bashibleopenapi "bashible-apiserver/pkg/generated/openapi"
)

// BashibleServerOptions contains state for master/api server
type BashibleServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	StdOut io.Writer
	StdErr io.Writer
}

// NewBashibleServerOptions returns a new BashibleServerOptions
func NewBashibleServerOptions(out, errOut io.Writer) *BashibleServerOptions {
	o := &BashibleServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			"",
			apiserver.Codecs.LegacyCodec(v1alpha1.SchemeGroupVersion),
		),
		StdOut: out,
		StdErr: errOut,
	}
	o.RecommendedOptions.Etcd = nil
	o.RecommendedOptions.Features.EnableProfiling = true

	return o
}

// NewCommandStartBashibleServer provides a CLI handler for 'start master' command
// with a default BashibleServerOptions.
func NewCommandStartBashibleServer(defaults *BashibleServerOptions, stopCh <-chan struct{}) *cobra.Command {
	o := *defaults
	cmd := &cobra.Command{
		Short: "Launch a bashible API server",
		Long:  "Launch a bashible API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunBashibleServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)

	return cmd
}

// Validate validates BashibleServerOptions
func (o BashibleServerOptions) Validate(args []string) error {
	errors := make([]error, 0)
	errors = append(errors, o.RecommendedOptions.Validate()...)
	return utilerrors.NewAggregate(errors)
}

// Complete fills in fields required to have valid data
func (o *BashibleServerOptions) Complete() error {
	return nil
}

// Config returns config for the api server given BashibleServerOptions
func (o *BashibleServerOptions) Config(stopCh <-chan struct{}) (*apiserver.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(apiserver.Codecs)

	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIConfig(
		bashibleopenapi.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(apiserver.Scheme))
	serverConfig.OpenAPIV3Config.Info.Title = "Bashible"
	serverConfig.OpenAPIV3Config.Info.Version = "0.1"
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(
		bashibleopenapi.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(apiserver.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "Bashible"
	serverConfig.OpenAPIConfig.Info.Version = "0.1"
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	deployInformer := serverConfig.SharedInformerFactory.Apps().V1().Deployments().Informer()
	deployHealthChecker, err := readyz.NewDeploymentReadinessCheck(stopCh, deployInformer, "d8-system", "deckhouse")
	if err != nil {
		return nil, fmt.Errorf("readyz.NewDeploymentReadinessCheck: %w", err)
	}
	serverConfig.ReadyzChecks = append(serverConfig.ReadyzChecks, deployHealthChecker)

	ctrlManager, err := apiserver.NewCtrlManager()
	if err != nil {
		return nil, fmt.Errorf("error creating ctr manager: %w", err)
	}

	config := &apiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: apiserver.ExtraConfig{
			CtrlManager: ctrlManager,
		},
	}
	return config, nil
}

// RunBashibleServer starts a new BashibleServer given BashibleServerOptions
func (o BashibleServerOptions) RunBashibleServer(stopCh <-chan struct{}) error {
	config, err := o.Config(stopCh)
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	server.GenericAPIServer.AddPostStartHookOrDie(
		"start-bashible-apiserver-informers",
		func(context genericapiserver.PostStartHookContext) error {
			config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
			return nil
		},
	)

	// make context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-stopCh
		cancel()
	}()

	// run ApiServer and CtrManager
	g, errCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return server.CtrlManager.Start(errCtx)
	})
	g.Go(func() error {
		return server.GenericAPIServer.PrepareRun().Run(errCtx.Done())
	})
	return g.Wait()
}
