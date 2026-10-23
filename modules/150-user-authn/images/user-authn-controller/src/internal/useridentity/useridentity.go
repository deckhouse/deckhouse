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

// Package useridentity reads and changes Dex UserIdentity objects (dex.coreos.com/v1).
//
// Dex v2.46 keeps one UserIdentity per pair of a connector and a user: the claims of the last
// login, the second factors, and the lock. Older Dex kept the same data in OfflineSessions (TOTP,
// external locks) and Password (local locks). The controller copies it over during 1.78 and
// writes both places in administrative operations.
//
// The object is kept unstructured, so fields this package does not know survive a write: Dex
// writes the whole object, and so does the controller, with the resourceVersion it read.
package useridentity

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// AuthenticatorTOTP is the key of the TOTP secret in mfaSecrets and its authenticatorID. The
	// Dex config of the module must name its TOTP authenticator the same (mfa.authenticators[].id).
	AuthenticatorTOTP = "totp"
	// MFATypeTOTP is MFASecret.type of a TOTP secret.
	MFATypeTOTP = "TOTP"

	// RecordAnnotation holds the controller's Record of what it wrote into the object. Dex keeps
	// annotations when it writes the object.
	RecordAnnotation = "user-authn.deckhouse.io/migrated"
	// AdminLockAnnotation holds the blockedUntil an administrator set with a UserOperation, in
	// RFC 3339. The lock is the administrator's only while the value equals blockedUntil: a later
	// automatic lock replaces blockedUntil, and the annotation then describes an older lock.
	AdminLockAnnotation = "user-authn.deckhouse.io/admin-lock"

	// SourceOfflineSessions, SourcePassword, and SourceUserOperation name what the controller
	// wrote into a UserIdentity from, in Record.Source.
	SourceOfflineSessions = "OfflineSessions"
	SourcePassword        = "Password"
	SourceUserOperation   = "UserOperation"
)

const (
	fieldUserID       = "userID"
	fieldConnectorID  = "connectorID"
	fieldClaims       = "claims"
	fieldMFASecrets   = "mfaSecrets"
	fieldWebAuthn     = "webauthnCredentials"
	fieldCreatedAt    = "createdAt"
	fieldLastLogin    = "lastLogin"
	fieldBlockedUntil = "blockedUntil"
)

// Claims mirrors the claims of a Dex UserIdentity.
type Claims struct {
	UserID            string   `json:"userID"`
	Username          string   `json:"username"`
	PreferredUsername string   `json:"preferredUsername"`
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"emailVerified"`
	Groups            []string `json:"groups,omitempty"`
}

// IsEmpty reports whether no identifying claim is set.
func (c Claims) IsEmpty() bool {
	return c.UserID == "" && c.Username == "" && c.PreferredUsername == "" && c.Email == ""
}

// Equal reports whether c and o hold the same claims.
func (c Claims) Equal(o Claims) bool {
	return c.UserID == o.UserID &&
		c.Username == o.Username &&
		c.PreferredUsername == o.PreferredUsername &&
		c.Email == o.Email &&
		c.EmailVerified == o.EmailVerified &&
		slices.Equal(c.Groups, o.Groups)
}

// MFASecret mirrors one entry of mfaSecrets.
type MFASecret struct {
	AuthenticatorID string
	Type            string
	Secret          string
	Confirmed       bool
	CreatedAt       time.Time
}

// Record is the value of RecordAnnotation: what the controller copied into the object and when.
// It is what keeps the copy from bringing back data that was removed from the UserIdentity.
type Record struct {
	// Time is when the controller first wrote the object, RFC 3339.
	Time string `json:"time"`
	// Source lists, comma-separated, what the controller wrote the object from: OfflineSessions,
	// Password, UserOperation.
	Source string `json:"source"`
	// TOTPFrom is the uid of the OfflineSessions object the TOTP secret was last taken from, or
	// confirmed from, or passed over for a confirmed secret of the UserIdentity. A secret in that
	// object is not copied again: if it is gone from the UserIdentity, it was removed there. An
	// unconfirmed secret held back behind a confirmed one does not move it.
	TOTPFrom string `json:"totpFrom,omitempty"`
	// LockFrom is the old-place lockedUntil last taken into blockedUntil, RFC 3339. The same value
	// is not taken again.
	LockFrom string `json:"lockFrom,omitempty"`
	// LastLogin is the lastLogin the controller last wrote. While the object still has it, nobody
	// else has logged the user in, and the controller keeps claims and lastLogin up to date.
	LastLogin string `json:"lastLogin,omitempty"`
	// ClearedAt is the last time a UserOperation (Reset2FA, Unlock) cleared data in the object.
	ClearedAt string `json:"clearedAt,omitempty"`
}

// AddSource adds src to Source, keeping it sorted and free of duplicates.
func (r *Record) AddSource(src string) {
	sources := []string{}
	if r.Source != "" {
		sources = strings.Split(r.Source, ",")
	}
	if !slices.Contains(sources, src) {
		sources = append(sources, src)
	}
	slices.Sort(sources)
	r.Source = strings.Join(sources, ",")
}

// FormatTime is how this package writes times: RFC 3339 in UTC, with nanoseconds only when set,
// which is what Dex writes too.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// Name is the object name of the pair (userID, connectorID), the same as Dex's OfflineSessions name.
func Name(userID, connectorID string) string {
	return naming.OfflineTokenName(userID, connectorID)
}

// Identity is a UserIdentity object.
type Identity struct {
	obj *unstructured.Unstructured
}

// New returns an empty UserIdentity of the pair (userID, connectorID). Dex compares both fields
// with the ones it looks the object up by, so they are copied as they are.
func New(userID, connectorID string) *Identity {
	obj := controller.Object(controller.UserIdentityGVK)
	obj.SetName(Name(userID, connectorID))
	obj.SetNamespace(naming.DexNamespace)
	obj.Object[fieldUserID] = userID
	obj.Object[fieldConnectorID] = connectorID
	return &Identity{obj: obj}
}

// FromUnstructured wraps obj. Changes to the Identity change obj.
func FromUnstructured(obj *unstructured.Unstructured) *Identity {
	return &Identity{obj: obj}
}

// Object returns the underlying object.
func (i *Identity) Object() *unstructured.Unstructured {
	return i.obj
}

// UserID returns the userID field.
func (i *Identity) UserID() string {
	return stringField(i.obj.Object, fieldUserID)
}

// ConnectorID returns the connectorID field.
func (i *Identity) ConnectorID() string {
	return stringField(i.obj.Object, fieldConnectorID)
}

// Claims returns the claims.
func (i *Identity) Claims() (Claims, error) {
	raw, found := i.obj.Object[fieldClaims]
	if !found || raw == nil {
		return Claims{}, nil
	}
	var c Claims
	if err := roundTrip(raw, &c); err != nil {
		return Claims{}, fmt.Errorf("useridentity %s claims: %w", i.obj.GetName(), err)
	}
	return c, nil
}

// SetClaims replaces the claims.
func (i *Identity) SetClaims(c Claims) {
	claims := map[string]any{
		"userID":            c.UserID,
		"username":          c.Username,
		"preferredUsername": c.PreferredUsername,
		"email":             c.Email,
		"emailVerified":     c.EmailVerified,
	}
	if len(c.Groups) > 0 {
		groups := make([]any, 0, len(c.Groups))
		for _, g := range c.Groups {
			groups = append(groups, g)
		}
		claims["groups"] = groups
	}
	i.obj.Object[fieldClaims] = claims
}

// BlockedUntil returns blockedUntil; zero means not blocked. Dex writes the zero time as
// 0001-01-01T00:00:00Z.
func (i *Identity) BlockedUntil() (time.Time, error) {
	return i.timeField(fieldBlockedUntil)
}

// SetBlockedUntil sets blockedUntil, or removes it when t is zero.
func (i *Identity) SetBlockedUntil(t time.Time) error {
	return i.setTimeField(fieldBlockedUntil, t)
}

// CreatedAt returns createdAt; zero when unset.
func (i *Identity) CreatedAt() (time.Time, error) {
	return i.timeField(fieldCreatedAt)
}

// SetCreatedAt sets createdAt, or removes it when t is zero.
func (i *Identity) SetCreatedAt(t time.Time) error {
	return i.setTimeField(fieldCreatedAt, t)
}

// LastLogin returns lastLogin; zero when unset.
func (i *Identity) LastLogin() (time.Time, error) {
	return i.timeField(fieldLastLogin)
}

// SetLastLogin sets lastLogin, or removes it when t is zero.
func (i *Identity) SetLastLogin(t time.Time) error {
	return i.setTimeField(fieldLastLogin, t)
}

// AdminLock returns the time in AdminLockAnnotation, if the annotation is set and parses.
func (i *Identity) AdminLock() (time.Time, bool) {
	value, ok := i.obj.GetAnnotations()[AdminLockAnnotation]
	if !ok {
		return time.Time{}, false
	}
	t, err := controller.ParseRFC3339(value)
	if err != nil || t == nil {
		return time.Time{}, false
	}
	return *t, true
}

// SetAdminLock sets AdminLockAnnotation to t.
func (i *Identity) SetAdminLock(t time.Time) {
	i.setAnnotation(AdminLockAnnotation, FormatTime(t))
}

// ClearAdminLock removes AdminLockAnnotation.
func (i *Identity) ClearAdminLock() {
	annotations := i.obj.GetAnnotations()
	if _, ok := annotations[AdminLockAnnotation]; !ok {
		return
	}
	delete(annotations, AdminLockAnnotation)
	i.obj.SetAnnotations(annotations)
}

// LockedByAdministrator reports whether the current blockedUntil is the one an administrator set.
func (i *Identity) LockedByAdministrator() bool {
	blocked, err := i.BlockedUntil()
	if err != nil || blocked.IsZero() {
		return false
	}
	admin, ok := i.AdminLock()
	return ok && admin.Equal(blocked)
}

// TOTP returns the TOTP entry of mfaSecrets, or nil.
func (i *Identity) TOTP() (*MFASecret, error) {
	entry, ok := i.totpEntry()
	if !ok {
		return nil, nil
	}
	created, err := controller.ParseRFC3339(stringField(entry, "createdAt"))
	if err != nil {
		return nil, fmt.Errorf("useridentity %s totp createdAt: %w", i.obj.GetName(), err)
	}
	secret := &MFASecret{
		AuthenticatorID: stringField(entry, "authenticatorID"),
		Type:            stringField(entry, "type"),
		Secret:          stringField(entry, "secret"),
	}
	if confirmed, ok := entry["confirmed"].(bool); ok {
		secret.Confirmed = confirmed
	}
	if created != nil {
		secret.CreatedAt = *created
	}
	return secret, nil
}

// SetTOTP replaces the TOTP entry of mfaSecrets. The replay counter of the old entry does not
// apply to a new secret and is dropped.
func (i *Identity) SetTOTP(s MFASecret) {
	secrets, ok := i.obj.Object[fieldMFASecrets].(map[string]any)
	if !ok || secrets == nil {
		secrets = map[string]any{}
	}
	secrets[AuthenticatorTOTP] = map[string]any{
		"authenticatorID": AuthenticatorTOTP,
		"type":            MFATypeTOTP,
		"secret":          s.Secret,
		"confirmed":       s.Confirmed,
		"createdAt":       FormatTime(s.CreatedAt),
	}
	i.obj.Object[fieldMFASecrets] = secrets
}

// ConfirmTOTP marks the TOTP entry confirmed and keeps the rest of it.
func (i *Identity) ConfirmTOTP() {
	entry, ok := i.totpEntry()
	if !ok {
		return
	}
	entry["confirmed"] = true
}

// ClearMFA removes every second factor, as Dex's ResetMFA does.
func (i *Identity) ClearMFA() {
	delete(i.obj.Object, fieldMFASecrets)
	delete(i.obj.Object, fieldWebAuthn)
}

// Record returns the controller's record and whether the object has one.
func (i *Identity) Record() (Record, bool, error) {
	value, ok := i.obj.GetAnnotations()[RecordAnnotation]
	if !ok {
		return Record{}, false, nil
	}
	var r Record
	if err := json.Unmarshal([]byte(value), &r); err != nil {
		return Record{}, true, fmt.Errorf("useridentity %s annotation %s: %w", i.obj.GetName(), RecordAnnotation, err)
	}
	return r, true, nil
}

// SetRecord writes r into RecordAnnotation.
func (i *Identity) SetRecord(r Record) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", RecordAnnotation, err)
	}
	i.setAnnotation(RecordAnnotation, string(raw))
	return nil
}

func (i *Identity) totpEntry() (map[string]any, bool) {
	secrets, ok := i.obj.Object[fieldMFASecrets].(map[string]any)
	if !ok {
		return nil, false
	}
	entry, ok := secrets[AuthenticatorTOTP].(map[string]any)
	return entry, ok
}

func (i *Identity) timeField(field string) (time.Time, error) {
	t, err := controller.ParseRFC3339(stringField(i.obj.Object, field))
	if err != nil {
		return time.Time{}, fmt.Errorf("useridentity %s %s: %w", i.obj.GetName(), field, err)
	}
	if t == nil || t.IsZero() {
		return time.Time{}, nil
	}
	return *t, nil
}

// setTimeField writes t, leaving an equal instant as it is so that a rewrite is not a change.
func (i *Identity) setTimeField(field string, t time.Time) error {
	current, err := i.timeField(field)
	if err != nil {
		return err
	}
	if t.IsZero() {
		if current.IsZero() {
			// Dex's own zero time stays: it means the same as no field.
			return nil
		}
		delete(i.obj.Object, field)
		return nil
	}
	if current.Equal(t) {
		return nil
	}
	i.obj.Object[field] = FormatTime(t)
	return nil
}

func (i *Identity) setAnnotation(key, value string) {
	annotations := i.obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	i.obj.SetAnnotations(annotations)
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func roundTrip(in, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}
