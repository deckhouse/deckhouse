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
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

const (
	devicesEndpoint = "/api/v1/self/sessions"
	// deviceLogoutEndpoint is the pattern of the route: the metrics count it whatever the device.
	deviceLogoutEndpoint = "/api/v1/self/sessions/{id}/logout"
	logoutOthersEndpoint = "/api/v1/self/logout-others"

	// DefaultMinDeviceAge is how long the device of a request must have been signed in before it may
	// act on the other devices or the account, unless configured (SetMinDeviceAge): sign out
	// everywhere, of the other devices or of another one, or change the password. A thief who has
	// just signed in with a stolen session or password cannot sign the owner out or take the account
	// over, while the owner's older devices can end the new one. A new device may sign out only of
	// itself; an administrator is not limited.
	DefaultMinDeviceAge = 24 * time.Hour

	// noDeviceMessage answers the logout of a device the user does not have. It does not claim a
	// sign-out: the device may be gone, or it may be someone else's.
	noDeviceMessage = "the user has no such device"
	// noSessionMessage is how the controller completes the Logout of a session the user does not have
	// (noSessionMessage of its useroperation package).
	noSessionMessage = "the user has no such session"
)

// deviceIDPattern is the form of the ID of a browser session and of a refresh token of Dex.
var deviceIDPattern = regexp.MustCompile(`^[a-z2-7]{1,64}$`)

// DeviceResponse is a device of the user; Current marks the device of the request.
type DeviceResponse struct {
	k8s.Device
	Current bool `json:"current,omitempty"`
}

// DevicesResponse lists the devices of the user, the most recently active first. Truncated tells
// that the user has more devices than listed. MinDeviceAgeSeconds is how long the device of a
// request must have been signed in to act on the others and on the password; 0 when any may.
type DevicesResponse struct {
	Sessions            []DeviceResponse `json:"sessions"`
	Truncated           bool             `json:"truncated,omitempty"`
	MinDeviceAgeSeconds int64            `json:"minDeviceAgeSeconds"`
}

// SelfDevices lists the devices signed in to the account of the user, from its UserAccount.
func (h *Handler) SelfDevices(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.beginSelf(w, r, devicesEndpoint, h.readLimiter)
	if !ok {
		return
	}
	devices, ok := h.devices(w, r, devicesEndpoint, claims)
	if !ok {
		return
	}

	current, _ := currentDevice(devices.Items, claims)
	resp := DevicesResponse{
		Sessions:            make([]DeviceResponse, 0, len(devices.Items)),
		Truncated:           devices.Truncated,
		MinDeviceAgeSeconds: int64(h.minDeviceAge / time.Second),
	}
	for _, d := range devices.Items {
		resp.Sessions = append(resp.Sessions, DeviceResponse{Device: d, Current: d.ID == current.ID && d.Kind == current.Kind})
	}
	h.count(devicesEndpoint, http.StatusOK)
	h.writeJSON(w, http.StatusOK, resp)
}

// SelfDeviceLogout signs the user out of one device: a browser session of Dex, or a refresh token
// issued without one. A device the user does not have is signed out already: it expired, was signed
// out of before, or is someone else's, and the answer does not tell these apart.
func (h *Handler) SelfDeviceLogout(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.beginSelfRequest(w, r, deviceLogoutEndpoint)
	if !ok {
		return
	}
	devices, ok := h.devices(w, r, deviceLogoutEndpoint, claims)
	if !ok {
		return
	}

	id := r.PathValue("id")
	if !deviceIDPattern.MatchString(id) {
		h.fail(w, deviceLogoutEndpoint, http.StatusNotFound, "not_found", "No such device")
		return
	}
	target, found := findDevice(devices.Items, id)
	if !found {
		h.logoutUnlisted(w, r, claims, devices, id)
		return
	}
	current, known := currentDevice(devices.Items, claims)
	isCurrent := known && current.ID == target.ID && current.Kind == target.Kind
	if !isCurrent && !h.oldEnough(w, deviceLogoutEndpoint, current, known) {
		return
	}

	logout := &k8s.Logout{Session: target.ID}
	if target.Kind == k8s.DeviceKindGrant {
		logout = &k8s.Logout{Grant: target.ID}
	}
	op, ok := h.logoutOperation(w, r, deviceLogoutEndpoint, claims, "self-logout-device-", logout)
	if !ok {
		return
	}
	h.runOperation(w, r, deviceLogoutEndpoint, claims, op, []string{target.ID})
}

// logoutUnlisted signs the user out of the device id their list does not have. The device of the
// request may be missing from it: it signed in moments ago, or the list is cut at the most recently
// active devices. A list that is cut may miss any other device too, of either kind: a Logout of a
// browser session and one of a grant are created at once, so that a slow controller cannot leave a
// grant signed in, and are waited for within one waitTimeout, so that the answer comes as soon as for
// one operation. Any other ID is not the user's device: it is gone, or someone else's, and the answer
// is the same for both.
func (h *Handler) logoutUnlisted(w http.ResponseWriter, r *http.Request, claims *auth.Claims, devices k8s.Devices, id string) {
	switch {
	case id == claims.SessionID:
		h.logoutDevice(w, r, claims, &k8s.Logout{Session: id}, id)
	case id == claims.GrantID:
		h.logoutDevice(w, r, claims, &k8s.Logout{Grant: id}, id)
	case devices.Truncated:
		current, known := currentDevice(devices.Items, claims)
		if !h.oldEnough(w, deviceLogoutEndpoint, current, known) {
			return
		}
		h.logoutUnknownKind(w, r, claims, id)
	default:
		h.count(deviceLogoutEndpoint, http.StatusOK)
		h.writeJSON(w, http.StatusOK, OperationResponse{Status: statusDone, Message: noDeviceMessage})
	}
}

// logoutUnknownKind signs the user out of the device id of unknown kind: a Logout of a session and one
// of a grant, waited for within one waitTimeout. The answer is of the one that found the device, of the
// session when neither did.
func (h *Handler) logoutUnknownKind(w http.ResponseWriter, r *http.Request, claims *auth.Claims, id string) {
	sessionOp, ok := h.logoutOperation(w, r, deviceLogoutEndpoint, claims, "self-logout-device-", &k8s.Logout{Session: id})
	if !ok {
		return
	}
	grantOp := sessionOp
	grantOp.Logout = &k8s.Logout{Grant: id}
	sessionName, ok := h.startOperation(w, r, deviceLogoutEndpoint, claims, sessionOp)
	if !ok {
		return
	}
	grantName, ok := h.startOperation(w, r, deviceLogoutEndpoint, claims, grantOp)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.waitTimeout)
	defer cancel()
	sessionStatus, sessionDone := h.waitForOperation(ctx, sessionName)
	grantStatus, grantDone := h.waitForOperation(ctx, grantName)

	op, name, status, done := sessionOp, sessionName, sessionStatus, sessionDone
	if sessionDone && sessionStatus.Phase == k8s.PhaseSucceeded && sessionStatus.Message == noSessionMessage {
		op, name, status, done = grantOp, grantName, grantStatus, grantDone
	}
	h.answerOperation(w, r, deviceLogoutEndpoint, claims, op, name, status, done, []string{id})
}

// logoutDevice runs the Logout of one device of the user of claims.
func (h *Handler) logoutDevice(w http.ResponseWriter, r *http.Request, claims *auth.Claims, logout *k8s.Logout, id string) {
	op, ok := h.logoutOperation(w, r, deviceLogoutEndpoint, claims, "self-logout-device-", logout)
	if !ok {
		return
	}
	h.runOperation(w, r, deviceLogoutEndpoint, claims, op, []string{id})
}

// SelfLogoutOthers signs the user out of every device but the one of the request.
func (h *Handler) SelfLogoutOthers(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.beginSelfRequest(w, r, logoutOthersEndpoint)
	if !ok || !h.namesDevice(w, r, logoutOthersEndpoint, claims) {
		return
	}
	devices, ok := h.devices(w, r, logoutOthersEndpoint, claims)
	if !ok {
		return
	}
	current, known := currentDevice(devices.Items, claims)
	if !h.oldEnough(w, logoutOthersEndpoint, current, known) {
		return
	}

	logout := &k8s.Logout{ExceptSession: current.ID}
	if current.Kind == k8s.DeviceKindGrant {
		logout = &k8s.Logout{ExceptGrant: current.ID}
	}
	op, ok := h.logoutOperation(w, r, logoutOthersEndpoint, claims, "self-logout-others-", logout)
	if !ok {
		return
	}
	others := make([]string, 0, len(devices.Items))
	for _, d := range devices.Items {
		if d.ID != current.ID {
			others = append(others, d.ID)
		}
	}
	h.runOperation(w, r, logoutOthersEndpoint, claims, op, others)
}

// devices reads the devices of the user of claims. It writes the response and returns false when
// they cannot be read.
func (h *Handler) devices(w http.ResponseWriter, r *http.Request, endpoint string, claims *auth.Claims) (k8s.Devices, bool) {
	name, err := h.k8sClient.AccountName(r.Context(), claims.UserID, claims.ConnectorID)
	switch {
	case errors.Is(err, k8s.ErrUserNotFound):
		h.logger.Warn("No local user has the user ID of the token", "user_id", claims.UserID, "remote_addr", r.RemoteAddr)
		h.fail(w, endpoint, http.StatusForbidden, "forbidden", "The local user of this token does not exist")
		return k8s.Devices{}, false
	case err != nil:
		h.logger.Error("Failed to find the account of the user", "error", err, "user_id", claims.UserID)
		h.fail(w, endpoint, http.StatusInternalServerError, "internal_error", "Failed to verify user")
		return k8s.Devices{}, false
	}

	// The cache finds the account by the pair of its status, not by its name: the name is a hash of
	// the pair, not a proof of it.
	devices, err := h.k8sClient.Devices(r.Context(), claims.UserID, claims.ConnectorID)
	if err != nil {
		h.logger.Error("Failed to read the devices of the user", "error", err, "user_account", name)
		h.fail(w, endpoint, http.StatusInternalServerError, "internal_error", "Failed to read the devices")
		return k8s.Devices{}, false
	}
	devices.Items = h.signedOut.filter(pairOf(claims), devices.Items, h.now())
	return devices, true
}

// deviceOldEnough tells whether the device of the request may act on the account: it has been
// signed in for h.minDeviceAge. It writes the response when not.
func (h *Handler) deviceOldEnough(w http.ResponseWriter, r *http.Request, endpoint string, claims *auth.Claims) bool {
	if !h.namesDevice(w, r, endpoint, claims) {
		return false
	}
	devices, ok := h.devices(w, r, endpoint, claims)
	if !ok {
		return false
	}
	current, known := currentDevice(devices.Items, claims)
	return h.oldEnough(w, endpoint, current, known)
}

// namesDevice tells whether the token names the device it was issued to: a browser session of Dex
// (sid) or a refresh token (d8_gid). It writes the response when not.
func (h *Handler) namesDevice(w http.ResponseWriter, r *http.Request, endpoint string, claims *auth.Claims) bool {
	if claims.SessionID != "" || claims.GrantID != "" {
		return true
	}
	h.logger.Warn("Request refused: the token names no device",
		"endpoint", endpoint, "user_id", claims.UserID, "connector_id", claims.ConnectorID, "remote_addr", r.RemoteAddr)
	h.fail(w, endpoint, http.StatusForbidden, "forbidden", "The token does not name its device")
	return false
}

// oldEnough tells whether the device of the request has been signed in for h.minDeviceAge: since its
// sign-in, or since the first login of its browser when Dex knows the browser from an earlier one. A
// device the list does not have yet signed in moments ago. With no minimum age, any device may. It
// writes the response when not.
func (h *Handler) oldEnough(w http.ResponseWriter, endpoint string, current k8s.Device, known bool) bool {
	if h.minDeviceAge <= 0 {
		return true
	}
	since := current.SignedInAt
	if !current.KnownSince.IsZero() {
		since = current.KnownSince
	}
	if known && !since.IsZero() {
		wait := since.Add(h.minDeviceAge).Sub(h.now())
		if wait <= 0 {
			return true
		}
		w.Header().Set("Retry-After", retryAfterSeconds(wait))
	}
	h.fail(w, endpoint, http.StatusForbidden, "too_early", fmt.Sprintf(
		"Signing out of other devices and changing the password are possible %s after the first sign-in from this browser",
		formatAge(h.minDeviceAge)))
	return false
}

// formatAge writes a minimum device age the way the settings take it: "24h", "1h30m", "45m".
func formatAge(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// currentDevice finds the device of the request: the browser session of the token, or else its
// refresh token. A refresh token issued in a live browser session is listed as part of it.
func currentDevice(devices []k8s.Device, claims *auth.Claims) (k8s.Device, bool) {
	for _, d := range devices {
		if claims.SessionID != "" && d.Kind == k8s.DeviceKindBrowser && d.ID == claims.SessionID {
			return d, true
		}
	}
	for _, d := range devices {
		if claims.GrantID != "" && d.Kind == k8s.DeviceKindGrant && d.ID == claims.GrantID {
			return d, true
		}
	}
	return k8s.Device{}, false
}

func findDevice(devices []k8s.Device, id string) (k8s.Device, bool) {
	for _, d := range devices {
		if d.ID == id {
			return d, true
		}
	}
	return k8s.Device{}, false
}
