// Copyright 2021 Flant JSC
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

package controlplane

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/settings"
	"github.com/deckhouse/lib-connection/pkg/ssh"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

type KubeProxyChecker struct {
	initParams           *client.KubernetesInitParams
	logCheckResult       bool
	askPassword          bool
	stopProxy            bool
	nodesExternalIPs     map[string]string
	clusterUUID          string
	sshProvider          libcon.SSHProvider
	baseProviderSettings *settings.BaseProviders
}

func NewKubeProxyChecker() *KubeProxyChecker {
	return &KubeProxyChecker{
		stopProxy: true,
	}
}

func (c *KubeProxyChecker) WithInitParams(p *client.KubernetesInitParams) *KubeProxyChecker {
	c.initParams = p
	return c
}

func (c *KubeProxyChecker) WithLogResult(f bool) *KubeProxyChecker {
	c.logCheckResult = f
	return c
}

func (c *KubeProxyChecker) WithAskPassword(f bool) *KubeProxyChecker {
	c.askPassword = f
	return c
}

func (c *KubeProxyChecker) WithStopProxy(f bool) *KubeProxyChecker {
	c.stopProxy = f
	return c
}

func (c *KubeProxyChecker) WithClusterUUID(uuid string) *KubeProxyChecker {
	c.clusterUUID = uuid
	return c
}

func (c *KubeProxyChecker) WithExternalIPs(ips map[string]string) *KubeProxyChecker {
	c.nodesExternalIPs = ips
	return c
}

func (c *KubeProxyChecker) WithSSHProvider(s libcon.SSHProvider, sett *settings.BaseProviders) *KubeProxyChecker {
	c.sshProvider = s
	c.baseProviderSettings = sett
	return c
}

func (c *KubeProxyChecker) IsReady(ctx context.Context, nodeName string) (bool, error) {
	if c.initParams == nil {
		return false, fmt.Errorf("kube proxy checker: Kubernetes init params are not configured")
	}
	if c.baseProviderSettings == nil {
		return false, fmt.Errorf("kube proxy checker: base provider settings are not configured")
	}
	if c.sshProvider == nil {
		return false, fmt.Errorf("kube proxy checker: SSH provider is not configured")
	}

	sshClient, err := c.sshProvider.Client(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get SSH client: %w", err)
	}

	if len(c.nodesExternalIPs) > 0 {
		ip, ok := c.nodesExternalIPs[nodeName]
		if !ok {
			return false, fmt.Errorf("no external IP found for node %s", nodeName)
		}

		sshClient.Session().SetAvailableHosts([]session.Host{
			{
				Host: ip,
				Name: nodeName,
			},
		})
	}

	kubeCl := kube.NewKubernetesClient(c.baseProviderSettings).
		WithNodeInterface(
			ssh.NewNodeInterfaceWrapper(sshClient, c.baseProviderSettings),
		)

	localInitParams := copyKubernetesInitParams(c.initParams)

	params := &kube.Config{
		KubeConfig:          localInitParams.KubeConfig,
		KubeConfigContext:   localInitParams.KubeConfigContext,
		KubeConfigInCluster: localInitParams.KubeConfigInCluster,
		RestConfig:          localInitParams.RestConfig,
	}

	if err := kubeCl.InitContext(ctx, params); err != nil {
		return false, fmt.Errorf("failed to open Kubernetes connection: %w", err)
	}

	defer func() {
		if !c.stopProxy {
			return
		}

		if kubeCl.KubeProxy != nil {
			kubeCl.KubeProxy.StopAll()
		}
	}()

	// d8-cluster-uuid
	cm, err := kubeCl.CoreV1().
		ConfigMaps("kube-system").
		Get(ctx, "d8-cluster-uuid", v1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get cluster UUID ConfigMap: %w", err)
	}

	c.printNs(ctx, cm)

	uuidInCluster := cm.Data["cluster-uuid"]
	if c.clusterUUID != "" && c.clusterUUID != uuidInCluster {
		return false, fmt.Errorf(
			"incorrect cluster UUID: cluster has %s, but %s was passed",
			uuidInCluster,
			c.clusterUUID,
		)
	}

	return true, nil
}

func copyKubernetesInitParams(
	params *client.KubernetesInitParams,
) *client.KubernetesInitParams {
	if params == nil {
		return nil
	}

	result := *params

	if params.RestConfig != nil {
		result.RestConfig = rest.CopyConfig(params.RestConfig)
	}

	return &result
}

func (c *KubeProxyChecker) Name() string {
	return "SSH access and kube-proxy availability"
}

func (c *KubeProxyChecker) printNs(ctx context.Context, cm *corev1.ConfigMap) {
	if !c.logCheckResult {
		return
	}

	yamlRepr, err := yaml.Marshal(cm)
	if err != nil {
		dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("ConfigMap marshal error %v", err))
		return
	}

	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Cluster UUID ConfigMap:\n%s", string(yamlRepr)))
}
