/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloud_status

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

// getZonesCount returns how many zones the NodeGroup spreads over. An absent provider Secret means
// no cloud and yields zero; an unreadable one is returned as an error, because zero zones makes
// Min and Max zero, and a Min of zero reports the NodeGroup Ready with no nodes at all.
func (s *Service) getZonesCount(ctx context.Context, ng *v1.NodeGroup) (int32, error) {
	if ng.Spec.CloudInstances != nil && len(ng.Spec.CloudInstances.Zones) > 0 {
		return int32(len(ng.Spec.CloudInstances.Zones)), nil
	}

	secret := &corev1.Secret{}
	err := s.Client.Get(ctx, types.NamespacedName{Namespace: "kube-system", Name: common.CloudProviderSecretName}, secret)
	if apierrors.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read cloud provider secret: %w", err)
	}

	var zones []string
	if err := json.Unmarshal(secret.Data["zones"], &zones); err != nil {
		return 0, nil
	}
	return int32(len(zones)), nil
}
