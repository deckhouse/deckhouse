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

package notify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/notify"
)

// token is the bearer token the webhook expects.
const token = "dh-test-secret-0123456789abcdef"

var applyAt = time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)

// notifySuite covers one announcement posted to a fake webhook.
type notifySuite struct {
	suite.Suite

	server *httptest.Server
	// status is what the webhook answers; set before a Notify.
	status int
	// received carries every request the webhook got, read after Notify returned.
	received chan received
}

// received is one request of the webhook.
type received struct {
	auth string
	data notify.Payload
}

func TestNotify(t *testing.T) {
	suite.Run(t, new(notifySuite))
}

func (s *notifySuite) SetupTest() {
	s.status = http.StatusOK
	s.received = make(chan received, 1)

	status := &s.status
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := received{auth: r.Header.Get("Authorization")}
		_ = json.NewDecoder(r.Body).Decode(&rec.data)
		s.received <- rec

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(*status)

		if *status >= 300 {
			_, _ = w.Write([]byte(`{"code":"E1","message":"rejected"}`))
		}
	}))
	s.T().Cleanup(s.server.Close)
}

func (s *notifySuite) notifier() *notify.Notifier {
	return notify.New(notify.Config{WebhookURL: s.server.URL, Auth: notify.Auth{Token: token}})
}

func (s *notifySuite) TestApplicationAnnouncement() {
	err := s.notifier().Notify(context.Background(), model.Announcement{
		Name: "ns.my.app", Kind: model.KindApplication, From: "v1.2.0", Version: "v1.3.0", ApplyAt: applyAt,
	})
	s.Require().NoError(err)

	got := <-s.received
	s.Equal("Bearer "+token, got.auth)
	s.Equal(notify.SubjectApplication, got.data.Subject)
	s.Equal("ns", got.data.Namespace)
	s.Equal("my.app", got.data.Name, "only the namespace is cut off at the first dot")
	s.Equal("v1.3.0", got.data.Version)
	s.Equal("2026-11-12T10:00:00Z", got.data.ApplyTime)
	s.Contains(got.data.Message, "v1.3.0 of application ns/my.app")
}

func (s *notifySuite) TestModuleAnnouncement() {
	err := s.notifier().Notify(context.Background(), model.Announcement{
		Name: "mod", Kind: model.KindModule, Version: "v2.0.0", ApplyAt: applyAt,
	})
	s.Require().NoError(err)

	got := <-s.received
	s.Equal(notify.SubjectModule, got.data.Subject)
	s.Equal("mod", got.data.Name)
	s.Empty(got.data.Namespace)
}

func (s *notifySuite) TestBasicAuth() {
	n := notify.New(notify.Config{WebhookURL: s.server.URL, Auth: notify.Auth{Username: "username", Password: "password"}})

	s.Require().NoError(n.Notify(context.Background(), model.Announcement{Name: "ns.app", Kind: model.KindApplication, Version: "v1.3.0"}))

	got := <-s.received
	s.Equal("Basic dXNlcm5hbWU6cGFzc3dvcmQ=", got.auth)
}

func (s *notifySuite) TestRejectedIsAnError() {
	s.status = http.StatusBadRequest

	err := s.notifier().Notify(context.Background(), model.Announcement{
		Name: "ns.app", Kind: model.KindApplication, Version: "v1.3.0", ApplyAt: applyAt,
	})

	s.Require().Error(err)
	s.Contains(err.Error(), "status code 400, code E1: rejected")
}
