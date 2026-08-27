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

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/immutable"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
)

// masterGroupIsImmutable reports whether the cluster's control plane runs immutable
// machines. They answer no sshd: everything converge does over SSH — creating a node
// user and rebuilding the Kubernetes client through a master — has nothing to connect
// to, and the node phases run against the client converge already holds.
//
// The probe runs on every converge of every cluster, so it answers with the classic
// SSH path whenever the NodeGroups cannot be read: that path is what every cluster but
// this one needs, and an unreadable list must not abort the phase for all of them.
func masterGroupIsImmutable(ctx gocontext.Context, kubeGetter kubernetes.KubeClientProviderWithCtx) bool {
	kubeCl, err := kubeGetter.KubeClientCtx(ctx)
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Keeping the classic SSH path: get kube client to read NodeGroups: %v", err))
		return false
	}

	nodeGroups, err := entity.GetNodeGroups(ctx, kubeCl)
	if err != nil {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Keeping the classic SSH path: read NodeGroups to tell an immutable control plane from a classic one: %v", err))
		return false
	}

	for _, ng := range nodeGroups {
		if ng.GetName() != global.MasterNodeGroupName {
			continue
		}
		return immutable.NodeGroupIsImmutable(&ng)
	}

	return false
}
