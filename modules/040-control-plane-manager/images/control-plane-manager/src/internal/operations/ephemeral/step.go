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

package ephemeral

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StepExecutor struct {
	client            client.Client
	operation         *controlplanev1alpha1.ControlPlaneOperation
	tenantIdentity    tenantIdentity
	clusterDomain     string
	serviceSubnetCIDR string
}

func (e *StepExecutor) Execute(ctx context.Context, stepName controlplanev1alpha1.StepName) (result operations.StepResult) {
	defer func() {
		if r := recover(); r != nil {
			result = operations.StepHasFailed(stepName, fmt.Errorf("panic in step %s: %v", stepName, r))
		}
	}()

	switch stepName {
	case controlplanev1alpha1.StepRenewPKICerts:
		return e.renewPKICerts(ctx)
	case controlplanev1alpha1.StepSyncManifests:
		return e.syncManifests(ctx)
	case controlplanev1alpha1.StepWaitPodReady:
		return e.waitPodReady(ctx)
	case controlplanev1alpha1.StepCertObserve:
		return e.certObserve(ctx)
	default:
		return operations.StepHasFailed(stepName, fmt.Errorf("unknown step %s", stepName))
	}
}

// TODO(virtual): шаги-кандидаты по мере роста ephemeral-пайплайна:
//   - bootstrap-шаг для RBAC и т.п.
//   - генерация bootstrap-скрипта для подключения внешней ноды
// для SyncManifests заполнять в STS (аннотация) имя операции, которая привела к изменению конфига (для обсервабилити)
