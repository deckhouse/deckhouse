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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/control-plane-manager/alerting",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, checkEncryptionAlgorithmMigration)

const defaultEncryptionAlgorithm = "RSA-2048"

func checkEncryptionAlgorithmMigration(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("D8EncryptionAlgorithmDeprecatedInClusterConfiguration")

	ccAlgo := input.Values.Get("global.clusterConfiguration.encryptionAlgorithm").String()
	mcAlgo := input.Values.Get("controlPlaneManager.encryptionAlgorithm").String()

	if ccAlgo != "" && ccAlgo != defaultEncryptionAlgorithm && mcAlgo == defaultEncryptionAlgorithm {
		input.MetricsCollector.Set(
			"d8_encryption_algorithm_deprecated_in_cluster_configuration", 1,
			map[string]string{},
			metrics.WithGroup("D8EncryptionAlgorithmDeprecatedInClusterConfiguration"),
		)
	}

	return nil
}
