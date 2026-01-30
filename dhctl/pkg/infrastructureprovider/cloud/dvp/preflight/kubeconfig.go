// Copyright 2026 Flant JSC
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

package preflight

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	preflightnew "github.com/deckhouse/deckhouse/dhctl/pkg/preflight_new"
	dhctljson "github.com/deckhouse/deckhouse/dhctl/pkg/util/json"
)

type KubeconfigDeps struct {
	ValidateKubeConfig bool
	ValidateKubeAPI    bool
	MetaConfig         *config.MetaConfig
}

type providerSpec struct {
	KubeconfigDataBase64 string `json:"kubeconfigDataBase64"`
}

type kubeconfigCheck struct {
	deps KubeconfigDeps
}

const kubeconfigCheckName preflightnew.CheckName = "dvp-kubeconfig"

func (kubeconfigCheck) Description() string {
	return "validate kubeconfig and access to kube-apiserver"
}

func (kubeconfigCheck) Phase() preflightnew.Phase {
	return preflightnew.PhaseProviderConfigCheck
}

func (kubeconfigCheck) RetryPolicy() preflightnew.RetryPolicy {
	return preflightnew.DefaultRetryPolicy
}

func (c kubeconfigCheck) Run(ctx context.Context) error {
	if !c.deps.ValidateKubeConfig {
		return nil
	}

	client, err := BuildKubeClient(c.deps.MetaConfig)
	if err != nil {
		return fmt.Errorf("build kube client: %w", err)
	}
	if !c.deps.ValidateKubeAPI {
		return nil
	}

	return WhoAmI(ctx, client)
}

func KubeconfigCheck(deps KubeconfigDeps) preflightnew.Check {
	check := kubeconfigCheck{deps: deps}
	return preflightnew.Check{
		Name:        kubeconfigCheckName,
		Description: check.Description(),
		Phase:       check.Phase(),
		Retry:       check.RetryPolicy(),
		Run:         check.Run,
	}
}

func BuildKubeClient(metaConfig *config.MetaConfig) (*kubernetes.Clientset, error) {
	spec, err := dhctljson.UnmarshalToFromMessageMap[providerSpec](metaConfig.ProviderClusterConfig, "provider")
	if err != nil {
		return nil, fmt.Errorf("Unable to unmarshal provider from provider cluster configuration: %v", err)
	}

	if spec.KubeconfigDataBase64 == "" {
		return nil, fmt.Errorf("provider.kubeconfigDataBase64 must be set")
	}

	kubeconfigBytes, err := base64.StdEncoding.DecodeString(spec.KubeconfigDataBase64)
	if err != nil {
		return nil, fmt.Errorf("Unable to decode provider.kubeconfigDataBase64: %w", err)
	}

	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse provider.kubeconfigDataBase64 as kubeconfig: %w", err)
	}

	restCfg, err := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("Unable to build rest config from provider.kubeconfigDataBase64: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("Unable to create kubernetes client from provider.kubeconfigDataBase64: %w", err)
	}

	return clientset, nil
}

func WhoAmI(ctx context.Context, client *kubernetes.Clientset) error {
	review := &authv1.SelfSubjectReview{}
	response, err := client.AuthenticationV1().SelfSubjectReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf(
			`Failed to connect to cluster using kubeconfig from provider.kubeconfigDataBase64, please verify its contents and fix the error.
Please note that the kubeconfig from provider.kubeconfigDataBase64 must be attached to system:serviceaccounts and should not use 'command' to connect.

=== client-go error ===

%v`,
			err,
		)
	}

	if response.Status.UserInfo.Username == "" {
		return fmt.Errorf("self subject review returned empty username")
	}

	if !strings.HasPrefix(response.Status.UserInfo.Username, "system:serviceaccount:") {
		return fmt.Errorf(
			"kubeconfig from provider.kubeconfigDataBase64 must be attached to system:serviceaccounts, but got: %s", response.Status.UserInfo.Username,
		)
	}

	return nil
}
