// Copyright 2024 Flant JSC
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

package utils

import (
	"context"
	goerrors "errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	kubedrain "github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type DrainOptions struct {
	Force bool
}

func GetDrainConfirmation(commanderMode bool) func(string) bool {
	if commanderMode {
		return func(msg string) bool {
			return input.NewConfirmation().WithMessage(msg).WithYesByDefault().Ask()
		}
	}

	return func(msg string) bool {
		return input.NewConfirmation().WithMessage(msg).Ask()
	}
}

func TryToDrainNode(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string, confirm func(string) bool, opts DrainOptions) error {
	// todo it is deeper for pass from command root, use app package directly
	if app.SkipDrainingNodes() {
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Skipping draining node %s because draining is disabled by env", nodeName))
		return nil
	}

	err := retry.NewLoop(fmt.Sprintf("Drain node '%s'", nodeName), 5, 10*time.Second).
		RunContext(ctx, func() error {
			return drainNode(ctx, kubeCl, nodeName, opts)
		})
	if err != nil {
		if goerrors.Is(err, kubedrain.ErrDrainTimeout) {
			if confirm("Cannot drain the node because the process timed out. Continue without a full-fledged drain?") {
				dhlog.FromContext(ctx).WarnContext(ctx, "Continuing without a full-fledged drain")
				return nil
			}

			return err
		}
		return err
	}

	return nil
}

func drainNode(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string, opts DrainOptions) error {
	node, err := kubeCl.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Node '%s' has been deleted. Skip", nodeName))
			return nil
		}

		return fmt.Errorf("failed to get node '%s': %v", nodeName, err)
	}

	drainer := &kubedrain.Helper{
		Client:              kubeCl,
		Ctx:                 ctx,
		IgnoreAllDaemonSets: true,
		DeleteEmptyDirData:  true,
		GracePeriodSeconds:  -1,
		Force:               opts.Force,
		// If a pod is not evicted in 30 seconds, retry the eviction next time.
		Timeout: 30 * time.Second,
		OnPodDeletedOrEvicted: func(pod *corev1.Pod, usingEviction bool) {
			verb := "Deleted"
			if usingEviction {
				verb = "Evicted"
			}

			dhlog.FromContext(ctx).DebugContext(ctx, strings.TrimRight(fmt.Sprintf("'%s' pod '%s' from Node", verb, klog.KObj(pod)), "\n"))
		},
		Out:    writer{func(elems ...interface{}) { dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprint(elems...)) }},
		ErrOut: writer{func(elems ...interface{}) { dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprint(elems...)) }},
	}

	if isNodeUnreachable(node) {
		// When the node is unreachable and some pods are not evicted for as long as this timeout, we ignore them.
		drainer.SkipWaitForDeleteTimeoutSeconds = 60 * 5 // 5 minutes
	}

	err = kubedrain.RunCordonOrUncordon(drainer, node, true)
	if err != nil {
		return fmt.Errorf("failed to cordon node '%s': %w", node.Name, err)
	}

	err = kubedrain.RunNodeDrain(drainer, node.Name)
	if err != nil {
		return fmt.Errorf("failed to drain node '%s': %w", node.Name, err)
	}

	return nil
}

func isNodeUnreachable(node *corev1.Node) bool {
	if node == nil {
		return false
	}

	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionUnknown
		}
	}

	return false
}

// writer implements io.Writer interface as a pass-through for klog.
type writer struct {
	logFunc func(elems ...interface{})
}

func (w writer) Write(p []byte) (int, error) {
	w.logFunc(string(p))

	return len(p), nil
}
