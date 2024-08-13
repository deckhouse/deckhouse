/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template

import (
	"context"

	"controller/pkg/apis/deckhouse.io/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

func (m *manager) setTemplateStatus(ctx context.Context, template *v1alpha1.ProjectTemplate, message string, ready bool) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := m.client.Get(ctx, types.NamespacedName{Name: template.Name}, template); err != nil {
			return err
		}
		template.Status.Message = message
		template.Status.Ready = ready
		return m.client.Status().Update(ctx, template)
	})
}
