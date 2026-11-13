// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package notify sends the update notifications of packages to the webhook of the notification
// settings (applications.update.notification); the package Reconciler decides when.
package notify

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// Subjects of a notification: what kind of package is updated.
const (
	SubjectApplication = "Application" // SubjectApplication announces an update of an Application.
	SubjectModule      = "Module"      // SubjectModule announces an update of a Module.
)

const (
	// timeout bounds one request; the package Reconciler retries a failure.
	timeout = 10 * time.Second
	// maxErrorBody is the most of a rejection's body read for its error message.
	maxErrorBody = 4096
)

// Config is the webhook of the notification settings.
type Config struct {
	WebhookURL    string
	SkipTLSVerify bool
	Auth          Auth
}

// Auth authenticates the request: basic auth when Username is set, else a bearer Token when set.
type Auth struct {
	Username string
	Password string
	Token    string
}

// Payload is the body of a notification.
type Payload struct {
	Subject   string `json:"subject"`
	Name      string `json:"name"`                // Name is the application or module name.
	Namespace string `json:"namespace,omitempty"` // Namespace is the application's; empty for a module.
	Version   string `json:"version"`
	ApplyTime string `json:"applyTime,omitempty"`
	Message   string `json:"message"`
}

// rejection is the error body a webhook may answer with.
type rejection struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

// Notifier sends announcements to one webhook; it satisfies model.Notifier. Safe for concurrent use.
type Notifier struct {
	config Config
	client *http.Client
}

// New returns a Notifier of the webhook in config.
func New(config Config) *Notifier {
	return &Notifier{
		config: config,
		client: &http.Client{
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: config.SkipTLSVerify}}, //nolint:gosec // the user's choice
			Timeout:   timeout,
		},
	}
}

// Notify posts one announcement, once: the package Reconciler retries a failure with backoff.
func (n *Notifier) Notify(ctx context.Context, a model.Announcement) error {
	if err := n.post(ctx, payloadOf(a)); err != nil {
		return fmt.Errorf("post update notification: %w", err)
	}

	return nil
}

// payloadOf builds the body of an announcement.
func payloadOf(a model.Announcement) Payload {
	p := Payload{Version: a.Version, ApplyTime: a.ApplyAt.UTC().Format(time.RFC3339)}
	applyAt := a.ApplyAt.UTC().Format(time.RFC850)

	if a.Kind == model.KindApplication {
		// The package name is namespace.name, and a namespace has no dots.
		namespace, name, _ := strings.Cut(a.Name, ".")

		p.Subject, p.Namespace, p.Name = SubjectApplication, namespace, name
		p.Message = fmt.Sprintf("New version %s of application %s/%s is available. The update will be applied at: %s",
			a.Version, namespace, name, applyAt)

		return p
	}

	p.Subject, p.Name = SubjectModule, a.Name
	p.Message = fmt.Sprintf("New version %s of module %s is available. The update will be applied at: %s",
		a.Version, a.Name, applyAt)

	return p
}

// post sends p; an error is a failed request or a non-2xx answer, with the webhook's reason.
func (n *Notifier) post(ctx context.Context, p Payload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.config.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	switch auth := n.config.Auth; {
	case auth.Username != "":
		req.SetBasicAuth(auth.Username, auth.Password)
	case auth.Token != "":
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return rejected(resp)
}

// rejected is the error of a non-2xx answer, with the code and message of its body when it has them.
func rejected(resp *http.Response) error {
	msg := fmt.Sprintf("webhook answered with status code %d", resp.StatusCode)

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil || len(raw) == 0 {
		return errors.New(msg)
	}

	var r rejection
	if err := json.Unmarshal(raw, &r); err != nil {
		return fmt.Errorf("%s: %s", msg, strings.TrimSpace(string(raw)))
	}

	if r.Code != "" {
		msg += ", code " + r.Code
	}

	if r.Message != "" {
		msg += ": " + r.Message
	}

	return errors.New(msg)
}
