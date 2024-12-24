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

package d8updater

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/updater"
)

type ReleaseNotifier struct {
	settings *updater.Settings
}

func NewReleaseNotifier() *ReleaseNotifier {
	return &ReleaseNotifier{}
}

type WebhookData struct {
	Subject       string            `json:"subject"`
	Version       string            `json:"version"`
	Requirements  map[string]string `json:"requirements,omitempty"`
	ChangelogLink string            `json:"changelogLink,omitempty"`

	ApplyTime string `json:"applyTime,omitempty"`
	Message   string `json:"message"`
}

func (u *ReleaseNotifier) sendReleaseNotification(dr *v1alpha1.DeckhouseRelease, applyTime time.Time) error {
	// // check it before calling sendReleaseNotification
	// if u.releaseData.Notified {
	// 	return nil
	// }

	if u.settings.NotificationConfig.WebhookURL == "" {
		return nil
	}

	data := &WebhookData{
		Version:       dr.GetVersion().String(),
		Requirements:  dr.GetRequirements(),
		ChangelogLink: dr.GetChangelogLink(),
		ApplyTime:     applyTime.Format(time.RFC3339),
		Subject:       updater.SubjectDeckhouse,
		Message:       fmt.Sprintf("New Deckhouse Release %s is available. Release will be applied at: %s", dr.GetVersion().String(), applyTime.Format(time.RFC850)),
	}

	err := sendWebhookNotification(u.settings.NotificationConfig, data)
	if err != nil {
		return fmt.Errorf("send release notification failed: %w", err)
	}

	// // set it after calling sendReleaseNotification
	// err := u.changeNotifiedFlag(true)
	// if err != nil {
	// 	return fmt.Errorf("change notified flag: %w", err)
	// }

	return nil
}

func sendWebhookNotification(config updater.NotificationConfig, data *WebhookData) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: config.SkipTLSVerify},
		},
		Timeout: 10 * time.Second,
	}

	var err error
	retry(5, 2*time.Second, func() (*http.Response, error) {
		// We can only read the buffer once, so for each attempt we need to create a new one
		buf := bytes.NewBuffer(nil)
		_ = json.NewEncoder(buf).Encode(data)

		var req *http.Request
		req, err = http.NewRequest(http.MethodPost, config.WebhookURL, buf)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")
		config.Auth.Fill(req)

		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		time.Sleep(3 * time.Second)
	})

	return err
}

func retry[T any](attempts int, sleep time.Duration, f func() (T, error)) (result T, err error) {
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(sleep)
			sleep *= 2
		}

		result, err = f()
		if err == nil {
			return result, nil
		}
	}

	return result, fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}
