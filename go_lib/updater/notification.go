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
	"fmt"
	"net/http"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/tidwall/gjson"

	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

type NotificationConfig struct {
	WebhookURL              string          `json:"webhook"`
	SkipTLSVerify           bool            `json:"tlsSkipVerify"`
	MinimalNotificationTime libapi.Duration `json:"minimalNotificationTime"`
	Auth                    *Auth           `json:"auth,omitempty"`
}

type Auth struct {
	Basic *BasicAuth `json:"basic,omitempty"`
	Token *string    `json:"bearerToken,omitempty"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (a *Auth) Fill(req *http.Request) {
	if a == nil {
		return
	}
	if a.Basic != nil {
		req.SetBasicAuth(a.Basic.Username, a.Basic.Password)
		return
	}
	if a.Token != nil {
		req.Header.Set("Authorization", "Bearer "+*a.Token)
	}
}

func ParseNotificationConfigFromValues(input *go_hook.HookInput) (*NotificationConfig, error) {
	webhook, ok := input.Values.GetOk("deckhouse.update.notification.webhook")
	if !ok {
		webhook = gjson.Result{}
	}

	var minimalTime libapi.Duration
	t, ok := input.Values.GetOk("deckhouse.update.notification.minimalNotificationTime")
	if ok {
		err := json.Unmarshal([]byte(t.Raw), &minimalTime)
		if err != nil {
			return nil, fmt.Errorf("parsing minimalNotificationTime: %v", err)
		}
	}

	skipTLSVertify := input.Values.Get("deckhouse.update.notification.tlsSkipVerify").Bool()

	var auth *Auth
	a, ok := input.Values.GetOk("deckhouse.update.notification.auth")
	if ok {
		auth = &Auth{}
		err := json.Unmarshal([]byte(a.Raw), auth)
		if err != nil {
			return nil, fmt.Errorf("parsing auth: %v", err)
		}
	}

	return &NotificationConfig{
		WebhookURL:              webhook.String(),
		SkipTLSVerify:           skipTLSVertify,
		MinimalNotificationTime: minimalTime,
		Auth:                    auth,
	}, nil
}

func sendWebhookNotification(config *NotificationConfig, data webhookData) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.SkipTLSVerify},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	buf := bytes.NewBuffer(nil)
	_ = json.NewEncoder(buf).Encode(data)

	req, err := http.NewRequest(http.MethodPost, config.WebhookURL, buf)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	config.Auth.Fill(req)

	for i := 0; i < 3; i++ {
		_, err = client.Do(req)
		if err == nil {
			return nil
		}
		time.Sleep(3 * time.Second)
	}

	return err
}

type webhookData struct {
	Version       string            `json:"version"`
	Requirements  map[string]string `json:"requirements,omitempty"`
	ChangelogLink string            `json:"changelogLink"`
	ApplyTime     string            `json:"applyTime,omitempty"`

	Message string `json:"message"`
}
