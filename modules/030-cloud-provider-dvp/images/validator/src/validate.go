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

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
)

func validate(ctx context.Context, input proto.PrepareInput) error {
	if err := validateKubeconfig(ctx, input); err != nil {
		return err
	}
	return validateCredentialsSecret(input)
}

func prepare(_ context.Context, input proto.PrepareInput) (*proto.PrepareResult, error) {
	cv, err := proto.ParseResourcesYAML(input.ResourcesYAML)
	if err != nil {
		return nil, fmt.Errorf("parse resources: %w", err)
	}
	cv.Settings = input.ModuleConfig
	return &proto.PrepareResult{
		Vars:                  cv,
		ProviderClusterConfig: input.ProviderClusterConfig,
	}, nil
}

func validateKubeconfig(ctx context.Context, input proto.PrepareInput) error {
	if input.Operation != proto.OperationBootstrap {
		return nil
	}
	if len(input.ProviderClusterConfig) == 0 {
		return nil
	}

	providerRaw, ok := input.ProviderClusterConfig["provider"]
	if !ok {
		return fmt.Errorf("provider.kubeconfigDataBase64 must be set")
	}

	var spec struct {
		KubeconfigDataBase64 string `json:"kubeconfigDataBase64"`
	}

	providerMap, ok := providerRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("provider cluster configuration: provider field has unexpected type")
	}
	kubeconfigB64, _ := providerMap["kubeconfigDataBase64"].(string)
	spec.KubeconfigDataBase64 = kubeconfigB64

	if spec.KubeconfigDataBase64 == "" {
		return fmt.Errorf("provider.kubeconfigDataBase64 must be set")
	}

	kubeconfigBytes, err := base64.StdEncoding.DecodeString(spec.KubeconfigDataBase64)
	if err != nil {
		return fmt.Errorf("decode provider.kubeconfigDataBase64: %w", err)
	}

	cfg, err := clientcmd.Load(kubeconfigBytes)
	if err != nil {
		return fmt.Errorf("parse provider.kubeconfigDataBase64 as kubeconfig: %w", err)
	}

	restCfg, err := clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return fmt.Errorf("build rest config from provider.kubeconfigDataBase64: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("create kubernetes client from provider.kubeconfigDataBase64: %w", err)
	}

	review := &authv1.SelfSubjectReview{}
	response, err := clientset.AuthenticationV1().SelfSubjectReviews().Create(ctx, review, metav1.CreateOptions{})
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

func validateCredentialsSecret(input proto.PrepareInput) error {
	cv, err := proto.ParseResourcesYAML(input.ResourcesYAML)
	if err != nil {
		return fmt.Errorf("parse resources: %w", err)
	}

	if len(cv.Secrets) == 0 {
		return fmt.Errorf(
			"DVP cloud provider config validation error: no credential Secret found\n" +
				"Hint: Check your config file: a Secret with provider credentials is required.",
		)
	}

	return nil
}
