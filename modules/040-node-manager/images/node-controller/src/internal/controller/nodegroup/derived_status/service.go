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

package derived_status

import (
	"context"
	"encoding/json"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/capacity"
)

func versionString(v *semver.Version) string {
	if v == nil {
		return "<nil>"
	}
	return v.String()
}

// Service reads everything through the manager cache. All of its sources live in kube-system,
// which internal/common/cache.go caches whole precisely so this path never issues a live GET —
// a live GET here used to cost hundreds of ms on every pass during a NodeGroup burst.
type Service struct {
	Client client.Client
}

// Result holds the get_crds-derived fields destined for NodeGroup.status.
type Result struct {
	Engine            string
	KubernetesVersion string
	CRIType           string
	Zones             []string
	NodeCapacity      *capacity.InstanceType
	InstanceClass     map[string]any
	SerializedLabels  string
	SerializedTaints  string
	UpdateEpoch       string
}

// ComputeWithCloudChecks derives get_crds fields and validation diagnostics from
// the same provider snapshot, matching the old hook's single-pass behavior.
func (s *Service) ComputeWithCloudChecks(ctx context.Context, ng *v1.NodeGroup) (Result, CloudCheckResult, error) {
	snap, err := s.BuildSnapshot(ctx, ng)
	if err != nil {
		return Result{}, CloudCheckResult{}, err
	}
	result, err := Derive(ctx, ng, snap)
	if err != nil {
		return result, CloudCheckResult{}, err
	}
	return result, Validate(ng, snap), nil
}

func normalizeJSONMap(v any) map[string]any {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
