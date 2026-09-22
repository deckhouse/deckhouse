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

package licensing

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// requestPayload is the part of a published registration request this controller
// reads back, to decide whether the published one still describes the cluster.
// requestMaxAge keeps the published request inside the anti-replay window of
// the license server (it rejects an iat older than 48 hours): once the file is
// this old it is rebuilt even when nothing about the cluster changed.
const requestMaxAge = 24 * time.Hour

type requestPayload struct {
	Seq        uint64           `json:"seq"`
	IAT        string           `json:"iat"`
	Metrics    map[string]int64 `json:"metrics"`
	ActiveKeys []string         `json:"active_keys"`
	Records    []string         `json:"records"`
}

// decodeRequest reads back a request this controller published. The signature is
// not checked: it is our own status field, and a failure to decode only costs a
// reissue.
func decodeRequest(request string) (*requestPayload, map[string]any, bool) {
	tok, err := licensing.Parse(request)
	if err != nil {
		return nil, nil, false
	}
	var payload requestPayload
	if err := json.Unmarshal(tok.Payload, &payload); err != nil {
		return nil, nil, false
	}
	return &payload, tok.Header, true
}

// requestStale reports whether the published registration request no longer
// describes the cluster. The license server reads the records and the installed
// keys out of it to issue a reissue, and the metrics to show the customer what
// the cluster consumes, so all three have to be current (specification 10.6).
// issuedAt reports the iat of a request the controller signed itself, so the
// payload is read without checking the signature. A zero time means unusable.
func (p *requestPayload) issuedAt() time.Time {
	iat, err := time.Parse(time.RFC3339, p.IAT)
	if err != nil {
		return time.Time{}
	}
	return iat
}

func requestIssuedAt(request string) time.Time {
	payload, _, ok := decodeRequest(request)
	if !ok {
		return time.Time{}
	}
	return payload.issuedAt()
}

func requestStale(request string, res licensing.Result, now time.Time) bool {
	payload, header, ok := decodeRequest(request)
	if !ok {
		return true
	}
	if iat := payload.issuedAt(); iat.IsZero() || !now.Before(iat.Add(requestMaxAge)) {
		return true
	}
	_, hasJWK := header["jwk"]
	if hasJWK != (len(res.ActiveKeys) == 0) {
		return true
	}
	return !equalStrings(payload.Records, res.AcceptedRecords) ||
		!equalStrings(payload.ActiveKeys, res.ActiveKeys) ||
		!reflect.DeepEqual(payload.Metrics, res.Consumption)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *reconciler) buildRegistrationRequest(
	priv ed25519.PrivateKey,
	clusterID string,
	res licensing.Result,
	seq uint64,
	now time.Time,
) (string, error) {
	return licensing.BuildRegistrationRequest(licensing.RegistrationInput{
		ClusterID: clusterID,
		// ponytail: public_domain is omitted. It lives in the global module
		// values, which this controller does not read, and it is an optional
		// hint for the sales side rather than part of the identity.
		Build:      r.build,
		DKPVersion: r.dkpVersion,
		Seq:        seq,
		JTI:        uuid.New().String(),
		IssuedAt:   now,
		Metrics:    res.Consumption,
		Records:    res.AcceptedRecords,
		ActiveKeys: res.ActiveKeys,
		Key:        priv,
		// A request carrying jwk declares a new identity, and it is only sent
		// while the cluster holds no key at all.
		IncludeJWK: len(res.ActiveKeys) == 0,
	})
}

func (r *reconciler) seqKey() types.NamespacedName {
	return types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: seqConfigMapName}
}

// loadSeq reads the registration counter. It is the one piece of licensing state
// that has to outlive both the status and a restart: the license server reads
// seq as strictly growing, and a counter that went backwards makes every further
// registration look like a replay.
//
// The read bypasses the cache: a stale value would reissue an already used seq.
func (r *reconciler) loadSeq(ctx context.Context) (uint64, error) {
	var cm corev1.ConfigMap
	err := r.apiReader.Get(ctx, r.seqKey(), &cm)
	if apierrors.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	raw := cm.Data[seqField]
	if raw == "" {
		return 0, nil
	}
	seq, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		// ponytail: an unreadable counter restarts from the published request
		// rather than from zero, and from zero only if that is gone too.
		r.logger.Error("registration counter is corrupt, it restarts",
			"value", raw)
		return 0, nil
	}
	return seq, nil
}

func (r *reconciler) saveSeq(ctx context.Context, seq uint64) error {
	key := r.seqKey()
	data := map[string]string{seqField: strconv.FormatUint(seq, 10)}

	var cm corev1.ConfigMap
	err := r.apiReader.Get(ctx, key, &cm)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Labels: objectLabels},
			Data:       data,
		})
	}
	if err != nil {
		return err
	}

	cm.Labels = objectLabels
	cm.Data = data
	if err := r.Update(ctx, &cm); err != nil {
		return fmt.Errorf("update registration counter: %w", err)
	}
	return nil
}

// requeueAfter is the soonest moment the policy can change on its own: the next
// record boundary, the end of the over-limit window, or the resync that bounds
// how stale anything can get while nothing happens (specification 10.6).
func requeueAfter(res licensing.Result, th licensing.Thresholds, issuedAt, now time.Time) time.Duration {
	next := resyncPeriod

	consider := func(at time.Time) {
		if d := at.Sub(now); d > 0 && d < next {
			next = d
		}
	}
	for _, rec := range res.Records {
		consider(rec.StartAt)
		if rec.ExpireAt != nil {
			consider(*rec.ExpireAt)
		}
	}
	if res.OverLimitSince != nil {
		consider(res.OverLimitSince.Add(th.OverLimitWindow))
	}
	if res.Key != nil && res.Key.ValidUntil != nil {
		consider(res.Key.ValidUntil.Add(-th.ExpiringSoon))
	}
	consider(issuedAt.Add(requestMaxAge))

	// A breakpoint that is seconds away is not worth a hot loop around it.
	if next < time.Minute {
		next = time.Minute
	}
	return next
}
