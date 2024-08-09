/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template

import (
	"context"
	"controller/pkg/apis/deckhouse.io/v1alpha1"
)

func (m *manager) setTemplateStatus(ctx context.Context, template *v1alpha1.ProjectTemplate, message string, ready bool) error {
	template.Status.Message = message
	template.Status.Ready = ready
	return m.client.Status().Update(ctx, template)
}
