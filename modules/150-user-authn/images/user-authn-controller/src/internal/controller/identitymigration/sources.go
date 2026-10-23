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

package identitymigration

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

// lockedByAdministratorAnnot marks an administrator lock on Password and OfflineSessions, the
// old places (see useroperation).
const lockedByAdministratorAnnot = "deckhouse.io/locked-by-administrator"

// Reasons a pair is not copied. They are logged; the pair is counted as skipped.
const (
	skipEmptyPair       = "OfflineSessions has no userID or connID"
	skipNameMismatch    = "OfflineSessions name is not the name Dex gives the pair"
	skipLocalNoPassword = "local pair without a Password yet"
	skipLocalOrphan     = "local pair without a Password or a User: the User was deleted"
	skipPairMismatch    = "UserIdentity of the name belongs to another pair"
	skipBrokenSession   = "OfflineSessions cannot be decoded"
)

// sessionSource is an OfflineSessions object read through the API reader, TOTP secret included.
type sessionSource struct {
	UID           string
	CreatedAt     time.Time
	UserID        string
	ConnID        string
	Email         string
	TOTP          string
	TOTPConfirmed bool
	LockedUntil   *time.Time
	AdminLock     bool
	Refresh       []refreshRef
}

// refreshRef is an entry of OfflineSessions.refresh: the refresh token a client holds. Dex
// writes these keys capitalized: storage.RefreshTokenRef has no JSON tags.
type refreshRef struct {
	ID        string
	CreatedAt time.Time
}

// passwordSource is a Password object read through the API reader.
type passwordSource struct {
	Name        string
	CreatedAt   time.Time
	UserID      string
	Username    string
	Email       string
	Groups      []string
	LockedUntil *time.Time
	AdminLock   bool
}

// tokenSource is a RefreshToken of the pair, read from the cache: claims are not secret.
type tokenSource struct {
	LastUsed    time.Time
	ConnectorID string
	Claims      useridentity.Claims
}

// pair is everything the old places hold about one (connector, user) pair.
type pair struct {
	Name     string
	UserID   string
	ConnID   string
	Session  *sessionSource
	Password *passwordSource
	Tokens   []tokenSource
}

func (p pair) isLocal() bool {
	return p.ConnID == naming.LocalConnectorID
}

// oldLock is the lock of the pair in the old place: Password for local users, OfflineSessions for
// everyone else.
func (p pair) oldLock() (*time.Time, bool) {
	if p.isLocal() {
		if p.Password == nil {
			return nil, false
		}
		return p.Password.LockedUntil, p.Password.AdminLock
	}
	if p.Session == nil {
		return nil, false
	}
	return p.Session.LockedUntil, p.Session.AdminLock
}

// lastLogin approximates the last login with the newest refresh token: a login replaces the token
// of its client, and a refresh does not change CreatedAt.
func (p pair) lastLogin() time.Time {
	var last time.Time
	if p.Session == nil {
		return last
	}
	for _, ref := range p.Session.Refresh {
		if ref.CreatedAt.After(last) {
			last = ref.CreatedAt
		}
	}
	return last
}

// claims are the claims the old places hold for the pair. A local user's are the ones Dex's
// password connector returns from the Password. An external user's come from the refresh token
// used last; groups are the union over every client, because a client filter only narrows them.
// Without tokens only the email of OfflineSessions is known, and the rest of current stays.
func (p pair) claims(current useridentity.Claims) useridentity.Claims {
	if p.isLocal() && p.Password != nil {
		return useridentity.Claims{
			UserID:        p.Password.UserID,
			Username:      p.Password.Username,
			Email:         p.Password.Email,
			EmailVerified: true,
			Groups:        slices.Clone(p.Password.Groups),
		}
	}

	if len(p.Tokens) == 0 {
		c := current
		if c.UserID == "" {
			c.UserID = p.UserID
		}
		if p.Session != nil && p.Session.Email != "" {
			c.Email = p.Session.Email
		}
		return c
	}

	latest := p.Tokens[0]
	groups := []string{}
	for _, tok := range p.Tokens {
		if tok.LastUsed.After(latest.LastUsed) {
			latest = tok
		}
		for _, g := range tok.Claims.Groups {
			if !slices.Contains(groups, g) {
				groups = append(groups, g)
			}
		}
	}
	slices.Sort(groups)
	c := latest.Claims
	c.Groups = groups
	if len(groups) == 0 {
		c.Groups = nil
	}
	if c.UserID == "" {
		c.UserID = p.UserID
	}
	if c.Email == "" && p.Session != nil {
		c.Email = p.Session.Email
	}
	return c
}

// oldPlaces reads what the old places hold about a pair. Secrets are only in the API objects, so
// OfflineSessions and Password are read through apiReader, which bypasses the cache; the Password
// index and refresh token claims come from client, the cache.
type oldPlaces struct {
	client    client.Reader
	apiReader client.Reader
	log       logr.Logger
}

// readPair reads the old places of the pair name. A non-empty reason means the pair is not to be
// copied.
func (o oldPlaces) readPair(ctx context.Context, name string) (pair, string, error) {
	p := pair{Name: name}

	session, reason, err := o.readSession(ctx, name)
	if err != nil || reason != "" {
		return p, reason, err
	}
	if session != nil {
		if session.UserID == "" || session.ConnID == "" {
			return p, skipEmptyPair, nil
		}
		if useridentity.Name(session.UserID, session.ConnID) != name {
			return p, skipNameMismatch, nil
		}
		p.UserID, p.ConnID, p.Session = session.UserID, session.ConnID, session
	}

	if session == nil || p.isLocal() {
		password, err := o.readPassword(ctx, name)
		if err != nil {
			return p, "", err
		}
		if password != nil && session == nil {
			p.UserID, p.ConnID = password.UserID, naming.LocalConnectorID
		}
		p.Password = password
	}

	if p.isLocal() && p.Password == nil {
		// Every local user is a User with a Password. Without a Password, the session belongs to a
		// User whose Password is not created yet, or to a user deleted before 1.78: deleting a User
		// left its OfflineSessions behind then.
		found, err := o.userExists(ctx, p.UserID)
		if err != nil {
			return p, "", err
		}
		if found {
			return p, skipLocalNoPassword, nil
		}
		return p, skipLocalOrphan, nil
	}
	if p.Session != nil {
		tokens, err := o.readTokens(ctx, p)
		if err != nil {
			return p, "", err
		}
		p.Tokens = tokens
	}
	return p, "", nil
}

func (o oldPlaces) readSession(ctx context.Context, name string) (*sessionSource, string, error) {
	obj := controller.Object(controller.OfflineSessionsGVK)
	err := o.apiReader.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("get offlinesessions %s: %w", name, err)
	}
	session, err := decodeSession(obj)
	if err != nil {
		o.log.Error(err, "Cannot decode OfflineSessions", "pair", name)
		return nil, skipBrokenSession, nil
	}
	return session, "", nil
}

// readPassword finds the Password of a local pair through the cache index and reads it fresh.
// During an email change two Passwords of the same user exist for a moment; the newer is the one
// the User controller keeps.
func (o oldPlaces) readPassword(ctx context.Context, name string) (*passwordSource, error) {
	list := controller.List(controller.PasswordGVK)
	if err := o.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingFields{controller.PasswordPairField: name}); err != nil {
		return nil, fmt.Errorf("list passwords of pair %s: %w", name, err)
	}

	var newest *passwordSource
	for i := range list.Items {
		obj := controller.Object(controller.PasswordGVK)
		key := types.NamespacedName{Namespace: list.Items[i].GetNamespace(), Name: list.Items[i].GetName()}
		err := o.apiReader.Get(ctx, key, obj)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get password %s: %w", key.Name, err)
		}
		password, err := decodePassword(obj)
		if err != nil {
			return nil, err
		}
		if password.UserID == "" || useridentity.Name(password.UserID, naming.LocalConnectorID) != name {
			continue
		}
		if newest == nil || password.CreatedAt.After(newest.CreatedAt) {
			newest = password
		}
	}
	return newest, nil
}

// userExists tells whether the User name exists. It reads through the API reader: the cache can
// still miss a User created a moment ago, or hold one deleted a moment ago. A name that is not a
// valid object name cannot be a User's.
func (o oldPlaces) userExists(ctx context.Context, name string) (bool, error) {
	if len(validation.IsDNS1123Subdomain(name)) > 0 {
		return false, nil
	}
	err := o.apiReader.Get(ctx, types.NamespacedName{Name: name}, controller.Object(controller.UserGVK))
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get user %s: %w", name, err)
	}
	return true, nil
}

// deleteOrphanSession deletes the OfflineSessions of a local pair that has neither a Password nor
// a User. The old Dex would hand its TOTP secret to a user created again with the same name, and
// the copy would then take the secret into the new user's UserIdentity. The UID precondition
// keeps a newer object of the same name.
func (m *Migrator) deleteOrphanSession(ctx context.Context, name string, s *sessionSource) error {
	obj := controller.Object(controller.OfflineSessionsGVK)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	uid := types.UID(s.UID)
	err := m.client.Delete(ctx, obj, client.Preconditions{UID: &uid})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete offlinesessions %s: %w", name, err)
	}
	m.log.Info("Deleted the OfflineSessions of a deleted local user", "pair", name, "userID", s.UserID)
	return nil
}

// readTokens reads the refresh tokens OfflineSessions refers to. A token of another connector
// or user is not the pair's and is left out.
func (o oldPlaces) readTokens(ctx context.Context, p pair) ([]tokenSource, error) {
	tokens := make([]tokenSource, 0, len(p.Session.Refresh))
	for _, ref := range p.Session.Refresh {
		obj := controller.Object(controller.RefreshTokenGVK)
		err := o.client.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: ref.ID}, obj)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("get refreshtoken %s: %w", ref.ID, err)
		}
		tok, err := decodeToken(obj)
		if err != nil {
			return nil, err
		}
		if tok.ConnectorID != "" && tok.ConnectorID != p.ConnID {
			continue
		}
		if tok.Claims.UserID != "" && tok.Claims.UserID != p.UserID {
			continue
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

type sessionFields struct {
	metav1.ObjectMeta `json:"metadata"`
	UserID            string                    `json:"userID"`
	ConnID            string                    `json:"connID"`
	Email             string                    `json:"email"`
	TOTP              string                    `json:"totp"`
	TOTPConfirmed     bool                      `json:"totpConfirmed"`
	LockedUntil       string                    `json:"lockedUntil"`
	Refresh           map[string]*refreshFields `json:"refresh"`
}

type refreshFields struct {
	ID        string `json:"ID"`
	CreatedAt string `json:"CreatedAt"`
}

type passwordFields struct {
	metav1.ObjectMeta `json:"metadata"`
	UserID            string   `json:"userID"`
	Username          string   `json:"username"`
	Email             string   `json:"email"`
	Groups            []string `json:"groups"`
	LockedUntil       string   `json:"lockedUntil"`
}

type tokenFields struct {
	LastUsed    string              `json:"LastUsed"`
	ConnectorID string              `json:"connectorID"`
	Claims      useridentity.Claims `json:"claims"`
}

func decodeSession(obj *unstructured.Unstructured) (*sessionSource, error) {
	var f sessionFields
	if err := controller.DecodeInto(obj, &f); err != nil {
		return nil, fmt.Errorf("decode offlinesessions %s: %w", obj.GetName(), err)
	}
	until, err := controller.ParseRFC3339(f.LockedUntil)
	if err != nil {
		return nil, fmt.Errorf("offlinesessions %s lockedUntil: %w", obj.GetName(), err)
	}
	s := &sessionSource{
		UID:           string(f.UID),
		CreatedAt:     f.CreationTimestamp.UTC(),
		UserID:        f.UserID,
		ConnID:        f.ConnID,
		Email:         f.Email,
		TOTP:          f.TOTP,
		TOTPConfirmed: f.TOTPConfirmed,
		LockedUntil:   until,
		AdminLock:     hasAdminLock(f.Annotations),
	}
	for _, ref := range f.Refresh {
		if ref == nil || ref.ID == "" {
			continue
		}
		created, err := controller.ParseRFC3339(ref.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("offlinesessions %s refresh %s CreatedAt: %w", obj.GetName(), ref.ID, err)
		}
		r := refreshRef{ID: ref.ID}
		if created != nil {
			r.CreatedAt = *created
		}
		s.Refresh = append(s.Refresh, r)
	}
	slices.SortFunc(s.Refresh, func(a, b refreshRef) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	return s, nil
}

func decodePassword(obj *unstructured.Unstructured) (*passwordSource, error) {
	var f passwordFields
	if err := controller.DecodeInto(obj, &f); err != nil {
		return nil, fmt.Errorf("decode password %s: %w", obj.GetName(), err)
	}
	until, err := controller.ParseRFC3339(f.LockedUntil)
	if err != nil {
		return nil, fmt.Errorf("password %s lockedUntil: %w", obj.GetName(), err)
	}
	return &passwordSource{
		Name:        f.Name,
		CreatedAt:   f.CreationTimestamp.UTC(),
		UserID:      f.UserID,
		Username:    f.Username,
		Email:       f.Email,
		Groups:      f.Groups,
		LockedUntil: until,
		AdminLock:   hasAdminLock(f.Annotations),
	}, nil
}

func decodeToken(obj *unstructured.Unstructured) (tokenSource, error) {
	var f tokenFields
	if err := controller.DecodeInto(obj, &f); err != nil {
		return tokenSource{}, fmt.Errorf("decode refreshtoken %s: %w", obj.GetName(), err)
	}
	lastUsed, err := controller.ParseRFC3339(f.LastUsed)
	if err != nil {
		return tokenSource{}, fmt.Errorf("refreshtoken %s LastUsed: %w", obj.GetName(), err)
	}
	tok := tokenSource{ConnectorID: f.ConnectorID, Claims: f.Claims}
	if lastUsed != nil {
		tok.LastUsed = *lastUsed
	}
	return tok, nil
}

// hasAdminLock tells whether the old place marks its lock as the administrator's.
func hasAdminLock(annotations map[string]string) bool {
	_, ok := annotations[lockedByAdministratorAnnot]
	return ok
}
