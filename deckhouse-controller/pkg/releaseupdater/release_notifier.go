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

package releaseupdater

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

const (
	SubjectDeckhouse = "Deckhouse"
	SubjectModule    = "Module"
)

type ReleaseNotifier struct {
	settings *Settings
}

func NewReleaseNotifier(settings *Settings) *ReleaseNotifier {
	return &ReleaseNotifier{
		settings: settings,
	}
}

type WebhookData struct {
	Subject       string            `json:"subject"`
	Version       string            `json:"version"`
	Requirements  map[string]string `json:"requirements,omitempty"`
	ChangelogLink string            `json:"changelogLink,omitempty"`

	ApplyTime string `json:"applyTime,omitempty"`
	Message   string `json:"message"`
}

type ResponseError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

const (
	MaxContentLen = 4096
)

func (u *ReleaseNotifier) SendPatchReleaseNotification(ctx context.Context, release v1alpha1.Release, applyTime time.Time, metricLabels MetricLabels) error {
	if release.GetNotified() {
		return nil
	}

	if !u.settings.NotificationConfig.IsEmpty() && u.settings.NotificationConfig.ReleaseType == ReleaseTypeAll {
		metricLabels.SetFalse(NotificationNotSent)

		err := u.sendReleaseNotification(ctx, release, applyTime)
		if err != nil {
			metricLabels.SetTrue(NotificationNotSent)
			return fmt.Errorf("send release notification: %w", err)
		}
	}
	return nil
}

func (u *ReleaseNotifier) SendMinorReleaseNotification(ctx context.Context, release v1alpha1.Release, applyTime time.Time, metricLabels MetricLabels) error {
	if release.GetNotified() {
		return nil
	}

	if !u.settings.NotificationConfig.IsEmpty() {
		metricLabels.SetFalse(NotificationNotSent)

		err := u.sendReleaseNotification(ctx, release, applyTime)
		if err != nil {
			metricLabels.SetTrue(NotificationNotSent)
			return fmt.Errorf("send release notification: %w", err)
		}
	}
	return nil
}

func (u *ReleaseNotifier) sendReleaseNotification(ctx context.Context, release v1alpha1.Release, applyTime time.Time) error {
	if u.settings.NotificationConfig.WebhookURL == "" {
		return nil
	}

	data := &WebhookData{
		Version:       release.GetVersion().String(),
		Requirements:  release.GetRequirements(),
		ChangelogLink: release.GetChangelogLink(),
		ApplyTime:     applyTime.Format(time.RFC3339),
		Subject:       u.settings.Subject,
		Message:       fmt.Sprintf("New Deckhouse Release %s is available. Release will be applied at: %s", release.GetVersion().String(), applyTime.Format(time.RFC850)),
	}

	err := sendWebhookNotification(ctx, u.settings.NotificationConfig, data)
	if err != nil {
		return fmt.Errorf("send webhook notification: %w", err)
	}

	return nil
}

func sendWebhookNotification(ctx context.Context, config NotificationConfig, data *WebhookData) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: config.SkipTLSVerify},
		},
		Timeout: 10 * time.Second,
	}

	buf := bytes.NewBuffer(nil)

	retryBackoff := 2 * time.Second
	if config.RetryMinTime.Duration > 0 {
		retryBackoff = config.RetryMinTime.Duration
	}

	_, err := retry(5, retryBackoff, func() (*http.Response, error) {
		defer buf.Reset()

		err := json.NewEncoder(buf).Encode(data)
		if err != nil {
			return nil, err
		}
		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, http.MethodPost, config.WebhookURL, buf)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Content-Type", "application/json")
		config.Auth.Fill(req)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		var responseError ResponseError

		if resp.ContentLength > 0 && resp.ContentLength <= MaxContentLen {

			decoder := json.NewDecoder(resp.Body)
			if err := decoder.Decode(&responseError); err != nil {

				return nil, fmt.Errorf("webhook returned error status %d", resp.StatusCode)
			}

			if responseError.Code != "" {
				return nil, fmt.Errorf("webhook error [%s]: %s", responseError.Code, responseError.Message)
			}
			return nil, fmt.Errorf("webhook error: %s", responseError.Message)
		}

		return nil, fmt.Errorf("webhook returned error status %d", resp.StatusCode)
	})

	return err
}

func retry[T any](attempts int, sleep time.Duration, f func() (T, error)) (T, error) {
	var err error
	var result T

	for i := 0; i < attempts; i++ {
		result, err = f()
		if err == nil {
			return result, nil
		}

		time.Sleep(sleep)
		sleep *= 2
	}

	return result, fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

type ReleaseType string

const (
	ReleaseTypeMinor ReleaseType = "Minor"
	ReleaseTypeAll   ReleaseType = "All"
)

type NotificationConfig struct {
	WebhookURL              string          `json:"webhook"`
	SkipTLSVerify           bool            `json:"tlsSkipVerify"`
	MinimalNotificationTime libapi.Duration `json:"minimalNotificationTime"`
	RetryMinTime            libapi.Duration `json:"retryMinTime"`
	Auth                    *Auth           `json:"auth,omitempty"`
	ReleaseType             ReleaseType     `json:"releaseType"`
}

func (cfg *NotificationConfig) IsEmpty() bool {
	return cfg != nil && *cfg == NotificationConfig{}
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
