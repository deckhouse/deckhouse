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

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

// Once the release of this module has succeeded without the ClusterIssuer grant objects, it no longer
// lists them and will not prune them, so the keep policy migrate_clusterissuers_grants.go set is
// removed: the cert-manager release owns the objects and must be able to delete them when it stops
// rendering them.
//
// It removes the keep policy unconditionally, whether cert-manager is enabled or not: the policy is
// needed only between the hooks of one run.
//
// TODO: remove together with migrate_clusterissuers_grants.go, in the first minor release after the
// next LTS release following v1.77 (see the TODO there).
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/160-multitenancy-manager",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes:  clusterIssuersGrantKubernetes,
}, releaseClusterIssuersGrants)

func releaseClusterIssuersGrants(_ context.Context, input *go_hook.HookInput) error {
	return patchResourcePolicy(input, nil)
}
