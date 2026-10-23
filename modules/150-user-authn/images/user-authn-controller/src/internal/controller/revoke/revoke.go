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

// Package revoke ends what Dex issued to a pair of a connector and a user: it raises the
// generation of the pair, records the revocation in a Revocation object, and deletes the browser
// sessions, the sign-ins in progress (authorization codes and auth requests), and the grants the
// pair has in Dex. It also deletes Revocation objects past retention.
//
// The generation goes first: once it is raised, Dex refuses everything it issued to the pair
// before, so the rest can take its time and fail and be retried. The Revocation object goes next,
// before the deletions, so that the revocation feed and the user-authz webhook hear of it within
// seconds. The deletions clean up what Dex already refuses - with one exception: when a User is
// deleted, its UserIdentity goes after the revocation, Dex reads the missing record as generation
// 0 again, and a code or an auth request stamped 0 would match. Deleting them closes that.
package revoke

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

// Pair is the pair to revoke, with the claims a UserIdentity created for it gets.
type Pair struct {
	UserID      string
	ConnectorID string
	Claims      useridentity.Claims
}

func (p Pair) name() string {
	return naming.OfflineTokenName(p.UserID, p.ConnectorID)
}

// Result is what a revocation did.
type Result struct {
	// Name is the Revocation object written, or the first of them when there are several.
	Name string
	// Generation is the new generation of the pair; 0 when the generation did not change.
	Generation uint64
	// RevocationsWritten counts the Revocation objects written, including ones that existed.
	RevocationsWritten   int
	RefreshTokensDeleted int
	AuthSessionsDeleted  int
}

func (r *Result) addRevocation(name string) {
	if r.Name == "" {
		r.Name = name
	}
	r.RevocationsWritten++
}

// Revoker revokes what Dex issued to pairs.
type Revoker struct {
	client    client.Client
	apiReader client.Reader
	log       logr.Logger
	now       func() time.Time
}

// New returns a Revoker. c serves cached reads through the indexes of controller.Indexes and
// writes; apiReader must bypass the cache: the UserIdentity and the references of OfflineSessions
// are read fresh. now defaults to time.Now when nil.
func New(c client.Client, apiReader client.Reader, log logr.Logger, now func() time.Time) *Revoker {
	if now == nil {
		now = time.Now
	}
	return &Revoker{client: c, apiReader: apiReader, log: log, now: now}
}

// LogoutPair revokes everything Dex issued to the pair: the generation, a Revocation of type pair,
// the browser sessions (a Revocation of type session each) together with the sign-ins in
// progress, and the grants, in this order.
// opUID is the UID of the UserOperation that
// asks for it, or empty: a retry of the same operation finds the Revocation by it and does not
// raise the generation again. A retry after a failure between the generation and the Revocation
// raises it once more, which is harmless. Every step can be repeated.
func (r *Revoker) LogoutPair(ctx context.Context, p Pair, reason, opUID string) (Result, error) {
	pairName := p.name()
	res := Result{}

	generation, name, found, err := r.writtenBy(ctx, pairName, opUID)
	if err != nil {
		return res, err
	}
	if !found {
		generation, err = useridentity.BumpGeneration(ctx, r.apiReader, r.client,
			useridentity.Pair{UserID: p.UserID, ConnectorID: p.ConnectorID}, p.Claims, r.now())
		if err != nil {
			return res, err
		}
		name, err = r.writePairRevocation(ctx, p, generation, reason, opUID)
		if err != nil {
			return res, err
		}
	}
	res.Generation = generation
	res.addRevocation(name)

	sessions, err := r.sessionsOf(ctx, pairName)
	if err != nil {
		return res, err
	}
	for _, s := range sessions {
		if s.generation >= generation {
			// A sign-in after the revocation.
			continue
		}
		// The pair revocation carries no sid: the revocation of each session tells Dex which
		// back-channel logouts to send.
		if err := r.writeSessionRevocation(ctx, p, s, reason, opUID, &res); err != nil {
			return res, err
		}
		deleted, err := r.deleteDexObject(ctx, controller.AuthSessionGVK, s.name)
		if err != nil {
			return res, err
		}
		if deleted {
			res.AuthSessionsDeleted++
		}
	}
	signIns, err := r.deletePendingSignIns(ctx, p, generation)
	if err != nil {
		return res, err
	}

	tokens, err := r.tokensOf(ctx, pairName)
	if err != nil {
		return res, err
	}
	revoked := map[string]struct{}{}
	for _, t := range tokens {
		if t.generation >= generation {
			continue
		}
		revoked[t.name] = struct{}{}
		deleted, err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, t.name)
		if err != nil {
			return res, err
		}
		if deleted {
			res.RefreshTokensDeleted++
		}
	}
	refs, err := r.offlineRefs(ctx, p)
	if err != nil {
		return res, err
	}
	for _, id := range sortedKeys(refs) {
		if _, ok := revoked[id]; !ok {
			continue
		}
		if err := r.removeRef(ctx, pairName, refs[id]); err != nil {
			return res, err
		}
	}

	r.log.Info("Revoked the credentials of the pair",
		"connectorID", p.ConnectorID,
		"userID", p.UserID,
		"reason", reason,
		"generation", generation,
		"revocation", name,
		"refreshTokensDeleted", res.RefreshTokensDeleted,
		"authSessionsDeleted", res.AuthSessionsDeleted,
		"authCodesDeleted", signIns.codes,
		"authRequestsDeleted", signIns.requests,
	)
	return res, nil
}

// writtenBy finds the pair Revocation an earlier run of the operation opUID wrote. With several, it
// takes the highest generation.
func (r *Revoker) writtenBy(ctx context.Context, pairName, opUID string) (uint64, string, bool, error) {
	if opUID == "" {
		return 0, "", false, nil
	}
	list := controller.List(controller.RevocationGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingLabels{
		revocation.OperationUIDLabel: opUID,
		revocation.TypeLabel:         revocation.TypePair,
		revocation.PairLabel:         pairName,
	}); err != nil {
		return 0, "", false, fmt.Errorf("list revocations of operation %s: %w", opUID, err)
	}
	var (
		generation uint64
		name       string
	)
	for i := range list.Items {
		rev, err := revocation.FromObject(&list.Items[i])
		if err != nil {
			return 0, "", false, err
		}
		if rev.Generation > generation {
			generation, name = rev.Generation, rev.Name
		}
	}
	return generation, name, name != "", nil
}

// writePairRevocation records the new generation of the pair together with the clients that held
// grants or session state of it, read after the generation was raised.
func (r *Revoker) writePairRevocation(ctx context.Context, p Pair, generation uint64, reason, opUID string) (string, error) {
	pairName := p.name()
	var clients []string
	refs, err := r.offlineRefs(ctx, p)
	if err != nil {
		return "", err
	}
	for _, rf := range refs {
		clients = append(clients, rf.clientID)
	}
	tokens, err := r.tokensOf(ctx, pairName)
	if err != nil {
		return "", err
	}
	for _, t := range tokens {
		clients = append(clients, t.clientID)
	}
	sessions, err := r.sessionsOf(ctx, pairName)
	if err != nil {
		return "", err
	}
	for _, s := range sessions {
		clients = append(clients, s.clients...)
	}

	name := revocation.PairName(pairName, generation)
	err = r.writeRevocation(ctx, revocation.Revocation{
		Name:         name,
		Type:         revocation.TypePair,
		ConnectorID:  p.ConnectorID,
		UserID:       p.UserID,
		Subject:      revocation.Subject(p.UserID, p.ConnectorID),
		Generation:   generation,
		Clients:      revocation.SortedClients(clients),
		Reason:       reason,
		Source:       revocation.SourceController,
		CreatedAt:    r.now(),
		OperationUID: opUID,
	})
	return name, err
}

// writeSessionRevocation records the end of the browser session s of the pair, with the clients
// it reached. Dex reads it to send the back-channel logout of the session to these clients: the
// logout token is signed with the keys of Dex.
func (r *Revoker) writeSessionRevocation(ctx context.Context, p Pair, s session, reason, opUID string, res *Result) error {
	name := revocation.SessionName(s.name)
	if err := r.writeRevocation(ctx, revocation.Revocation{
		Name:         name,
		Type:         revocation.TypeSession,
		ConnectorID:  p.ConnectorID,
		UserID:       p.UserID,
		Subject:      revocation.Subject(p.UserID, p.ConnectorID),
		SessionID:    s.name,
		Clients:      revocation.SortedClients(s.clients),
		Reason:       reason,
		Source:       revocation.SourceController,
		CreatedAt:    r.now(),
		OperationUID: opUID,
	}); err != nil {
		return err
	}
	res.addRevocation(name)
	return nil
}

// writeRevocation creates rev. An object of the same name is a revocation written before, and
// counts as written.
func (r *Revoker) writeRevocation(ctx context.Context, rev revocation.Revocation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj, err := rev.Object()
	if err != nil {
		return err
	}
	if err := r.client.Create(ctx, obj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create revocation %s: %w", rev.Name, err)
	}
	writtenCounter.WithLabelValues(rev.Type, rev.Reason).Inc()
	return nil
}

// token is a RefreshToken of the pair as the cache keeps it.
type token struct {
	name       string
	clientID   string
	generation uint64
}

func (r *Revoker) tokensOf(ctx context.Context, pairName string) ([]token, error) {
	list := controller.List(controller.RefreshTokenGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingFields{controller.RefreshTokenPairField: pairName}); err != nil {
		return nil, fmt.Errorf("list refresh tokens of pair %s: %w", pairName, err)
	}
	out := make([]token, 0, len(list.Items))
	for i := range list.Items {
		item := &list.Items[i]
		clientID, _, _ := unstructured.NestedString(item.Object, "clientID")
		out = append(out, token{name: item.GetName(), clientID: clientID, generation: objectGeneration(item)})
	}
	slices.SortFunc(out, func(a, b token) int { return strings.Compare(a.name, b.name) })
	return out, nil
}

// session is an AuthSession of the pair as the cache keeps it. Its name is the session ID, the sid
// claim of the tokens issued under it.
type session struct {
	name       string
	clients    []string
	generation uint64
	// deviceHash is the hash of the device mark of the browser of the session.
	deviceHash string
	// expiry is when the session ends unless used again: the earlier of its idle and absolute
	// expiry, zero when Dex wrote neither.
	expiry time.Time
}

// expired tells whether the session ended at now; Dex may not have deleted it yet.
func (s session) expired(now time.Time) bool {
	return !s.expiry.IsZero() && !now.Before(s.expiry)
}

func (r *Revoker) sessionsOf(ctx context.Context, pairName string) ([]session, error) {
	list := controller.List(controller.AuthSessionGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingFields{controller.AuthSessionPairField: pairName}); err != nil {
		return nil, fmt.Errorf("list auth sessions of pair %s: %w", pairName, err)
	}
	out := make([]session, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, decodeSession(&list.Items[i]))
	}
	slices.SortFunc(out, func(a, b session) int { return strings.Compare(a.name, b.name) })
	return out, nil
}

func decodeSession(obj *unstructured.Unstructured) session {
	states, _, _ := unstructured.NestedMap(obj.Object, "clientStates")
	deviceHash, _, _ := unstructured.NestedString(obj.Object, "deviceHash")
	return session{
		name:       obj.GetName(),
		clients:    slices.Sorted(maps.Keys(states)),
		generation: objectGeneration(obj),
		deviceHash: deviceHash,
		expiry:     sessionExpiry(obj),
	}
}

// sessionExpiry is the earlier of the idle and the absolute expiry of an AuthSession. A time that
// is missing or does not read is left out.
func sessionExpiry(obj *unstructured.Unstructured) time.Time {
	var expiry time.Time
	for _, field := range []string{"idleExpiry", "absoluteExpiry"} {
		value, _, _ := unstructured.NestedString(obj.Object, field)
		parsed, err := controller.ParseRFC3339(value)
		if err != nil || parsed == nil || parsed.Year() <= 1 {
			continue
		}
		if expiry.IsZero() || parsed.Before(expiry) {
			expiry = *parsed
		}
	}
	return expiry
}

// signInPageSize bounds a page of authorization codes or auth requests read at once: the
// controller does not cache them, and every sign-in in the cluster has one of each for a while.
const signInPageSize = 500

// signInsDeleted counts the sign-ins in progress a revocation deleted.
type signInsDeleted struct {
	codes    int
	requests int
}

// deletePendingSignIns deletes the authorization codes and the auth requests Dex holds for the
// pair below generation: sign-ins that finished before the revocation and were not turned into
// tokens yet. Dex refuses them while the UserIdentity keeps the new generation; once the
// UserIdentity of a deleted User goes, Dex reads generation 0 again, and a code or a request
// stamped 0 would pass. Requests that have not authenticated anybody yet carry no user and stay.
//
// They are read through the API reader, a page at a time: the cache does not keep them (the name
// of a code is the code), and they are short-lived.
func (r *Revoker) deletePendingSignIns(ctx context.Context, p Pair, generation uint64) (signInsDeleted, error) {
	var deleted signInsDeleted
	for _, kind := range []struct {
		gvk     schema.GroupVersionKind
		counter *int
	}{
		{gvk: controller.AuthCodeGVK, counter: &deleted.codes},
		{gvk: controller.AuthRequestGVK, counter: &deleted.requests},
	} {
		names, err := r.pendingSignInsOf(ctx, kind.gvk, p, generation)
		if err != nil {
			return deleted, err
		}
		for _, name := range names {
			gone, err := r.deleteDexObject(ctx, kind.gvk, name)
			if err != nil {
				return deleted, err
			}
			if gone {
				*kind.counter++
			}
		}
	}
	return deleted, nil
}

// pendingSignInsOf lists the names of the objects of gvk (AuthCode or AuthRequest) that Dex issued
// to the pair below generation, sorted.
func (r *Revoker) pendingSignInsOf(ctx context.Context, gvk schema.GroupVersionKind, p Pair, generation uint64) ([]string, error) {
	var names []string
	token := ""
	for {
		list := controller.List(gvk)
		opts := []client.ListOption{client.InNamespace(naming.DexNamespace), client.Limit(signInPageSize)}
		if token != "" {
			opts = append(opts, client.Continue(token))
		}
		if err := r.apiReader.List(ctx, list, opts...); err != nil {
			return nil, fmt.Errorf("list %s of pair %s: %w", gvk.Kind, p.name(), err)
		}
		for i := range list.Items {
			item := &list.Items[i]
			if signInOf(item, p) && objectGeneration(item) < generation {
				names = append(names, item.GetName())
			}
		}
		token = list.GetContinue()
		if token == "" {
			break
		}
	}
	slices.Sort(names)
	return names, nil
}

// signInOf tells whether an AuthCode or an AuthRequest names the pair: its connectorID and the
// userID of its claims. An auth request before the user signs in has neither.
func signInOf(obj *unstructured.Unstructured, p Pair) bool {
	connID, _, _ := unstructured.NestedString(obj.Object, "connectorID")
	userID, _, _ := unstructured.NestedString(obj.Object, "claims", "userID")
	return connID == p.ConnectorID && userID == p.UserID
}

// ref is a reference of OfflineSessions.refresh: the refresh token a client of the pair holds, and
// the browser session it was issued under.
type ref struct {
	id        string
	clientID  string
	sessionID string
	// deviceHash is the hash of the device mark of the browser of the sign-in.
	deviceHash string

	// inGrants and inRefresh tell where the OfflineSessions holds the reference: in grants, and as
	// the entry of the client in refresh (the last grant of the client).
	inGrants  bool
	inRefresh bool
}

// offlineRefs reads the references of the OfflineSessions of the pair, by refresh token ID: those of
// grants, every grant of the pair, and of refresh, the last grant of each client, which is all an
// object written before grants has. The object is read fresh: a reference Dex has just added must
// not survive a revocation because the cache has not seen it yet.
func (r *Revoker) offlineRefs(ctx context.Context, p Pair) (map[string]ref, error) {
	obj := controller.Object(controller.OfflineSessionsGVK)
	err := r.apiReader.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: p.name()}, obj)
	if apierrors.IsNotFound(err) {
		return map[string]ref{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get offlinesessions %s: %w", p.name(), err)
	}
	userID, _, _ := unstructured.NestedString(obj.Object, "userID")
	connID, _, _ := unstructured.NestedString(obj.Object, "connID")
	if (userID != "" && userID != p.UserID) || (connID != "" && connID != p.ConnectorID) {
		// Another pair with a colliding name: not this pair's references.
		return map[string]ref{}, nil
	}
	out := map[string]ref{}
	for _, field := range []string{"grants", "refresh"} {
		entries, _, _ := unstructured.NestedMap(obj.Object, field)
		for key, raw := range entries {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id, _ := entry["ID"].(string)
			if id == "" {
				continue
			}
			rf, seen := out[id]
			if !seen {
				clientID, _ := entry["ClientID"].(string)
				sessionID, _ := entry["SessionID"].(string)
				deviceHash, _ := entry["DeviceHash"].(string)
				rf = ref{id: id, clientID: clientID, sessionID: sessionID, deviceHash: deviceHash}
			}
			if field == "grants" {
				rf.inGrants = true
			} else {
				// The key is the client; an entry written before ClientID was kept names it only there.
				rf.clientID, rf.inRefresh = key, true
			}
			out[id] = rf
		}
	}
	return out, nil
}

// removeRef removes the references to the refresh token of rf from the OfflineSessions of the pair:
// its entry of grants, and the entry of its client in refresh if that still names it. JSON Patches
// with a test, one per field: a sign-in that replaced a reference in between keeps it, and a field
// without the entry fails its own patch only. A merge patch from the cache is not an option, the
// cache strips connectorData. Dex reads the grants left of the client from grants.
func (r *Revoker) removeRef(ctx context.Context, pairName string, rf ref) error {
	var paths []string
	if rf.inGrants {
		paths = append(paths, "/grants/"+escapePointer(rf.id))
	}
	if rf.inRefresh {
		paths = append(paths, "/refresh/"+escapePointer(rf.clientID))
	}
	for _, path := range paths {
		patch := fmt.Sprintf(`[{"op":"test","path":%q,"value":%q},{"op":"remove","path":%q}]`, path+"/ID", rf.id, path)
		if err := r.patchJSON(ctx, controller.OfflineSessionsGVK, pairName, patch); err != nil {
			return err
		}
	}
	return nil
}

// patchJSON applies a JSON Patch. A patch that does not apply - the test failed, or the path is gone -
// and a missing object mean somebody else has already changed what the patch was for.
func (r *Revoker) patchJSON(ctx context.Context, gvk schema.GroupVersionKind, name, patch string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	obj := controller.Object(gvk)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	err := r.client.Patch(ctx, obj, client.RawPatch(types.JSONPatchType, []byte(patch)))
	switch {
	case err == nil, apierrors.IsNotFound(err), apierrors.IsInvalid(err):
		return nil
	default:
		return fmt.Errorf("patch %s %s: %w", gvk.Kind, name, err)
	}
}

func (r *Revoker) deleteDexObject(ctx context.Context, gvk schema.GroupVersionKind, name string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	obj := controller.Object(gvk)
	obj.SetNamespace(naming.DexNamespace)
	obj.SetName(name)
	if err := r.client.Delete(ctx, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete %s %s: %w", gvk.Kind, name, err)
	}
	return true, nil
}

// objectGeneration is the generation field of a RefreshToken or an AuthSession: the generation of
// the pair Dex issued it at. Without the field, or with a value that is not a number, it is 0, as
// for everything issued before generations existed.
func objectGeneration(obj *unstructured.Unstructured) uint64 {
	g, err := controller.AsInt64(obj.Object["generation"])
	if err != nil || g < 0 {
		return 0
	}
	return uint64(g)
}

// escapePointer escapes a JSON Pointer reference token (RFC 6901).
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
