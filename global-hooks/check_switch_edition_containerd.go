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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

const (
	cseEditionName      = "CSE"
	criTypeContainerdV1 = "Containerd"
)

var nodeGroupGVRForCRICheck = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1",
	Resource: "nodegroups",
}

// containerdV1SwitchEditionChecker blocks a switch to CSE while the cluster still asks for containerd v1 (EE to CSE).
func containerdV1SwitchEditionChecker(
	ctx context.Context,
	input *go_hook.HookInput,
	dc dependency.Container,
	p *switchEditionCheckerParams,
) error {
	allowSwitch := func(cause string, args ...any) error {
		input.Logger.Info(
			"Allow switch edition by containerd checker",
			slog.String("cause", fmt.Sprintf(cause, args...)),
		)
		return nil
	}

	if p.currentEdition != cseEditionName {
		return allowSwitch("current edition %s is not CSE", p.currentEdition)
	}

	client, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot get k8s client for containerd check: %w", err)
	}

	defaultCRI, found, err := clusterDefaultCRI(ctx, client)
	if err != nil {
		return err
	}
	if !found {
		return allowSwitch("kube-system/d8-cluster-configuration secret not found")
	}

	if defaultCRI == criTypeContainerdV1 {
		return fmt.Errorf(
			"defaultCRI is %s in the kube-system/d8-cluster-configuration secret, but CSE does not ship "+
				"containerd v1. Set defaultCRI to ContainerdV2 and migrate the nodes first",
			criTypeContainerdV1,
		)
	}

	blocked, err := nodeGroupsRequestingContainerdV1(ctx, client, defaultCRI)
	if err != nil {
		return err
	}
	if len(blocked) > 0 {
		return fmt.Errorf(
			"NodeGroups %v resolve to CRI %s, but CSE does not ship containerd v1. "+
				"Set spec.cri.type to ContainerdV2 and migrate their nodes first",
			blocked, criTypeContainerdV1,
		)
	}

	return allowSwitch("no NodeGroup requests containerd v1")
}

// clusterDefaultCRI reads defaultCRI from the ClusterConfiguration secret.
func clusterDefaultCRI(ctx context.Context, client k8s.Client) (string, bool, error) {
	secret, err := client.CoreV1().Secrets("kube-system").Get(ctx, "d8-cluster-configuration", metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("cannot get kube-system/d8-cluster-configuration: %w", err)
	}

	raw, ok := secret.Data["cluster-configuration.yaml"]
	if !ok {
		return "", false, nil
	}

	var clusterConfig struct {
		DefaultCRI string `json:"defaultCRI"`
	}
	if err := yaml.Unmarshal(raw, &clusterConfig); err != nil {
		return "", false, fmt.Errorf("cannot parse cluster-configuration.yaml: %w", err)
	}

	return clusterConfig.DefaultCRI, true, nil
}

func nodeGroupsRequestingContainerdV1(ctx context.Context, client k8s.Client, defaultCRI string) ([]string, error) {
	nodeGroups, err := client.Dynamic().Resource(nodeGroupGVRForCRICheck).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot list NodeGroups: %w", err)
	}

	var blocked []string
	for _, item := range nodeGroups.Items {
		criType, _, err := unstructured.NestedString(item.Object, "spec", "cri", "type")
		if err != nil {
			return nil, fmt.Errorf("cannot read spec.cri.type of NodeGroup %s: %w", item.GetName(), err)
		}

		if effectiveCRIType(criType, defaultCRI) == criTypeContainerdV1 {
			blocked = append(blocked, item.GetName())
		}
	}

	sort.Strings(blocked)

	return blocked, nil
}

// effectiveCRIType resolves the CRI type for a NodeGroup: the NodeGroup wins, then defaultCRI, then containerd v1.
func effectiveCRIType(nodeGroupCRI, defaultCRI string) string {
	if nodeGroupCRI != "" {
		return nodeGroupCRI
	}
	if defaultCRI != "" {
		return defaultCRI
	}

	return criTypeContainerdV1
}
