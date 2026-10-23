/*
Copyright 2026 Flant JSC

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

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

var devicesNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

const (
	oldBrowser = "oldbrowseraaaaaaaaaaaaaaaa"
	newBrowser = "newbrowseraaaaaaaaaaaaaaaa"
	oldGrant   = "oldgrantaaaaaaaaaaaaaaaaaa"
	newGrant   = "newgrantaaaaaaaaaaaaaaaaaa"
	// knownBrowser signed in again an hour ago; Dex has known the browser for three days.
	knownBrowser = "knownbrowseraaaaaaaaaaaaaa"
)

// testDevices are two browsers and two devices without one, signed in two days and an hour ago, and
// a browser signed in again an hour ago that Dex has known for three days.
func testDevices() k8s.Devices {
	at := func(d time.Duration) *time.Time { t := devicesNow.Add(-d); return &t }
	return k8s.Devices{Items: []k8s.Device{
		{ID: newBrowser, Kind: k8s.DeviceKindBrowser, Clients: []string{"console"}, SignedInAt: at(time.Hour)},
		{ID: newGrant, Kind: k8s.DeviceKindGrant, Clients: []string{"kubeconfig-generator"}, SignedInAt: at(time.Hour)},
		{ID: oldBrowser, Kind: k8s.DeviceKindBrowser, Clients: []string{"console", "grafana"}, SignedInAt: at(48 * time.Hour)},
		{ID: oldGrant, Kind: k8s.DeviceKindGrant, Clients: []string{"kubeconfig-generator"}, SignedInAt: at(48 * time.Hour)},
		{ID: knownBrowser, Kind: k8s.DeviceKindBrowser, Clients: []string{"console"}, SignedInAt: at(time.Hour), KnownSince: at(72 * time.Hour)},
	}}
}

// withDevice is jane on the device of the sid and d8_gid claims given.
func withDevice(claims *auth.Claims, sessionID, grantID string) *auth.Claims {
	c := *claims
	c.SessionID, c.GrantID = sessionID, grantID
	return &c
}

func newDevicesHandler(claims *auth.Claims, client *mockK8sClient) *Handler {
	h := newTestHandler(&mockVerifier{claims: claims}, client)
	h.now = func() time.Time { return devicesNow }
	return h
}

func TestHandler_SelfDevices(t *testing.T) {
	tests := []struct {
		name        string
		claims      *auth.Claims
		client      *mockK8sClient
		wantStatus  int
		wantCurrent string
		wantCount   int
	}{
		{
			name:        "the browser of the token is the current device",
			claims:      withDevice(janeClaims, oldBrowser, "agrantofthatbrowseraaaaaaa"),
			client:      &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: testDevices()},
			wantStatus:  http.StatusOK,
			wantCurrent: oldBrowser,
			wantCount:   5,
		},
		{
			name:        "without a browser the refresh token is the current device",
			claims:      withDevice(jdoeClaims, "", oldGrant),
			client:      &mockK8sClient{devices: testDevices()},
			wantStatus:  http.StatusOK,
			wantCurrent: oldGrant,
			wantCount:   5,
		},
		{
			name:       "no account yet",
			claims:     janeClaims,
			client:     &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: k8s.Devices{Items: []k8s.Device{}}},
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown local user",
			claims:     janeClaims,
			client:     &mockK8sClient{},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "the account cannot be read",
			claims:     janeClaims,
			client:     &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devicesErr: errors.New("boom")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			// The name of the account is a hash of the pair: one of another pair is not listed.
			name:   "the account belongs to another pair",
			claims: janeClaims,
			client: &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: func() k8s.Devices {
				d := testDevices()
				d.UserID, d.ConnectorID = "john", "local"
				return d
			}()},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newDevicesHandler(tt.claims, tt.client)
			req := httptest.NewRequest(http.MethodGet, devicesEndpoint, nil)
			req.Header.Set("Authorization", "Bearer validtoken")
			w := httptest.NewRecorder()
			h.SelfDevices(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus != http.StatusOK {
				return
			}
			var resp DevicesResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Sessions == nil || len(resp.Sessions) != tt.wantCount {
				t.Fatalf("sessions = %+v, want %d", resp.Sessions, tt.wantCount)
			}
			var current []string
			for _, s := range resp.Sessions {
				if s.Current {
					current = append(current, s.ID)
				}
			}
			if tt.wantCurrent == "" && len(current) != 0 || tt.wantCurrent != "" && !reflect.DeepEqual(current, []string{tt.wantCurrent}) {
				t.Errorf("current = %v, want %q", current, tt.wantCurrent)
			}
			if resp.MinDeviceAgeSeconds != int64(DefaultMinDeviceAge/time.Second) {
				t.Errorf("minDeviceAgeSeconds = %d, want a day", resp.MinDeviceAgeSeconds)
			}
			if len(tt.client.createdOperations()) != 0 {
				t.Error("listing the devices created an operation")
			}
		})
	}
}

// The minimum device age is configured: the list tells the console, and the actions follow it. Zero
// lets any device act at once, and the message names the age.
func TestHandler_MinDeviceAge(t *testing.T) {
	tests := []struct {
		name        string
		age         time.Duration
		wantSeconds string
		wantStatus  int
		wantMessage string
	}{
		{name: "a day refuses a device of an hour", age: DefaultMinDeviceAge, wantSeconds: "86400", wantStatus: http.StatusForbidden,
			wantMessage: "possible 24h after the first sign-in from this browser"},
		{name: "an hour and a half refuses it too", age: 90 * time.Minute, wantSeconds: "5400", wantStatus: http.StatusForbidden,
			wantMessage: "possible 1h30m after the first sign-in from this browser"},
		{name: "half an hour lets it", age: 30 * time.Minute, wantSeconds: "1800", wantStatus: http.StatusOK},
		{name: "zero lets it", age: 0, wantSeconds: "0", wantStatus: http.StatusOK},
	}
	jane := func() *mockK8sClient {
		return &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: testDevices(), statuses: succeeded("")}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := withDevice(janeClaims, newBrowser, "")

			h := newDevicesHandler(claims, jane())
			h.SetMinDeviceAge(tt.age)
			req := httptest.NewRequest(http.MethodGet, devicesEndpoint, nil)
			req.Header.Set("Authorization", "Bearer validtoken")
			w := httptest.NewRecorder()
			h.SelfDevices(w, req)
			if !strings.Contains(w.Body.String(), `"minDeviceAgeSeconds":`+tt.wantSeconds) {
				t.Errorf("list = %s, want minDeviceAgeSeconds %s", w.Body.String(), tt.wantSeconds)
			}

			h = newDevicesHandler(claims, jane())
			h.SetMinDeviceAge(tt.age)
			w = httptest.NewRecorder()
			h.SelfLogoutOthers(w, selfPost(logoutOthersEndpoint))
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantMessage != "" && !strings.Contains(w.Body.String(), tt.wantMessage) {
				t.Errorf("body = %s, want %q", w.Body.String(), tt.wantMessage)
			}
		})
	}
}

func TestHandler_SelfDevicesHasABudgetApartFromActions(t *testing.T) {
	client := &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: testDevices(),
		statuses: succeeded("")}
	h := newDevicesHandler(withDevice(janeClaims, oldBrowser, ""), client)

	for i := range userRequestBurst + 1 {
		req := httptest.NewRequest(http.MethodGet, devicesEndpoint, nil)
		req.Header.Set("Authorization", "Bearer validtoken")
		w := httptest.NewRecorder()
		h.SelfDevices(w, req)
		if want := http.StatusOK; w.Code != want {
			t.Fatalf("read %d: status = %d, want %d", i+1, w.Code, want)
		}
	}

	w := httptest.NewRecorder()
	h.SelfLogoutOthers(w, selfPost(logoutOthersEndpoint))
	if w.Code != http.StatusOK {
		t.Errorf("an action after the reads: status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// TestHandler_EveryDeviceHasABudgetOfItsOwn: a device signed in an hour ago that uses up its budget,
// for the actions and for the reads, leaves the budget of the device of two days whole: the owner
// still lists the devices and signs the new one out.
func TestHandler_EveryDeviceHasABudgetOfItsOwn(t *testing.T) {
	client := &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: testDevices(),
		statuses: succeeded("")}
	verifier := &mockVerifier{claims: withDevice(janeClaims, newBrowser, "")}
	h := newTestHandler(verifier, client)

	for i := range userRequestBurst {
		w := httptest.NewRecorder()
		h.SelfLogoutOthers(w, selfPost(logoutOthersEndpoint))
		if w.Code != http.StatusForbidden {
			t.Fatalf("new device, action %d: status = %d, want %d", i+1, w.Code, http.StatusForbidden)
		}
	}
	w := httptest.NewRecorder()
	h.SelfLogoutOthers(w, selfPost(logoutOthersEndpoint))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("new device past its action budget: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	for range userReadBurst + 1 {
		w = httptest.NewRecorder()
		h.SelfDevices(w, selfGet(devicesEndpoint))
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("new device past its read budget: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	verifier.claims = withDevice(janeClaims, oldBrowser, "")
	w = httptest.NewRecorder()
	h.SelfDevices(w, selfGet(devicesEndpoint))
	if w.Code != http.StatusOK {
		t.Fatalf("old device, list: status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
	req := selfPost("/api/v1/self/sessions/" + newBrowser + "/logout")
	req.SetPathValue("id", newBrowser)
	w = httptest.NewRecorder()
	h.SelfDeviceLogout(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("old device signs the new one out: status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_SelfDeviceLogout(t *testing.T) {
	logout := func(user string, l *k8s.Logout) *k8s.UserOperation {
		return &k8s.UserOperation{GenerateName: "self-logout-device-", Type: k8s.OperationTypeLogout, User: user, Logout: l}
	}
	jane := func() *mockK8sClient {
		return &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: testDevices(), statuses: succeeded("")}
	}

	tests := []struct {
		name           string
		claims         *auth.Claims
		client         *mockK8sClient
		id             string
		wantStatus     int
		wantError      string
		wantRetryAfter string
		wantOperation  *k8s.UserOperation
	}{
		{
			name:          "another browser from a device of two days",
			claims:        withDevice(janeClaims, oldBrowser, ""),
			client:        jane(),
			id:            newBrowser,
			wantStatus:    http.StatusOK,
			wantOperation: logout("jane", &k8s.Logout{Session: newBrowser}),
		},
		{
			name:          "a kubeconfig from a device of two days",
			claims:        withDevice(janeClaims, oldBrowser, ""),
			client:        jane(),
			id:            newGrant,
			wantStatus:    http.StatusOK,
			wantOperation: logout("jane", &k8s.Logout{Grant: newGrant}),
		},
		{
			name:          "the device of the request, however young",
			claims:        withDevice(janeClaims, newBrowser, ""),
			client:        jane(),
			id:            newBrowser,
			wantStatus:    http.StatusOK,
			wantOperation: logout("jane", &k8s.Logout{Session: newBrowser}),
		},
		{
			name:           "another device from a device of an hour",
			claims:         withDevice(janeClaims, newBrowser, ""),
			client:         jane(),
			id:             oldBrowser,
			wantStatus:     http.StatusForbidden,
			wantError:      "too_early",
			wantRetryAfter: "82800",
		},
		{
			name:       "a device the user does not have",
			claims:     withDevice(janeClaims, oldBrowser, ""),
			client:     jane(),
			id:         "someoneelsesbrowseraaaaaaa",
			wantStatus: http.StatusNotFound,
			wantError:  "not_found",
		},
		{
			name:       "an ID that is no device ID",
			claims:     withDevice(janeClaims, oldBrowser, ""),
			client:     &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: k8s.Devices{Items: []k8s.Device{{ID: "Bad-ID", Kind: k8s.DeviceKindGrant}}}},
			id:         "Bad-ID",
			wantStatus: http.StatusNotFound,
			wantError:  "not_found",
		},
		{
			name:       "an external user",
			claims:     withDevice(jdoeClaims, "", oldGrant),
			client:     &mockK8sClient{devices: testDevices(), statuses: succeeded("")},
			id:         newGrant,
			wantStatus: http.StatusOK,
			wantOperation: &k8s.UserOperation{
				GenerateName: "self-logout-device-",
				Type:         k8s.OperationTypeLogout,
				Target:       &k8s.Target{Email: jdoeClaims.Email, ConnectorID: "ldap", UserID: jdoeClaims.UserID},
				Logout:       &k8s.Logout{Grant: newGrant},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newDevicesHandler(tt.claims, tt.client)
			req := selfPost("/api/v1/self/sessions/" + tt.id + "/logout")
			req.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()
			h.SelfDeviceLogout(w, req)

			assertDeviceAction(t, w, tt.client, tt.wantStatus, tt.wantError, tt.wantRetryAfter, tt.wantOperation)
		})
	}
}

func TestHandler_SelfLogoutOthers(t *testing.T) {
	others := func(l *k8s.Logout) *k8s.UserOperation {
		return &k8s.UserOperation{GenerateName: "self-logout-others-", Type: k8s.OperationTypeLogout, User: "jane", Logout: l}
	}
	jane := func() *mockK8sClient {
		return &mockK8sClient{usernames: map[string]string{"jane": "jane"}, devices: testDevices(), statuses: succeeded("")}
	}

	tests := []struct {
		name           string
		claims         *auth.Claims
		client         *mockK8sClient
		wantStatus     int
		wantError      string
		wantRetryAfter string
		wantOperation  *k8s.UserOperation
	}{
		{
			name:          "from a browser of two days",
			claims:        withDevice(janeClaims, oldBrowser, "agrantofthatbrowseraaaaaaa"),
			client:        jane(),
			wantStatus:    http.StatusOK,
			wantOperation: others(&k8s.Logout{ExceptSession: oldBrowser}),
		},
		{
			name:          "from a device of two days without a browser",
			claims:        withDevice(janeClaims, "", oldGrant),
			client:        jane(),
			wantStatus:    http.StatusOK,
			wantOperation: others(&k8s.Logout{ExceptGrant: oldGrant}),
		},
		{
			name:          "from a refresh token whose browser session has ended",
			claims:        withDevice(janeClaims, "anendedbrowseraaaaaaaaaaaa", oldGrant),
			client:        jane(),
			wantStatus:    http.StatusOK,
			wantOperation: others(&k8s.Logout{ExceptGrant: oldGrant}),
		},
		{
			name:           "from a device of an hour",
			claims:         withDevice(janeClaims, newBrowser, ""),
			client:         jane(),
			wantStatus:     http.StatusForbidden,
			wantError:      "too_early",
			wantRetryAfter: "82800",
		},
		{
			name:          "from a browser signed in again an hour ago and known for three days",
			claims:        withDevice(janeClaims, knownBrowser, ""),
			client:        jane(),
			wantStatus:    http.StatusOK,
			wantOperation: others(&k8s.Logout{ExceptSession: knownBrowser}),
		},
		{
			name:       "from a device not listed yet",
			claims:     withDevice(janeClaims, "justsignedinaaaaaaaaaaaaaa", ""),
			client:     jane(),
			wantStatus: http.StatusForbidden,
			wantError:  "too_early",
		},
		{
			name:       "from a token that names no device",
			claims:     withDevice(janeClaims, "", ""),
			client:     jane(),
			wantStatus: http.StatusForbidden,
			wantError:  "forbidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newDevicesHandler(tt.claims, tt.client)
			w := httptest.NewRecorder()
			h.SelfLogoutOthers(w, selfPost(logoutOthersEndpoint))

			assertDeviceAction(t, w, tt.client, tt.wantStatus, tt.wantError, tt.wantRetryAfter, tt.wantOperation)
		})
	}
}

// selfPost is a self-service POST with a token and an empty body.
func selfPost(target string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, nil)
	req.Header.Set("Authorization", "Bearer validtoken")
	return req
}

func selfGet(target string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer validtoken")
	return req
}

func assertDeviceAction(
	t *testing.T,
	w *httptest.ResponseRecorder,
	client *mockK8sClient,
	wantStatus int,
	wantError string,
	wantRetryAfter string,
	wantOperation *k8s.UserOperation,
) {
	t.Helper()

	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, wantStatus, w.Body.String())
	}
	if got := w.Header().Get("Retry-After"); got != wantRetryAfter {
		t.Errorf("Retry-After = %q, want %q", got, wantRetryAfter)
	}
	created := client.createdOperations()
	switch {
	case wantOperation == nil && len(created) != 0:
		t.Errorf("created %+v, want no operation", created)
	case wantOperation != nil && (len(created) != 1 || !reflect.DeepEqual(created[0], *wantOperation)):
		t.Errorf("created %+v, want %+v", created, *wantOperation)
	}
	if wantError == "" {
		return
	}
	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error != wantError {
		t.Errorf("error = %q, want %q", resp.Error, wantError)
	}
}
