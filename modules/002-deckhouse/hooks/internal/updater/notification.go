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
