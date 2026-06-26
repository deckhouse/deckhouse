package ephemeral

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StepExecutor struct {
	client         client.Client
	operation      *controlplanev1alpha1.ControlPlaneOperation
	tenantIdentity tenantIdentity
}

func (e *StepExecutor) Execute(ctx context.Context, stepName controlplanev1alpha1.StepName) (result operations.StepResult) {
	defer func() {
		if r := recover(); r != nil {
			result = operations.StepHasFailed(stepName, fmt.Errorf("panic in step %s: %v", stepName, r))
		}
	}()

	switch stepName {
	case controlplanev1alpha1.StepSyncManifests:
		return e.syncManifests(ctx)
	case controlplanev1alpha1.StepWaitPodReady:
		return e.waitPodReady(ctx)
	default:
		return operations.StepHasFailed(stepName, fmt.Errorf("unknown step %s", stepName))
	}
}

// TODO(virtual): шаги-кандидаты по мере роста ephemeral-пайплайна:
//   - bootstrap-шаг для RBAC и т.п.
//   - генерация bootstrap-скрипта для подключения внешней ноды
//   - SyncCA / RenewPKICerts / RenewKubeconfigs / CertObserve (после решения по владельцу PKI)
