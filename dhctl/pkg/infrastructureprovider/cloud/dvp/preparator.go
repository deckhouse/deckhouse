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

	"github.com/name212/govalue"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	dhctljson "github.com/deckhouse/deckhouse/dhctl/pkg/util/json"
)

type MetaConfigPreparator struct {
	validateKubeConfig bool
	validateKubeAPI    bool
	logger             log.Logger
	additionalData     *PreparatorAdditionalData
}

func NewMetaConfigPreparator() *MetaConfigPreparator {
	return &MetaConfigPreparator{
		logger: log.NewSilentLogger(),
	}
}

func (p *MetaConfigPreparator) WithLogger(logger log.Logger) *MetaConfigPreparator {
	if !govalue.IsNil(logger) {
		p.logger = logger
	}

	return p
}

func (p *MetaConfigPreparator) WithAdditionalData(data *PreparatorAdditionalData) *MetaConfigPreparator {
	p.additionalData = data

	return p
}

func (p *MetaConfigPreparator) EnableValidateKubeConfig(validateKubeAPI bool) *MetaConfigPreparator {
	p.validateKubeAPI = validateKubeAPI
	p.validateKubeConfig = true
	return p
}

func (p *MetaConfigPreparator) Validate(ctx context.Context, metaConfig *config.MetaConfig) error {
	if !p.validateKubeConfig {
		return nil
	}

	client, err := p.KubeconfigDataBase64(metaConfig)
	if err != nil {
		return err
	}

	if !p.validateKubeAPI {
		return nil
	}

	return p.whoAmI(ctx, client)
}

func (p *MetaConfigPreparator) Prepare(_ context.Context, metaConfig *config.MetaConfig) error {
	if govalue.IsNil(p.additionalData) {
		p.logSkipPrepare("Additional data for cloud provider dvp not provided")
		return nil
	}

	return p.prepareSSHPublicKey(metaConfig, p.additionalData)
}

func (p *MetaConfigPreparator) KubeconfigDataBase64(metaConfig *config.MetaConfig) (*kubernetes.Clientset, error) {
	spec, err := dhctljson.UnmarshalToFromMessageMap[DVPProviderSpec](metaConfig.ProviderClusterConfig, "provider")
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

func (p *MetaConfigPreparator) whoAmI(ctx context.Context, client *kubernetes.Clientset) error {
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

func (p *MetaConfigPreparator) logSkipPrepare(msg string, args ...any) {
	m := fmt.Sprintf(msg, args...)
	p.logger.LogDebugF("%s. Skip prepare\n", m)
}

func (p *MetaConfigPreparator) prepareSSHPublicKey(metaConfig *config.MetaConfig, data *PreparatorAdditionalData) error {
	metaConfigSSHPubKey, err := p.getSSHPublicKey(metaConfig)
	if err != nil {
		return err
	}

	const processSuffix = "\n"

	if strings.HasSuffix(metaConfigSSHPubKey, processSuffix) {
		p.logSkipPrepare("Meta config ssh pub key already contains new line")
		return nil
	}

	if metaConfigSSHPubKey == "" {
		// log in getSSHPublicKey
		return nil
	}

	originalSSHPubKey, err := data.extractSSHPubKey(p.logger)
	if err != nil {
		return err
	}

	if metaConfigSSHPubKey == "" {
		// log in extractSSHPubKey
		return nil
	}

	if originalSSHPubKey == metaConfigSSHPubKey {
		p.logSkipPrepare("Meta config ssh pub key equals to original ssh pub key")
		return nil
	}

	originalSSHPubKeyTrimmed := strings.TrimSuffix(originalSSHPubKey, processSuffix)

	if originalSSHPubKeyTrimmed != metaConfigSSHPubKey {
		p.logSkipPrepare("Original trimmed ssh key not equal to meta config ssh pub key. Probably new key: %s", originalSSHPubKey)
		return nil
	}

	return p.setSSHPublicKey(metaConfig, originalSSHPubKey)
}

const sshPublicKeyConfigKey = "sshPublicKey"

func (p *MetaConfigPreparator) getSSHPublicKey(metaConfig *config.MetaConfig) (string, error) {
	providerConfig := metaConfig.ProviderClusterConfig
	if len(providerConfig) == 0 {
		p.logSkipPrepare("Provider cluster config not provided")
		return "", nil
	}

	sshPubKeyJSON, ok := providerConfig[sshPublicKeyConfigKey]
	if !ok {
		p.logSkipPrepare("%s is not provided", sshPublicKeyConfigKey)
		return "", nil
	}

	var sshPubKey string
	err := json.Unmarshal(sshPubKeyJSON, &sshPubKey)
	if err != nil {
		return "", fmt.Errorf("Cannot unmarshal ssh pub key for getting from meta config: %w", err)
	}

	return sshPubKey, nil
}

func (p *MetaConfigPreparator) setSSHPublicKey(metaConfig *config.MetaConfig, key string) error {
	keyForStore, err := json.Marshal(key)
	if err != nil {
		return fmt.Errorf("Cannot marshal ssh pub key for set to provider cluster config after prepare")
	}

	metaConfig.ProviderClusterConfig[sshPublicKeyConfigKey] = json.RawMessage(keyForStore)

	return nil
}
