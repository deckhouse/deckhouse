package ephemeral

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"strings"
)

type tenantIdentity struct {
	Name      string
	Namespace string
}

func tenantIdentityFromOperation(operation *controlplanev1alpha1.ControlPlaneOperation) tenantIdentity {
	return tenantIdentity{
		Name:      strings.TrimPrefix(operation.Namespace, constants.VirtualControlPlaneNamespacePrefix),
		Namespace: operation.Namespace,
	}
}
