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

package useraccount

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/useridentity"
)

const lockedByAdministratorAnnot = "deckhouse.io/locked-by-administrator"

// passwordView is a Dex Password without hash/previousHashes.
type passwordView struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Email       string
	Username    string
	UserID      string
	Groups      []string
	Attempts    int64
	LockedUntil *time.Time
}

// sessionView is a Dex OfflineSessions without connectorData/totp: the pair and its grants.
type sessionView struct {
	Name        string
	Namespace   string
	Annotations map[string]string
	Email       string
	UserID      string
	ConnID      string
	Attempts    int64
	LockedUntil *time.Time
	// Grants are the refresh tokens of the pair: those of grants and, for an object written before
	// grants, the last one of each client in refresh.
	Grants []grantView
}

// grantView is a refresh token of a pair as OfflineSessions refers to it: the device it was issued to.
type grantView struct {
	ID        string
	ClientID  string
	SessionID string
	IPAddress string
	UserAgent string
	// DeviceHash is the hash of the device mark of the browser that signed in, empty without one.
	DeviceHash string
	CreatedAt  time.Time
	LastUsed   time.Time
}

// browserView is a Dex AuthSession without its cookie secret and nonce: a browser signed in to Dex.
type browserView struct {
	ID          string
	UserID      string
	ConnectorID string
	Clients     []string
	IPAddress   string
	UserAgent   string
	// DeviceHash is the hash of the device mark of the browser, empty without one.
	DeviceHash   string
	CreatedAt    time.Time
	LastActivity time.Time
	// Expiry is when the session ends: the earlier of its idle and absolute expiry, zero if unknown.
	Expiry time.Time
}

// identityView is a Dex UserIdentity without its secrets: the cache keeps only the flags of its
// second factors.
type identityView struct {
	Name          string
	UserID        string
	ConnectorID   string
	Email         string
	BlockedUntil  *time.Time
	LockedByAdmin bool
	// Migrated tells whether the controller has marked the object: until then Dex also reads the
	// lock from the old place.
	Migrated  bool
	Attempts  int64
	LastLogin *time.Time
	TwoFactor v1alpha1.UserAccountTwoFactor
	// KnownDevices is when Dex first saw each browser of the pair complete a login, by the hash of its
	// device mark.
	KnownDevices map[string]time.Time
}

// userView is a deckhouse.io User without spec.password.
type userView struct {
	Name     string
	UID      types.UID
	Email    string
	UserID   string
	Groups   []string
	ExpireAt *time.Time
}

// providerView is a DexProvider without connector credentials.
type providerView struct {
	Name string
	Type string
}

type passwordFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Email             string   `json:"email,omitempty"`
	Username          string   `json:"username,omitempty"`
	UserID            string   `json:"userID,omitempty"`
	Groups            []string `json:"groups,omitempty"`
	LockedUntil       string   `json:"lockedUntil,omitempty"`
}

type sessionFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Email             string                    `json:"email,omitempty"`
	UserID            string                    `json:"userID,omitempty"`
	ConnID            string                    `json:"connID,omitempty"`
	LockedUntil       string                    `json:"lockedUntil,omitempty"`
	Grants            map[string]refTokenFields `json:"grants,omitempty"`
	Refresh           map[string]refTokenFields `json:"refresh,omitempty"`
}

// refTokenFields is a RefreshTokenRef of Dex, which OfflineSessions stores with the field names of Go.
type refTokenFields struct {
	ID        string `json:"ID,omitempty"`
	ClientID  string `json:"ClientID,omitempty"`
	SessionID string `json:"SessionID,omitempty"`
	IPAddress string `json:"IPAddress,omitempty"`
	UserAgent string `json:"UserAgent,omitempty"`
	// DeviceHash is the hash of the device mark of the browser of the sign-in.
	DeviceHash string `json:"DeviceHash,omitempty"`
	CreatedAt  string `json:"CreatedAt,omitempty"`
	LastUsed   string `json:"LastUsed,omitempty"`
}

type authSessionFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	UserID            string                     `json:"userID,omitempty"`
	ConnectorID       string                     `json:"connectorID,omitempty"`
	ClientStates      map[string]json.RawMessage `json:"clientStates,omitempty"`
	IPAddress         string                     `json:"ipAddress,omitempty"`
	UserAgent         string                     `json:"userAgent,omitempty"`
	DeviceHash        string                     `json:"deviceHash,omitempty"`
	CreatedAt         string                     `json:"createdAt,omitempty"`
	LastActivity      string                     `json:"lastActivity,omitempty"`
	AbsoluteExpiry    string                     `json:"absoluteExpiry,omitempty"`
	IdleExpiry        string                     `json:"idleExpiry,omitempty"`
}

type userFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              userSpecFields   `json:"spec,omitempty"`
	Status            userStatusFields `json:"status,omitempty"`
}

type userSpecFields struct {
	Email  string `json:"email,omitempty"`
	UserID string `json:"userID,omitempty"`
}

type userStatusFields struct {
	Groups   []string `json:"groups,omitempty"`
	ExpireAt string   `json:"expireAt,omitempty"`
}

type providerFields struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              providerSpecFields `json:"spec,omitempty"`
}

type providerSpecFields struct {
	Type string `json:"type,omitempty"`
}

func decodePassword(obj *unstructured.Unstructured) (passwordView, error) {
	var fields passwordFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return passwordView{}, fmt.Errorf("decode password: %w", err)
	}
	attempts, err := controller.AsInt64(obj.Object["incorrectPasswordLoginAttempts"])
	if err != nil {
		return passwordView{}, fmt.Errorf("password %s incorrectPasswordLoginAttempts: %w", obj.GetName(), err)
	}
	until, err := controller.ParseRFC3339(fields.LockedUntil)
	if err != nil {
		return passwordView{}, fmt.Errorf("password %s lockedUntil: %w", obj.GetName(), err)
	}
	return passwordView{
		Name:        fields.Name,
		Namespace:   fields.Namespace,
		Annotations: fields.Annotations,
		Email:       fields.Email,
		Username:    fields.Username,
		UserID:      fields.UserID,
		Groups:      fields.Groups,
		Attempts:    attempts,
		LockedUntil: until,
	}, nil
}

func decodeSession(obj *unstructured.Unstructured) (sessionView, error) {
	var fields sessionFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return sessionView{}, fmt.Errorf("decode offlinesessions: %w", err)
	}
	attempts, err := controller.AsInt64(obj.Object["incorrectPasswordLoginAttempts"])
	if err != nil {
		return sessionView{}, fmt.Errorf("offlinesessions %s incorrectPasswordLoginAttempts: %w", obj.GetName(), err)
	}
	until, err := controller.ParseRFC3339(fields.LockedUntil)
	if err != nil {
		return sessionView{}, fmt.Errorf("offlinesessions %s lockedUntil: %w", obj.GetName(), err)
	}
	return sessionView{
		Name:        fields.Name,
		Namespace:   fields.Namespace,
		Annotations: fields.Annotations,
		Email:       fields.Email,
		UserID:      fields.UserID,
		ConnID:      fields.ConnID,
		Attempts:    attempts,
		LockedUntil: until,
		Grants:      decodeGrants(fields.Grants, fields.Refresh),
	}, nil
}

// decodeGrants merges the references of grants and refresh by grant ID; refresh is keyed by the
// client, which an entry written without ClientID names only there.
func decodeGrants(grants, refresh map[string]refTokenFields) []grantView {
	byID := make(map[string]grantView, len(grants)+len(refresh))
	for _, ref := range grants {
		if ref.ID != "" {
			byID[ref.ID] = newGrantView(ref)
		}
	}
	for clientID, ref := range refresh {
		if _, seen := byID[ref.ID]; seen || ref.ID == "" {
			continue
		}
		if ref.ClientID == "" {
			ref.ClientID = clientID
		}
		byID[ref.ID] = newGrantView(ref)
	}
	out := slices.Collect(maps.Values(byID))
	slices.SortFunc(out, func(a, b grantView) int { return strings.Compare(a.ID, b.ID) })
	return out
}

func newGrantView(ref refTokenFields) grantView {
	return grantView{
		ID:         ref.ID,
		ClientID:   ref.ClientID,
		SessionID:  ref.SessionID,
		IPAddress:  ref.IPAddress,
		UserAgent:  ref.UserAgent,
		DeviceHash: ref.DeviceHash,
		CreatedAt:  lenientTime(ref.CreatedAt),
		LastUsed:   lenientTime(ref.LastUsed),
	}
}

// decodeBrowser decodes an AuthSession. Its client states only name the applications.
func decodeBrowser(obj *unstructured.Unstructured) (browserView, error) {
	var fields authSessionFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return browserView{}, fmt.Errorf("decode authsession: %w", err)
	}
	expiry := lenientTime(fields.IdleExpiry)
	if absolute := lenientTime(fields.AbsoluteExpiry); !absolute.IsZero() && (expiry.IsZero() || absolute.Before(expiry)) {
		expiry = absolute
	}
	return browserView{
		ID:           fields.Name,
		UserID:       fields.UserID,
		ConnectorID:  fields.ConnectorID,
		Clients:      slices.Sorted(maps.Keys(fields.ClientStates)),
		IPAddress:    fields.IPAddress,
		UserAgent:    fields.UserAgent,
		DeviceHash:   fields.DeviceHash,
		CreatedAt:    lenientTime(fields.CreatedAt),
		LastActivity: lenientTime(fields.LastActivity),
		Expiry:       expiry,
	}, nil
}

// lenientTime parses a time Dex wrote; a time that is missing, unreadable, or the zero time of Go is
// unknown (zero). A device list must not fail an account over one odd time.
func lenientTime(value string) time.Time {
	parsed, err := controller.ParseRFC3339(value)
	if err != nil || parsed == nil || parsed.Year() <= 1 {
		return time.Time{}
	}
	return *parsed
}

func decodeIdentity(obj *unstructured.Unstructured) (identityView, error) {
	if obj == nil {
		return identityView{}, fmt.Errorf("decode useridentity: object is nil")
	}
	id := useridentity.FromUnstructured(obj)
	claims, err := id.Claims()
	if err != nil {
		return identityView{}, err
	}
	blocked, err := id.BlockedUntil()
	if err != nil {
		return identityView{}, err
	}
	lastLogin, err := id.LastLogin()
	if err != nil {
		return identityView{}, err
	}
	attempts, err := id.IncorrectPasswordLoginAttempts()
	if err != nil {
		return identityView{}, err
	}
	totp, err := id.TOTP()
	if err != nil {
		return identityView{}, err
	}
	view := identityView{
		Name:          obj.GetName(),
		UserID:        id.UserID(),
		ConnectorID:   id.ConnectorID(),
		Email:         claims.Email,
		LockedByAdmin: id.LockedByAdministrator(),
		Migrated:      id.Migrated(),
		Attempts:      attempts,
		KnownDevices:  id.KnownDevices(),
	}
	if !blocked.IsZero() {
		view.BlockedUntil = &blocked
	}
	if !lastLogin.IsZero() {
		view.LastLogin = &lastLogin
	}
	if totp != nil {
		view.TwoFactor = v1alpha1.UserAccountTwoFactor{Enrolled: true, Confirmed: totp.Confirmed}
	}
	return view, nil
}

func decodeUser(obj *unstructured.Unstructured) (userView, error) {
	var fields userFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return userView{}, fmt.Errorf("decode user: %w", err)
	}
	expireAt, err := controller.ParseRFC3339(fields.Status.ExpireAt)
	if err != nil {
		return userView{}, fmt.Errorf("user %s status.expireAt: %w", obj.GetName(), err)
	}
	return userView{
		Name:     fields.Name,
		UID:      fields.UID,
		Email:    fields.Spec.Email,
		UserID:   fields.Spec.UserID,
		Groups:   fields.Status.Groups,
		ExpireAt: expireAt,
	}, nil
}

func decodeProvider(obj *unstructured.Unstructured) (providerView, error) {
	var fields providerFields
	if err := controller.DecodeInto(obj, &fields); err != nil {
		return providerView{}, fmt.Errorf("decode dexprovider: %w", err)
	}
	return providerView{Name: fields.Name, Type: fields.Spec.Type}, nil
}

func hasAdminLock(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	_, ok := annotations[lockedByAdministratorAnnot]
	return ok
}
