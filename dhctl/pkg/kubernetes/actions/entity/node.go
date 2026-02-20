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

package entity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	nodeGroupResource = schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "nodegroups"}
)

const (
	defaultCloudConfigRetryAttempts = 60
	defaultCloudConfigRetryInterval = 10 * time.Second
)

type NodeIP struct {
	InternalIP string
	ExternalIP string
	NodeName   string
}

type CloudConfigOptions struct {
	ExcludePackagesProxyEndpointIP string
	RetryAttempts                  int
	RetryInterval                  time.Duration
}

func (i *NodeIP) Name() string {
	if i.NodeName != "" {
		return i.NodeName
	}

	return i.InternalIP
}

func GetCloudConfig(ctx context.Context, kubeCl *client.KubernetesClient, nodeGroupName string, showDeckhouseLogs bool, logger log.Logger, apiserverHosts ...string) (string, error) {
	return GetCloudConfigWithOptions(ctx, kubeCl, nodeGroupName, showDeckhouseLogs, logger, CloudConfigOptions{}, apiserverHosts...)
}

func GetCloudConfigWithOptions(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	nodeGroupName string,
	showDeckhouseLogs bool,
	logger log.Logger,
	opts CloudConfigOptions,
	apiserverHosts ...string,
) (string, error) {
	var cloudData string
	retryAttempts, retryInterval := resolveCloudConfigRetryOptions(opts)

	name := fmt.Sprintf("Waiting for %s cloud config️", nodeGroupName)
	err := logger.LogProcess("default", name, func() error {
		if showDeckhouseLogs {
			ctx, cancel := context.WithCancel(ctx)
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

		err := retry.NewSilentLoop(name, retryAttempts, retryInterval).RunContext(ctx, func() error {
			if nodeGroupName == global.MasterNodeGroupName {
				if opts.ExcludePackagesProxyEndpointIP != "" {
					logger.LogInfoF(
						"Waiting while packages proxy endpoint IP '%s' is excluded from bootstrap secret for node group '%s'\n",
						opts.ExcludePackagesProxyEndpointIP,
						nodeGroupName,
					)
				}
			}
			secret, err := kubeCl.CoreV1().
				Secrets("d8-cloud-instance-manager").
				Get(ctx, "manual-bootstrap-for-"+nodeGroupName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if len(apiserverHosts) > 0 {
				var endpoints []string

				endpointsRaw := secret.Data["apiserverEndpoints"]
				logger.LogDebugF("Got raw apiserverEndpoints: %v", string(endpointsRaw))

				err := yaml.Unmarshal(endpointsRaw, &endpoints)
				if err != nil {
					return fmt.Errorf("failed to unmarshal apiserver endpoints: %v", err)
				}

				hostsMap := make(map[string]struct{}, len(endpoints))

				for _, endpoint := range endpoints {
					host, _, err := net.SplitHostPort(endpoint)
					if err != nil {
						return fmt.Errorf("failed to split endpoint `%s` into host and port: %v", endpoint, err)
					}

					logger.LogDebugF("Got API-server host %s from secret\n", host)

					hostsMap[host] = struct{}{}
				}

				for _, host := range apiserverHosts {
					_, ok := hostsMap[host]
					if !ok {
						return fmt.Errorf("apiserver host '%s' not found in cloud config", host)
					}
				}
			} else if nodeGroupName == global.MasterNodeGroupName {
				logger.LogDebugLn("Got empty apiserver endpoints from arguments")
			}

			if opts.ExcludePackagesProxyEndpointIP != "" {
				endpoints, err := extractPackagesProxyAddressesFromBootstrapSecret(secret.Data)
				if err != nil {
					return fmt.Errorf("failed to parse packages proxy endpoints from bootstrap secret: %v", err)
				}
				if len(endpoints) == 0 {
					return fmt.Errorf(
						"packages proxy endpoints are not found in bootstrap secret while waiting to exclude IP '%s'",
						opts.ExcludePackagesProxyEndpointIP,
					)
				}

				logger.LogInfoF(
					"Bootstrap secret packages proxy endpoints for node group '%s': %s\n",
					nodeGroupName,
					strings.Join(endpoints, ","),
				)

				for _, endpoint := range endpoints {
					host := endpoint
					if parsedHost, _, splitErr := net.SplitHostPort(endpoint); splitErr == nil {
						host = parsedHost
					}
					if host == "" {
						return fmt.Errorf("packages proxy endpoint `%s` has empty host", endpoint)
					}
					if host == opts.ExcludePackagesProxyEndpointIP {
						return fmt.Errorf(
							"packages proxy endpoint with excluded IP '%s' is still present in bootstrap secret. Current endpoints: %s",
							opts.ExcludePackagesProxyEndpointIP,
							strings.Join(endpoints, ","),
						)
					}
				}
			}

			cloudData = base64.StdEncoding.EncodeToString(secret.Data["cloud-config"])

			return nil
		})
		if err != nil {
			return err
		}

		logger.LogInfoLn("Cloud configuration found!")
		return nil
	})
	return cloudData, err
}

func resolveCloudConfigRetryOptions(opts CloudConfigOptions) (int, time.Duration) {
	attempts := opts.RetryAttempts
	if attempts <= 0 {
		attempts = defaultCloudConfigRetryAttempts
	}

	interval := opts.RetryInterval
	if interval <= 0 {
		interval = defaultCloudConfigRetryInterval
	}

	return attempts, interval
}

func extractPackagesProxyAddressesFromBootstrapSecret(secretData map[string][]byte) ([]string, error) {
	rawAddresses := secretData["packagesProxyAddresses"]
	if len(rawAddresses) == 0 {
		return nil, nil
	}

	var addresses []string
	if err := yaml.Unmarshal(rawAddresses, &addresses); err != nil {
		return nil, fmt.Errorf("unmarshal packagesProxyAddresses: %v", err)
	}

	cleanAddresses := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		cleanAddresses = append(cleanAddresses, addr)
	}

	return cleanAddresses, nil
}

func CreateNodeGroup(ctx context.Context, kubeCl *client.KubernetesClient, nodeGroupName string, logger log.Logger, data map[string]interface{}) error {
	doc := unstructured.Unstructured{}
	doc.SetUnstructuredContent(data)

	return retry.NewLoop(fmt.Sprintf("Create NodeGroup %q", nodeGroupName), 45, 15*time.Second).
		WithLogger(logger).
		RunContext(ctx, func() error {
			res, err := kubeCl.Dynamic().
				Resource(nodeGroupResource).
				Create(ctx, &doc, metav1.CreateOptions{})
			if err == nil {
				logger.LogInfoF("NodeGroup %q created\n", res.GetName())
				return nil
			}

			if errors.IsAlreadyExists(err) {
				logger.LogInfoF("Object %v, updating ... ", err)
				content, err := doc.MarshalJSON()
				if err != nil {
					return err
				}
				_, err = kubeCl.Dynamic().
					Resource(nodeGroupResource).
					Patch(ctx, doc.GetName(), types.MergePatchType, content, metav1.PatchOptions{})
				if err != nil {
					return err
				}
				logger.LogInfoF("OK!")
				return nil
			}

			return err
		})
}

func GetNodeGroupDirect(ctx context.Context, kubeCl *client.KubernetesClient, nodeGroupName string) (*unstructured.Unstructured, error) {
	var err error
	ng, err := kubeCl.Dynamic().
		Resource(nodeGroupResource).
		Get(ctx, nodeGroupName, metav1.GetOptions{})

	return ng, err
}

func GetNodeGroup(ctx context.Context, kubeCl *client.KubernetesClient, nodeGroupName string) (*unstructured.Unstructured, error) {
	var ng *unstructured.Unstructured
	err := retry.NewSilentLoop(fmt.Sprintf("Get NodeGroup %q", nodeGroupName), 45, 15*time.Second).
		RunContext(ctx, func() error {
			var err error
			ng, err = GetNodeGroupDirect(ctx, kubeCl, nodeGroupName)

			return err
		})
	if err != nil {
		return nil, err
	}

	return ng, nil
}

func GetNodeGroups(ctx context.Context, kubeCl *client.KubernetesClient) ([]unstructured.Unstructured, error) {
	ngs, err := kubeCl.Dynamic().
		Resource(nodeGroupResource).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return ngs.Items, err
}

func UpdateNodeGroup(ctx context.Context, kubeCl *client.KubernetesClient, nodeGroupName string, ng *unstructured.Unstructured) error {
	err := retry.NewLoop(fmt.Sprintf("Update node template in NodeGroup %q", nodeGroupName), 45, 15*time.Second).
		BreakIf(errors.IsConflict).
		RunContext(ctx, func() error {
			_, err := kubeCl.Dynamic().
				Resource(nodeGroupResource).
				Update(ctx, ng, metav1.UpdateOptions{})

			return err
		})

	if errors.IsConflict(err) {
		return global.ErrNodeGroupChanged
	}

	return err
}

func WaitForSingleNodeBecomeReady(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(fmt.Sprintf("Waiting for Node %s to become Ready", nodeName), 100, 20*time.Second).
		RunContext(ctx, func() error {
			node, err := kubeCl.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			for _, c := range node.Status.Conditions {
				if c.Type == corev1.NodeReady {
					if c.Status == corev1.ConditionTrue {
						return nil
					}
				}
			}

			return fmt.Errorf("node %q is not Ready yet", nodeName)
		})
}

func WaitForNodesBecomeReady(ctx context.Context, kubeCl *client.KubernetesClient, nodeGroupsMap map[string]int) error {
	ngsName := slices.Collect(maps.Keys(nodeGroupsMap))
	return retry.NewLoop(fmt.Sprintf("Waiting for NodeGroups %v to become Ready", ngsName), 100, 20*time.Second).
		RunContext(ctx, func() error {
			desiredReadyNodes := 0
			for _, countNodes := range nodeGroupsMap {
				desiredReadyNodes += countNodes
			}
			labelSel := fmt.Sprintf("node.deckhouse.io/group in (%s)", strings.Join(ngsName, ", "))

			nodes, err := kubeCl.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: labelSel})
			if err != nil {
				return err
			}

			readyNodes := make(map[string]struct{})

			for _, node := range nodes.Items {
				for _, c := range node.Status.Conditions {
					if c.Type == corev1.NodeReady {
						if c.Status == corev1.ConditionTrue {
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

			return fmt.Errorf("%s", strings.TrimSuffix(message, "\n"))
		})
}

func WaitForNodesListBecomeReady(ctx context.Context, kubeCl *client.KubernetesClient, nodes []string, checker hook.NodeChecker) error {
	return retry.NewLoop("Waiting for nodes to become Ready", 100, 20*time.Second).
		RunContext(ctx, func() error {
			desiredReadyNodes := len(nodes)
			var nodesList corev1.NodeList

			for _, nodeName := range nodes {
				node, err := kubeCl.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				nodesList.Items = append(nodesList.Items, *node)
			}

			readyNodes := make(map[string]struct{})

			for _, node := range nodesList.Items {
				for _, c := range node.Status.Conditions {
					if c.Type == corev1.NodeReady {
						if c.Status == corev1.ConditionTrue {
							ready := true
							if checker != nil {
								var err error
								ready, err = checker.IsReady(ctx, node.Name)
								if err != nil {
									log.InfoF("While doing check '%s' node %s has error: %v\n", checker.Name(), node.Name, err)
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

			return fmt.Errorf("%s", strings.TrimSuffix(message, "\n"))
		})
}

func GetNodeGroupTemplates(ctx context.Context, kubeCl *client.KubernetesClient) (map[string]map[string]interface{}, error) {
	nodeTemplates := make(map[string]map[string]interface{})

	err := retry.NewLoop("Get NodeGroups node template settings", 10, 5*time.Second).
		RunContext(ctx, func() error {
			nodeGroups, err := kubeCl.Dynamic().Resource(nodeGroupResource).List(ctx, metav1.ListOptions{})
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

func DeleteNode(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string) error {
	return retry.NewLoop(fmt.Sprintf("Delete Node %s", nodeName), 45, 10*time.Second).
		RunContext(ctx, func() error {
			err := kubeCl.CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
			if errors.IsNotFound(err) {
				// Node has already been deleted
				return nil
			}
			return err
		})
}

func DeleteNodeGroup(ctx context.Context, kubeCl *client.KubernetesClient, nodeGroupName string) error {
	return retry.NewLoop(fmt.Sprintf("Delete NodeGroup %s", nodeGroupName), 45, 10*time.Second).
		RunContext(ctx, func() error {
			err := kubeCl.Dynamic().Resource(nodeGroupResource).Delete(ctx, nodeGroupName, metav1.DeleteOptions{})
			if errors.IsNotFound(err) {
				// NodeGroup has already been deleted
				return nil
			}
			return err
		})
}

func requestNodeExists(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string) (bool, error) {
	_, err := kubeCl.
		CoreV1().
		Nodes().
		Get(ctx, nodeName, metav1.GetOptions{})

	if err == nil {
		return true, nil
	}

	if errors.IsNotFound(err) {
		return false, nil
	}

	return true, err
}

func IsNodeExistsInCluster(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string, logger log.Logger) (bool, error) {
	exists := false
	err := retry.NewLoop(fmt.Sprintf("Checking node exists %s", nodeName), 5, 2*time.Second).
		WithLogger(logger).
		RunContext(ctx, func() error {
			var err error
			exists, err = requestNodeExists(ctx, kubeCl, nodeName)
			return err
		})

	return exists, err
}

var (
	getMasterNodesIPsDefaultOpts = retry.AttemptsWithWaitOpts(5, 5*time.Second)
)

func GetMasterNodesIPs(ctx context.Context, kubeProvider kubernetes.KubeClientProviderWithCtx, loopParams retry.Params) ([]NodeIP, error) {
	selector, err := kubernetes.GetLabelSelector(global.NodeGroupLabel, selection.Equals, []string{global.MasterNodeGroupName})
	if err != nil {
		return nil, err
	}

	listOpts := metav1.ListOptions{LabelSelector: selector}

	var nodes *corev1.NodeList

	loopParams = retry.SafeCloneOrNewParams(loopParams, getMasterNodesIPsDefaultOpts...).
		WithName("Get control plane nodes IPs from Kubernetes cluster")

	err = retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
		var err error
		kubeCl, err := kubeProvider.KubeClientCtx(ctx)
		if err != nil {
			return err
		}

		nodes, err = kubeCl.CoreV1().Nodes().List(ctx, listOpts)
		if err != nil {
			log.DebugF("Cannot get nodes. Got error: %v", err)
			return err
		}
		return nil
	})

	if err != nil {
		log.DebugF("Cannot get nodes after %d attempts\n", loopParams.Attempts())
		return nil, err
	}

	var nodeIPs []NodeIP

	for _, node := range nodes.Items {
		ip := NodeIP{
			NodeName: node.GetName(),
		}

		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				ip.InternalIP = addr.Address
			}
			if addr.Type == "ExternalIP" {
				ip.ExternalIP = addr.Address
			}
		}
		nodeIPs = append(nodeIPs, ip)
	}

	return nodeIPs, nil
}

func UnstructuredToNodeGroup(o *unstructured.Unstructured) (*v1.NodeGroup, error) {
	content, err := o.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("Cannot marshal nodegroup %s: %v", o.GetName(), err)
	}

	var ng v1.NodeGroup

	err = json.Unmarshal(content, &ng)
	if err != nil {
		return nil, fmt.Errorf("Cannot unmarshal nodegroup %s: %v", o.GetName(), err)
	}

	return &ng, nil
}
