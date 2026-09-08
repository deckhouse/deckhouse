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

package converge

import (
	gocontext "context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
)

// Same budget as entity.GetNodeGroup and as the per-group read in
// controller/immutable.go: converge itself restarts the apiservers it reads through.
const (
	nodeGroupListAttempts = 600
	nodeGroupListInterval = 1 * time.Second
)

// masterGroupIsImmutable reports whether the cluster's control plane runs immutable
// machines. They answer no sshd: everything converge does over SSH — creating a node
// user and rebuilding the Kubernetes client through a master — has nothing to connect
// to, and the node phases run against the client converge already holds.
//
// A read that keeps failing stops the converge instead of answering. Guessing the
// classic path here creates a NodeUser and waits for bashible on machines that run
// none, while the same object answers "immutable" to every other reader in the run.
func masterGroupIsImmutable(ctx gocontext.Context, kubeGetter kubernetes.KubeClientProviderWithCtx) (bool, error) {
	kubeCl, err := kubeGetter.KubeClientCtx(ctx)
	if err != nil {
		return false, fmt.Errorf("get kube client to read NodeGroups: %w", err)
	}

	var nodeGroups []unstructured.Unstructured
	err = retry.NewSilentLoop("List NodeGroups", nodeGroupListAttempts, nodeGroupListInterval).
		RunContext(ctx, func() error {
			var err error
			nodeGroups, err = entity.GetNodeGroups(ctx, kubeCl)

			return err
		})
	if err != nil {
		return false, fmt.Errorf("read NodeGroups to tell an immutable control plane from a classic one: %w", err)
	}

	for _, ng := range nodeGroups {
		if ng.GetName() != global.MasterNodeGroupName {
			continue
		}

		return immutable.NodeGroupIsImmutable(&ng), nil
	}

	return false, nil
}
