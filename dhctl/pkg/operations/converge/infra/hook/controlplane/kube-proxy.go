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

	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type KubeProxyChecker struct {
	initParams       *client.KubernetesInitParams
	logCheckResult   bool
	askPassword      bool
	stopProxy        bool
	nodesExternalIPs map[string]string
	clusterUUID      string
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

func (c *KubeProxyChecker) IsReady(nodeName string) (bool, error) {
	sshClient := ssh.NewClientFromFlags()

	if len(c.nodesExternalIPs) > 0 {
		ip, ok := c.nodesExternalIPs[nodeName]
		if !ok {
			return false, fmt.Errorf("Not found external ip for node %s", nodeName)
		}

		sshClient.Settings.SetAvailableHosts([]string{ip})
	}

	var err error
	sshClient, err = sshClient.Start()
	if err != nil {
		return false, err
	}

	kubeCl := client.NewKubernetesClient().WithSSHClient(sshClient)
	err = kubeCl.Init(client.AppKubernetesInitParams())
	if err != nil {
		return false, fmt.Errorf("open kubernetes connection: %v", err)
	}

	defer func() {
		if !c.stopProxy {
			return
		}

		if kubeCl.KubeProxy != nil {
			kubeCl.KubeProxy.Stop(0)
		}

		if kubeCl.SSHClient != nil {
			kubeCl.SSHClient.Stop()
		}
	}()

	// d8-cluster-uuid
	ns, err := kubeCl.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "d8-cluster-uuid", v1.GetOptions{})
	if err != nil {
		return false, err
	}

	c.printNs(ns)

	uuidInCluster := ns.Data["cluster-uuid"]
	if c.clusterUUID != "" && c.clusterUUID != uuidInCluster {
		return false, fmt.Errorf("Incorrect cluster uuid. In cluster %s != %s passed.", uuidInCluster, c.clusterUUID)
	}

	return true, nil
}

func (c *KubeProxyChecker) Name() string {
	return "Ssh access and kube-proxy availability"
}

func (c *KubeProxyChecker) printNs(cm *apiv1.ConfigMap) {
	if !c.logCheckResult {
		return
	}

	yamlRepr, err := yaml.Marshal(cm)
	if err != nil {
		log.ErrorF("ConfigMap marshal error %v\n", err)
		return
	}

	log.InfoF("Cluster UUID ConfigMap:\n%s\n", string(yamlRepr))
}
