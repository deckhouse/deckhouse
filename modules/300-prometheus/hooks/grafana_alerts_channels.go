/*
Copyright 2021 Flant JSC

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
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/300-prometheus/hooks/internal/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/grafana_alerts_channels",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grafana_alerts_channels",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "GrafanaAlertsChannel",
			FilterFunc: filterGrafanaAlertsChannelCRD,
		},
	},
}, grafanaAlertsChannelsHandler)

type GrafanaAlertsChannel struct {
	OrgID                 int                    `json:"org_id"`
	Type                  string                 `json:"type"`
	Name                  string                 `json:"name"`
	UID                   string                 `json:"uid"`
	IsDefault             bool                   `json:"is_default"`
	DisableResolveMessage bool                   `json:"disable_resolve_message"`
	SendReminder          bool                   `json:"send_reminder"`
	Frequency             time.Duration          `json:"frequency,omitempty"`
	Settings              map[string]interface{} `json:"settings"`
	SecureSettings        map[string]interface{} `json:"secure_settings"`
}

type GrafanaAlertsChannelsConfig struct {
	Notifiers []*GrafanaAlertsChannel `json:"notifiers"`
}

func getChannelSettings(notifyChannel *v1alpha1.GrafanaAlertsChannel) (map[string]interface{}, map[string]interface{}) {
	settings := make(map[string]interface{})
	secureSettings := make(map[string]interface{})

	if notifyChannel.Spec.Type == "PrometheusAlertManager" {
		alertManager := notifyChannel.Spec.AlertManager
		settings["url"] = alertManager.Address

		auth := alertManager.Auth
		if auth != nil {
			settings["basicAuthUser"] = auth.Basic.Username
			secureSettings["basicAuthPassword"] = auth.Basic.Password
		}
	}

	return settings, secureSettings
}

func filterGrafanaAlertsChannelCRD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var notifyChannel v1alpha1.GrafanaAlertsChannel

	err := sdk.FromUnstructured(obj, &notifyChannel)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal unstructure object %s/%s to v1alpha1.GrafanaAlertsChannel: %v", obj.GetNamespace(), obj.GetName(), err)
	}

	grafanaChannelType, ok := v1alpha1.GrafanaAlertChannelTypes[notifyChannel.Spec.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported GrafanaAlertsChannel type %s", notifyChannel.Spec.Type)
	}

	settings, securitySettings := getChannelSettings(&notifyChannel)

	return &GrafanaAlertsChannel{
		OrgID:                 1,
		Name:                  obj.GetName(),
		UID:                   obj.GetName(),
		IsDefault:             notifyChannel.Spec.IsDefault,
		Type:                  grafanaChannelType,
		DisableResolveMessage: notifyChannel.Spec.DisableResolveMessage,
		Settings:              settings,
		SecureSettings:        securitySettings,
	}, nil
}

func grafanaAlertsChannelsHandler(_ context.Context, input *go_hook.HookInput) error {
	alertsChannelsRaw := input.Snapshots.Get("grafana_alerts_channels")

	alertsChannels := make([]*GrafanaAlertsChannel, 0)

	for nchRaw, err := range sdkobjectpatch.SnapshotIter[GrafanaAlertsChannel](alertsChannelsRaw) {
		if err != nil {
			return fmt.Errorf("failed to iterate over grafana_alerts_channels: %w", err)
		}

		alertsChannels = append(alertsChannels, &nchRaw)
	}

	cfg := GrafanaAlertsChannelsConfig{
		Notifiers: alertsChannels,
	}

	input.Values.Set("prometheus.internal.grafana.alertsChannelsConfig", cfg)

	return nil
}
