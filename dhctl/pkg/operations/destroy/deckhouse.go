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

package destroy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"time"

	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/converge"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
)

type DeckhouseDestroyer struct {
	convergeUnlocker func(fullUnlock bool)
	sshClient        *ssh.Client
	kubeCl           *client.KubernetesClient
	state            *State
}

func NewDeckhouseDestroyer(sshClient *ssh.Client, state *State) *DeckhouseDestroyer {
	return &DeckhouseDestroyer{
		sshClient: sshClient,
		state:     state,
	}
}

func (g *DeckhouseDestroyer) UnlockConverge(fullUnlock bool) {
	if g.convergeUnlocker != nil {
		g.convergeUnlocker(fullUnlock)
		g.convergeUnlocker = nil
	}
}

func (g *DeckhouseDestroyer) StopProxy() {
	if g.kubeCl == nil {
		return
	}

	g.kubeCl.KubeProxy.Stop(0)
	g.kubeCl = nil
}

func (g *DeckhouseDestroyer) GetKubeClient() (*client.KubernetesClient, error) {
	if g.kubeCl != nil {
		return g.kubeCl, nil
	}

	kubeCl, err := operations.ConnectToKubernetesAPI(g.sshClient)
	if err != nil {
		return nil, err
	}

	unlockConverge, err := converge.LockConvergeFromLocal(kubeCl, "local-destroyer")
	if err != nil {
		return nil, err
	}

	g.kubeCl = kubeCl
	g.convergeUnlocker = unlockConverge

	return kubeCl, err
}

func (g *DeckhouseDestroyer) DeleteResources(cloudType string) error {
	resourcesDestroyed, err := g.state.IsResourcesDestroyed()
	if err != nil {
		return err
	}

	if resourcesDestroyed {
		log.WarnLn("Resources was destroyed. Skip it")
		return nil
	}

	kubeCl, err := g.GetKubeClient()
	if err != nil {
		return err
	}

	err = log.Process("common", "Delete resources from the Kubernetes cluster", func() error {
		return g.deleteEntities(kubeCl)
	})

	if err != nil {
		return err
	}

	return g.PrintCCMLogs(kubeCl)
}

func (g *DeckhouseDestroyer) PrintCCMLogs(kubeCl *client.KubernetesClient) error {
	getPods := func(kubeCl *client.KubernetesClient, namespace, labelSelector string) ([]v1.Pod, error) {
		pods, err := kubeCl.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			log.DebugF("Cannot get pod %s/%s. Got error: %v", namespace, labelSelector, err)
			return nil, err
		}

		if len(pods.Items) == 0 {
			log.DebugF("Cannot get deckhouse pod. Count of returned pods is zero")
			return nil, fmt.Errorf("Pods not found")
		}

		return pods.Items, nil
	}

	const ns = "d8-cloud-provider-yandex"

	getLogs := func(kubeCl *client.KubernetesClient, namespace, podName string) ([]byte, error) {
		logOptions := v1.PodLogOptions{Container: "yandex-cloud-controller-manager", TailLines: pointer.Int64(30000)}
		request := kubeCl.CoreV1().Pods(namespace).GetLogs(podName, &logOptions)
		result, err := request.DoRaw(context.TODO())
		if err != nil {
			return nil, err
		}

		return result, nil
	}

	printLogs := func(logs []byte) {
		reader := bufio.NewReader(bytes.NewReader(logs))
		for {
			l, _, err := reader.ReadLine()
			if err != nil {
				break
			}

			log.InfoLn(string(l))
		}
	}

	var pods []v1.Pod

	err := retry.NewLoop("Get CCM pods", 3, 3*time.Second).Run(func() error {
		var err error
		pods, err = getPods(kubeCl, ns, "app=cloud-controller-manager")
		return err
	})

	if err != nil {
		return err
	}

	for _, pod := range pods {
		var logs []byte
		err := retry.NewLoop(fmt.Sprintf("Get logs for pod %s", pod.Name), 3, 3*time.Second).Run(func() error {
			var err error
			logs, err = getLogs(kubeCl, ns, pod.Name)
			return err
		})
		if err != nil {
			return err
		}

		_ = log.Process("common", fmt.Sprintf("Get logs for pod %s", pod.Name), func() error {
			printLogs(logs)
			return nil
		})
	}

	return nil
}

func (g *DeckhouseDestroyer) deleteEntities(kubeCl *client.KubernetesClient) error {
	err := deckhouse.DeleteDeckhouseDeployment(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForDeckhouseDeploymentDeletion(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteServices(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForServicesDeletion(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteStorageClasses(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePVC(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePods(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVCDeletion(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVDeletion(kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteMachinesIfResourcesExist(kubeCl)
	if err != nil {
		return err
	}

	return nil
}
