// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dvp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	otattribute "go.opentelemetry.io/otel/attribute"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/providerdata"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

type PreparatorOptions struct {
	// ValidateKubeAPI enables full kubeconfig validation including an API call.
	// Only meaningful when operation is bootstrap.
	ValidateKubeAPI bool
}

type Preparator struct {
	operation string
	opts      PreparatorOptions
}

func NewPreparator(operation string, opts PreparatorOptions) *Preparator {
	return &Preparator{operation: operation, opts: opts}
}

func (p *Preparator) Validate(ctx context.Context, input config.ProviderInput) error {
	ctx, span := telemetry.StartSpan(ctx, "dvp.Validate")
	defer span.End()

	var secretsCount, nodeGroupsCount, instanceClassesCount int
	if input.CloudProviderVars != nil {
		secretsCount = len(input.CloudProviderVars.Secrets)
		nodeGroupsCount = len(input.CloudProviderVars.NodeGroups)
		instanceClassesCount = len(input.CloudProviderVars.InstanceClasses)
	}
	span.SetAttributes(
		otattribute.String("provider.name", input.ProviderName),
		otattribute.String("provider.operation", p.operation),
		otattribute.Bool("provider.cloudProviderVarsPresent", input.CloudProviderVars != nil),
		otattribute.Int("provider.secretsCount", secretsCount),
		otattribute.Int("provider.nodeGroupsCount", nodeGroupsCount),
		otattribute.Int("provider.instanceClassesCount", instanceClassesCount),
	)

	if err := p.validateLegacyClusterConfiguration(ctx, input); err != nil {
		return err
	}
	span.AddEvent("kubeconfig validated")

	return p.validateCloudProviderResources(input)
}

func (p *Preparator) Prepare(_ context.Context, input config.ProviderInput) (providerdata.PrepareResult, error) {
	return providerdata.PrepareResult{Vars: input.CloudProviderVars}, nil
}

func (p *Preparator) validateLegacyClusterConfiguration(ctx context.Context, input config.ProviderInput) error {
	if p.operation != providerdata.OperationBootstrap {
		return nil
	}

	// DVP can be configured via ModuleConfig/cloud-provider-dvp without a
	// DVPClusterConfiguration section. In that case there is no kubeconfig to
	// validate here — the module is responsible for sourcing it from settings.
	if len(input.ProviderClusterConfig) == 0 {
		return nil
	}

	client, err := p.buildKubeClient(input)
	if err != nil {
		return err
	}

	if p.opts.ValidateKubeAPI {
		return p.whoAmI(ctx, client)
	}

	return nil
}

func (p *Preparator) validateCloudProviderResources(input config.ProviderInput) error {
	if input.CloudProviderVars == nil || len(input.CloudProviderVars.Secrets) == 0 {
		return fmt.Errorf("DVP cloud provider config validation error: no credential Secret found\n" +
			"Hint: Check your config file: a Secret with provider credentials is required.")
	}
	return nil
}

func (p *Preparator) buildKubeClient(input config.ProviderInput) (*kubernetes.Clientset, error) {
	raw, ok := input.ProviderClusterConfig["provider"]
	if !ok {
		return nil, fmt.Errorf("provider.kubeconfigDataBase64 must be set")
	}

	var spec DVPProviderSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal provider from provider cluster configuration: %w", err)
	}

	if spec.KubeconfigDataBase64 == "" {
		return nil, fmt.Errorf("provider.kubeconfigDataBase64 must be set")
	}

	kubeconfigBytes, err := base64.StdEncoding.DecodeString(spec.KubeconfigDataBase64)
	if err != nil {
		return nil, fmt.Errorf("decode provider.kubeconfigDataBase64: %w", err)
	}

	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("parse provider.kubeconfigDataBase64 as kubeconfig: %w", err)
	}

	restCfg, err := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config from provider.kubeconfigDataBase64: %w", err)
	}

	log.DebugF("dvp kubeconfig host: %s, clusters: %d, contexts: %d\n",
		restCfg.Host, len(cfg.Clusters), len(cfg.Contexts))

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client from provider.kubeconfigDataBase64: %w", err)
	}

	return clientset, nil
}

func (p *Preparator) whoAmI(ctx context.Context, client *kubernetes.Clientset) error {
	review := &authv1.SelfSubjectReview{}
	response, err := client.AuthenticationV1().SelfSubjectReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf(
			"connect to cluster using provider.kubeconfigDataBase64: %w\n"+
				"Check that the kubeconfig is attached to a service account and does not use 'command' to connect.",
			err,
		)
	}

	if response.Status.UserInfo.Username == "" {
		return fmt.Errorf("self subject review returned empty username")
	}

	if !strings.HasPrefix(response.Status.UserInfo.Username, "system:serviceaccount:") {
		return fmt.Errorf(
			"kubeconfig from provider.kubeconfigDataBase64 must be attached to system:serviceaccounts, but got: %s",
			response.Status.UserInfo.Username,
		)
	}

	return nil
}

