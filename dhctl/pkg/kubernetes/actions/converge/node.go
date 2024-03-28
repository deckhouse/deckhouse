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

package converge

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	nodeGroupResource   = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
	ErrNodeGroupChanged = fmt.Errorf("Node group was changed during accept diff.")
)

const HideDeckhouseLogs = false
const ShowDeckhouseLogs = true

func GetCloudConfig(kubeCl *client.KubernetesClient, nodeGroupName string, showDeckhouseLogs bool) (string, error) {
	var cloudData string

	name := fmt.Sprintf("Waiting for %s cloud config️", nodeGroupName)
	err := log.Process("default", name, func() error {
		if showDeckhouseLogs {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					default:
						_, _ = deckhouse.NewLogPrinter(kubeCl).
							WithLeaderElectionAwarenessMode(types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-leader-election"}).
							Print(ctx)
					}
				}
			}()
		}

		err := retry.NewSilentLoop(name, 45, 5*time.Second).Run(func() error {
			secret, err := kubeCl.CoreV1().
				Secrets("d8-cloud-instance-manager").
				Get(context.TODO(), "manual-bootstrap-for-"+nodeGroupName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			cloudData = base64.StdEncoding.EncodeToString(secret.Data["cloud-config"])
			return nil
		})
		if err != nil {
			return err
		}

		log.InfoLn("Cloud configuration found!")
		return nil
	})
	return cloudData, err
}

func CreateNodeGroup(kubeCl *client.KubernetesClient, nodeGroupName string, data map[string]interface{}) error {
	doc := unstructured.Unstructured{}
	doc.SetUnstructuredContent(data)

	return retry.NewLoop(fmt.Sprintf("Create NodeGroup %q", nodeGroupName), 45, 15*time.Second).Run(func() error {
		res, err := kubeCl.Dynamic().
			Resource(nodeGroupResource).
			Create(context.TODO(), &doc, metav1.CreateOptions{})
		if err == nil {
			log.InfoF("NodeGroup %q created\n", res.GetName())
			return nil
		}

		if errors.IsAlreadyExists(err) {
			log.InfoF("Object %v, updating ... ", err)
			content, err := doc.MarshalJSON()
			if err != nil {
				return err
			}
			_, err = kubeCl.Dynamic().
				Resource(nodeGroupResource).
				Patch(context.TODO(), doc.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
			if err != nil {
				return err
			}
			log.InfoLn("OK!")
			return nil
		}

		return err
	})
}

func GetNodeGroupDirect(kubeCl *client.KubernetesClient, nodeGroupName string) (*unstructured.Unstructured, error) {
	var err error
	ng, err := kubeCl.Dynamic().
		Resource(nodeGroupResource).
		Get(context.TODO(), nodeGroupName, metav1.GetOptions{})

	return ng, err
}

func GetNodeGroup(kubeCl *client.KubernetesClient, nodeGroupName string) (*unstructured.Unstructured, error) {
	var ng *unstructured.Unstructured
	err := retry.NewSilentLoop(fmt.Sprintf("Get NodeGroup %q", nodeGroupName), 45, 15*time.Second).Run(func() error {
		var err error
		ng, err = GetNodeGroupDirect(kubeCl, nodeGroupName)

		return err
	})

	if err != nil {
		return nil, err
	}

	return ng, nil
}

func GetNodeGroups(kubeCl *client.KubernetesClient) ([]unstructured.Unstructured, error) {
	ngs, err := kubeCl.Dynamic().
		Resource(nodeGroupResource).
		List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	return ngs.Items, err
}

func UpdateNodeGroup(kubeCl *client.KubernetesClient, nodeGroupName string, ng *unstructured.Unstructured) error {
	err := retry.NewLoop(fmt.Sprintf("Update node template in NodeGroup %q", nodeGroupName), 45, 15*time.Second).
		BreakIf(errors.IsConflict).
		Run(func() error {
			_, err := kubeCl.Dynamic().
				Resource(nodeGroupResource).
				Update(context.TODO(), ng, metav1.UpdateOptions{})

			return err
		})

	if errors.IsConflict(err) {
		return ErrNodeGroupChanged
	}

	return err
}

func WaitForSingleNodeBecomeReady(kubeCl *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(fmt.Sprintf("Waiting for  Node %s to become Ready", nodeName), 100, 20*time.Second).Run(func() error {
		node, err := kubeCl.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for _, c := range node.Status.Conditions {
			if c.Type == apiv1.NodeReady {
				if c.Status == apiv1.ConditionTrue {
					return nil
				}
			}
		}

		return fmt.Errorf("node %q is not Ready yet", nodeName)
	})
}

func WaitForNodesBecomeReady(kubeCl *client.KubernetesClient, nodeGroupName string, desiredReadyNodes int) error {
	return retry.NewLoop(fmt.Sprintf("Waiting for NodeGroup %s to become Ready", nodeGroupName), 100, 20*time.Second).Run(func() error {
		nodes, err := kubeCl.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node.deckhouse.io/group=" + nodeGroupName})
		if err != nil {
			return err
		}

		readyNodes := make(map[string]struct{})

		for _, node := range nodes.Items {
			for _, c := range node.Status.Conditions {
				if c.Type == apiv1.NodeReady {
					if c.Status == apiv1.ConditionTrue {
						readyNodes[node.Name] = struct{}{}
					}
				}
			}
		}

		message := fmt.Sprintf("Nodes Ready %v of %v\n", len(readyNodes), desiredReadyNodes)
		for _, node := range nodes.Items {
			condition := "NotReady"
			if _, ok := readyNodes[node.Name]; ok {
				condition = "Ready"
			}
			message += fmt.Sprintf("* %s | %s\n", node.Name, condition)
		}

		if len(readyNodes) >= desiredReadyNodes {
			log.InfoLn(message)
			return nil
		}

		return fmt.Errorf(strings.TrimSuffix(message, "\n"))
	})
}

func WaitForNodesListBecomeReady(kubeCl *client.KubernetesClient, nodes []string, checker hook.NodeChecker) error {
	return retry.NewLoop("Waiting for nodes to become Ready", 100, 20*time.Second).Run(func() error {
		desiredReadyNodes := len(nodes)
		var nodesList apiv1.NodeList

		for _, nodeName := range nodes {
			node, err := kubeCl.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			nodesList.Items = append(nodesList.Items, *node)
		}

		readyNodes := make(map[string]struct{})

		for _, node := range nodesList.Items {
			for _, c := range node.Status.Conditions {
				if c.Type == apiv1.NodeReady {
					if c.Status == apiv1.ConditionTrue {
						ready := true
						if checker != nil {
							var err error
							ready, err = checker.IsReady(node.Name)
							if err != nil {
								log.WarnF("While doing check '%s' node %s has error: %v\n", checker.Name(), node.Name, err)
							} else if !ready {
								log.InfoF("Node %s is ready but %s is not ready\n", node.Name, checker.Name())
							}
						}

						if ready {
							readyNodes[node.Name] = struct{}{}
						}
					}
				}
			}
		}

		message := fmt.Sprintf("Nodes Ready %v of %v\n", len(readyNodes), desiredReadyNodes)
		for _, node := range nodesList.Items {
			condition := "NotReady"
			if _, ok := readyNodes[node.Name]; ok {
				condition = "Ready"
			}
			message += fmt.Sprintf("* %s | %s\n", node.Name, condition)
		}

		if len(readyNodes) >= desiredReadyNodes {
			log.InfoLn(message)
			return nil
		}

		return fmt.Errorf(strings.TrimSuffix(message, "\n"))
	})
}

func GetNodeGroupTemplates(kubeCl *client.KubernetesClient) (map[string]map[string]interface{}, error) {
	nodeTemplates := make(map[string]map[string]interface{})

	err := retry.NewLoop("Get NodeGroups node template settings", 10, 5*time.Second).Run(func() error {
		nodeGroups, err := kubeCl.Dynamic().Resource(nodeGroupResource).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, group := range nodeGroups.Items {
			var nodeTemplate map[string]interface{}
			if spec, ok := group.Object["spec"].(map[string]interface{}); ok {
				nodeTemplate, _ = spec["nodeTemplate"].(map[string]interface{})
				// if we do not set node template in cluster provider configuration
				// we get nil node template from config,
				// but k8s always returns empty map (not nil)
				// and we have D8TerraformStateExporterNodeTemplateChanged alert
				// therefore, we convert empty map to nil
				if len(nodeTemplate) == 0 {
					nodeTemplate = nil
				}
			}

			nodeTemplates[group.GetName()] = nodeTemplate
		}
		return nil
	})

	return nodeTemplates, err
}

func DeleteNode(kubeCl *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(fmt.Sprintf("Delete Node %s", nodeName), 45, 10*time.Second).Run(func() error {
		err := kubeCl.CoreV1().Nodes().Delete(context.TODO(), nodeName, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			// Node has already been deleted
			return nil
		}
		return err
	})
}

func DeleteNodeGroup(kubeCl *client.KubernetesClient, nodeGroupName string) error {
	return retry.NewLoop(fmt.Sprintf("Delete NodeGroup %s", nodeGroupName), 45, 10*time.Second).Run(func() error {
		err := kubeCl.Dynamic().Resource(nodeGroupResource).Delete(context.TODO(), nodeGroupName, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			// NodeGroup has already been deleted
			return nil
		}
		return err
	})
}

func requestNodeExists(kubeCl *client.KubernetesClient, nodeName string) (bool, error) {
	_, err := kubeCl.
		CoreV1().
		Nodes().
		Get(context.TODO(), nodeName, metav1.GetOptions{})

	if err == nil {
		return true, nil
	}

	if errors.IsNotFound(err) {
		return false, nil
	}

	return true, err
}

func IsNodeExistsInCluster(kubeCl *client.KubernetesClient, nodeName string) (bool, error) {
	exists := false
	err := retry.NewLoop(fmt.Sprintf("Checking node exists %s", nodeName), 5, 2*time.Second).Run(func() error {
		var err error
		exists, err = requestNodeExists(kubeCl, nodeName)
		return err
	})

	return exists, err
}
