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

	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

type ReleaseType string

const (
	ReleaseTypeMinor ReleaseType = "Minor"
	ReleaseTypeAll   ReleaseType = "All"
)

type NotificationConfig struct {
	WebhookURL              string          `json:"webhook"`
	SkipTLSVerify           bool            `json:"tlsSkipVerify"`
	MinimalNotificationTime libapi.Duration `json:"minimalNotificationTime"`
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

func sendWebhookNotification(config NotificationConfig, data WebhookData) error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.SkipTLSVerify},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	var err error
	for i := 0; i < 3; i++ {
		// We can only read the buffer once, so for each attempt we need to create a new one
		buf := bytes.NewBuffer(nil)
		_ = json.NewEncoder(buf).Encode(data)

		var req *http.Request
		req, err = http.NewRequest(http.MethodPost, config.WebhookURL, buf)
		if err != nil {
			return err
		}
		req.Header.Add("Content-Type", "application/json")
		config.Auth.Fill(req)

		_, err = client.Do(req)
		if err == nil {
			return nil
		}
		time.Sleep(3 * time.Second)
	}

	return err
}

type WebhookData struct {
	Subject       string            `json:"subject"`
	Version       string            `json:"version"`
	Requirements  map[string]string `json:"requirements,omitempty"`
	ChangelogLink string            `json:"changelogLink,omitempty"`
	ModuleName    string            `json:"moduleName,omitempty"`

	ApplyTime string `json:"applyTime,omitempty"`
	Message   string `json:"message"`
}
