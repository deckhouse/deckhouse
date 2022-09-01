/*
Copyright 2022 Flant JSC

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

package updater

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	"github.com/deckhouse/deckhouse/modules/020-deckhouse/hooks/internal/apis/v1alpha1"
)

type NotificationConfig struct {
	WebhookURL              string
	MinimalNotificationTime v1alpha1.Duration
}

func ParseNotificationConfigFromValues(input *go_hook.HookInput) *NotificationConfig {
	webhook, ok := input.Values.GetOk("deckhouse.update.notification.webhook")
	if !ok {
		return nil
	}

	var minimalTime v1alpha1.Duration
	t, ok := input.Values.GetOk("deckhouse.update.notification.minimalNotificationTime")
	if ok {
		err := json.Unmarshal([]byte(t.Raw), &minimalTime)
		if err != nil {
			panic(err)
		}
	}

	return &NotificationConfig{
		WebhookURL:              webhook.String(),
		MinimalNotificationTime: minimalTime,
	}
}

func sendWebhookNotification(webhookURL string, data webhookData) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	buf := bytes.NewBuffer(nil)
	_ = json.NewEncoder(buf).Encode(data)

	_, err := client.Post(webhookURL, "application/json", buf)

	return err
}

type webhookData struct {
	Version       string            `json:"version"`
	Requirements  map[string]string `json:"requirements,omitempty"`
	ChangelogLink string            `json:"changelogLink"`
	ApplyTime     string            `json:"applyTime,omitempty"`

	Message string `json:"message"`
}
