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

// Package revocation reads and writes Dex Revocation objects (dex.coreos.com/v1).
//
// A Revocation records that something issued by Dex is no longer valid: everything of a pair of a
// connector and a user below a generation (type pair), a browser session (type session), or one
// grant, a refresh token and what was issued from it (type grant). Dex writes them on its logout and
// RFC 7009 revocation paths, the controller on UserOperation and User deletion. The revocation feed
// of the controller, the user-authz webhook, and basic-auth-proxy read them and refuse what they
// name. The fields are top-level, as in every Dex storage type.
package revocation

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"slices"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

const (
	// TypePair revokes everything issued to a pair below Generation.
	TypePair = "pair"
	// TypeSession ends the browser session SessionID.
	TypeSession = "session"
	// TypeGrant revokes the grant GrantID of ClientID.
	TypeGrant = "grant"

	// TypeLabel holds the type, PairLabel the name of the pair (naming.OfflineTokenName), and
	// OperationUIDLabel the UID of the UserOperation that wrote the object.
	TypeLabel         = "user-authn.deckhouse.io/revocation-type"
	PairLabel         = "user-authn.deckhouse.io/pair"
	OperationUIDLabel = "user-authn.deckhouse.io/operation-uid"

	// SourceController and SourceDex name the writer.
	SourceController = "user-authn-controller"
	SourceDex        = "dex"

	// The reasons the controller writes. Dex writes TokenRevoked and SessionLogout.
	ReasonLock                = "Lock"
	ReasonResetPassword       = "ResetPassword"
	ReasonReset2FA            = "Reset2FA"
	ReasonUserDeleted         = "UserDeleted"
	ReasonLogout              = "Logout"
	ReasonLogoutSession       = "LogoutSession"
	ReasonLogoutExceptSession = "LogoutExceptSession"
)

const (
	fieldType        = "type"
	fieldConnectorID = "connectorID"
	fieldUserID      = "userID"
	fieldSubject     = "subject"
	fieldGeneration  = "generation"
	fieldSessionID   = "sessionID"
	fieldGrantID     = "grantID"
	fieldClientID    = "clientID"
	fieldClients     = "clients"
	fieldReason      = "reason"
	fieldSource      = "source"
	fieldCreatedAt   = "createdAt"
)

// minRetention is the retention at the shortest idTokenTTL that makes sense, 10 minutes.
const minRetention = 70 * time.Minute

// Retention is how long a revocation is kept: idTokenTTL and an hour, at least 1h10m. No ID token
// of a revoked pair lives longer than idTokenTTL, a session of dex-authenticator goes to Dex for a
// refresh after idTokenTTL and is refused there, and an authorization code lives 30 minutes.
func Retention(idTokenTTL time.Duration) time.Duration {
	return max(idTokenTTL+time.Hour, minRetention)
}

// Revocation is a Revocation object.
type Revocation struct {
	Name        string
	Type        string
	ConnectorID string
	UserID      string
	// Subject is the sub claim of the tokens of the pair, Subject(UserID, ConnectorID).
	Subject string
	// Generation is the new generation of the pair; set for TypePair only.
	Generation uint64
	SessionID  string
	GrantID    string
	ClientID   string
	// Clients are the clients that held grants or session state of the pair when it was revoked,
	// sorted and unique. The feed sends the revocation to these clients only.
	Clients []string
	Reason  string
	Source  string
	// CreatedAt is when the writer created the object. Created is when the API server did.
	CreatedAt time.Time
	Created   time.Time
	// OperationUID is the UID of the UserOperation that wrote the object, if one did.
	OperationUID string
}

// PairName is the name of the pair revocation of pair (naming.OfflineTokenName) at generation.
func PairName(pair string, generation uint64) string {
	return fmt.Sprintf("pair-%s-g%d", pair, generation)
}

// SessionName is the name of the revocation of the browser session sid.
func SessionName(sid string) string {
	return "session-" + sid
}

// GrantName is the name of the revocation of the grant id (RefreshToken.ID).
func GrantName(id string) string {
	return "grant-" + id
}

// Subject is the sub claim Dex puts into the tokens of the pair: the protobuf message
// IDTokenSubject{user_id = 1, conn_id = 2} in unpadded base64url, as Dex's tokens.GenSubject.
func Subject(userID, connectorID string) string {
	return base64.RawURLEncoding.EncodeToString(protoStrings(userID, connectorID))
}

// protoStrings is the protobuf encoding of a message whose fields 1 and 2 are the strings first and
// second. Protobuf leaves out empty strings.
func protoStrings(first, second string) []byte {
	var b []byte
	for i, value := range []string{first, second} {
		if value == "" {
			continue
		}
		b = append(b, byte((i+1)<<3|2))
		b = binary.AppendUvarint(b, uint64(len(value)))
		b = append(b, value...)
	}
	return b
}

// Pair is the name of the pair of the revocation, naming.OfflineTokenName(UserID, ConnectorID).
func (r Revocation) Pair() string {
	return naming.OfflineTokenName(r.UserID, r.ConnectorID)
}

// Object returns r as an object to create in the Dex namespace.
func (r Revocation) Object() (*unstructured.Unstructured, error) {
	if r.Generation > math.MaxInt64 {
		return nil, fmt.Errorf("revocation %s: generation %d does not fit the object", r.Name, r.Generation)
	}
	obj := controller.Object(controller.RevocationGVK)
	obj.SetName(r.Name)
	obj.SetNamespace(naming.DexNamespace)
	labels := map[string]string{
		TypeLabel: r.Type,
		PairLabel: r.Pair(),
	}
	if r.OperationUID != "" {
		labels[OperationUIDLabel] = r.OperationUID
	}
	obj.SetLabels(labels)

	clients := make([]any, 0, len(r.Clients))
	for _, c := range r.Clients {
		clients = append(clients, c)
	}
	obj.Object[fieldType] = r.Type
	obj.Object[fieldConnectorID] = r.ConnectorID
	obj.Object[fieldUserID] = r.UserID
	obj.Object[fieldSubject] = r.Subject
	obj.Object[fieldGeneration] = int64(r.Generation)
	obj.Object[fieldSessionID] = r.SessionID
	obj.Object[fieldGrantID] = r.GrantID
	obj.Object[fieldClientID] = r.ClientID
	obj.Object[fieldClients] = clients
	obj.Object[fieldReason] = r.Reason
	obj.Object[fieldSource] = r.Source
	obj.Object[fieldCreatedAt] = r.CreatedAt.UTC().Format(time.RFC3339Nano)
	return obj, nil
}

// FromObject decodes a Revocation object.
func FromObject(obj *unstructured.Unstructured) (Revocation, error) {
	if obj == nil {
		return Revocation{}, fmt.Errorf("revocation object is nil")
	}
	generation, err := controller.AsInt64(obj.Object[fieldGeneration])
	if err != nil {
		return Revocation{}, fmt.Errorf("revocation %s generation: %w", obj.GetName(), err)
	}
	if generation < 0 {
		return Revocation{}, fmt.Errorf("revocation %s generation: %d is negative", obj.GetName(), generation)
	}
	clients, _, err := unstructured.NestedStringSlice(obj.Object, fieldClients)
	if err != nil {
		return Revocation{}, fmt.Errorf("revocation %s clients: %w", obj.GetName(), err)
	}
	r := Revocation{
		Name:         obj.GetName(),
		Type:         stringField(obj, fieldType),
		ConnectorID:  stringField(obj, fieldConnectorID),
		UserID:       stringField(obj, fieldUserID),
		Subject:      stringField(obj, fieldSubject),
		Generation:   uint64(generation),
		SessionID:    stringField(obj, fieldSessionID),
		GrantID:      stringField(obj, fieldGrantID),
		ClientID:     stringField(obj, fieldClientID),
		Clients:      clients,
		Reason:       stringField(obj, fieldReason),
		Source:       stringField(obj, fieldSource),
		Created:      obj.GetCreationTimestamp().UTC(),
		OperationUID: obj.GetLabels()[OperationUIDLabel],
	}
	if created, err := controller.ParseRFC3339(stringField(obj, fieldCreatedAt)); err == nil && created != nil {
		r.CreatedAt = *created
	}
	return r, nil
}

// Time is when the revocation happened: when the API server created the object, or, while that is
// unknown, when the writer did. Retention counts from it.
func (r Revocation) Time() time.Time {
	if !r.Created.IsZero() {
		return r.Created
	}
	return r.CreatedAt
}

// Expires is when the revocation stops mattering: nothing it revokes is valid for longer than
// retention after it.
func (r Revocation) Expires(retention time.Duration) time.Time {
	return r.Time().Add(retention)
}

// SortedClients returns the clients sorted and unique, without empty ids.
func SortedClients(clients []string) []string {
	out := make([]string, 0, len(clients))
	for _, c := range clients {
		if c != "" {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func stringField(obj *unstructured.Unstructured, field string) string {
	s, _ := obj.Object[field].(string)
	return s
}
