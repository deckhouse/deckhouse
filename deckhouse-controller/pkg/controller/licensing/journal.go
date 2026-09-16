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
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// sampleDue reports whether the observation window needs a new sample. The
// window is hourly; a reconcile landing slightly early still samples, otherwise
// every resync would push the next sample another ten minutes out.
//
// A sample dated in the future is due immediately. It can only come from a clock
// that was wrong when it was written, and waiting it out would stop sampling
// until that date arrives.
func sampleDue(journal *licensing.Journal, now time.Time) bool {
	n := len(journal.Samples)
	if n == 0 {
		return true
	}
	last := journal.Samples[n-1].At
	return last.After(now) || now.Sub(last) >= minSampleAge
}

// stats reads one metric out of the window and rounds it for publication.
//
// The extrapolation is a least squares fit, so a window that did not change at
// all still moves in the last bits of the mantissa from one reconcile to the
// next. Published raw, that noise rewrites the EffectiveLicense status on every
// tick forever.
func stats(journal *licensing.Journal, name string, now, horizon time.Time) licensing.MetricValue {
	v := journal.Stats(name, now, horizon, retention)
	return licensing.MetricValue{
		Instant:      round3(v.Instant),
		Avg7d:        round3(v.Avg7d),
		Extrapolated: round3(v.Extrapolated),
	}
}

// round3 is the precision every published consumption value is cut to. Three
// decimals is far finer than a whole core, and coarse enough to be stable.
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

func (r *reconciler) journalKey() types.NamespacedName {
	return types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: journalConfigMapName}
}

// loadJournal reads the observation window. It goes through the uncached reader:
// the window is appended to, so reading a stale copy would drop observations.
//
// The second return value is the broken payload of a journal that could not be
// decoded, to be kept aside on the next save.
func (r *reconciler) loadJournal(ctx context.Context) (*licensing.Journal, string, error) {
	var cm corev1.ConfigMap
	err := r.apiReader.Get(ctx, r.journalKey(), &cm)
	if apierrors.IsNotFound(err) {
		return new(licensing.Journal), "", nil
	}
	if err != nil {
		return nil, "", err
	}

	journal := new(licensing.Journal)
	raw := cm.Data[journalField]
	if raw == "" {
		return journal, "", nil
	}
	if err := json.Unmarshal([]byte(raw), journal); err != nil {
		// ponytail: a broken journal is restarted rather than repaired. Only the
		// counter is salvaged, because a seq that went backwards is the one part
		// the license server cannot shrug off; the window itself refills in a
		// week.
		journal = new(licensing.Journal)
		var salvage struct {
			Seq uint64 `json:"seq"`
		}
		if json.Unmarshal([]byte(raw), &salvage) == nil {
			journal.Seq = salvage.Seq
		}
		r.logger.Error("consumption journal is corrupt; observations restart, "+
			"re-registration in the license server may be required",
			slog.Uint64("salvagedSeq", journal.Seq), log.Err(err))
		return journal, raw, nil
	}
	return journal, "", nil
}

// saveJournal writes the window back. corrupt, when set, is the payload of a
// journal that failed to decode: it is parked under its own key so that whoever
// looks into the incident still has the bytes, and it is never parsed again.
func (r *reconciler) saveJournal(ctx context.Context, journal *licensing.Journal, corrupt string) error {
	raw, err := json.Marshal(journal)
	if err != nil {
		return fmt.Errorf("marshal journal: %w", err)
	}

	key := r.journalKey()
	data := map[string]string{journalField: string(raw)}

	var cm corev1.ConfigMap
	err = r.apiReader.Get(ctx, key, &cm)
	if apierrors.IsNotFound(err) {
		if corrupt != "" {
			data[journalCorruptField] = corrupt
		}
		return r.Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Labels: objectLabels},
			Data:       data,
		})
	}
	if err != nil {
		return err
	}

	switch {
	case corrupt != "":
		data[journalCorruptField] = corrupt
	case cm.Data[journalCorruptField] != "":
		// An earlier corruption keeps its evidence until someone removes it.
		data[journalCorruptField] = cm.Data[journalCorruptField]
	}

	cm.Labels = objectLabels
	cm.Data = data
	return r.Update(ctx, &cm)
}

// buildRegistrationRequest signs the current consumption and the accepted record
// ids with the cluster key. The request declares the cluster identity with an
// inline jwk while no record has been accepted yet, and references it by
// thumbprint afterwards: the license server already knows the key by then.
func (r *reconciler) buildRegistrationRequest(
	priv ed25519.PrivateKey,
	clusterID string,
	res licensing.Result,
	values map[string]licensing.MetricValue,
	seq uint64,
	now time.Time,
) (string, error) {
	accepted := make([]string, 0, len(res.Records))
	registered := false
	for _, rec := range res.Records {
		if !rec.Accepted {
			continue
		}
		accepted = append(accepted, rec.ID)
		if rec.Type == licensing.TypeWorkload {
			registered = true
		}
	}
	sort.Strings(accepted)

	return licensing.BuildRegistrationRequest(licensing.RegistrationInput{
		ClusterID: clusterID,
		// ponytail: public_domain is omitted. It lives in the global module
		// values, which this controller does not read, and it is an optional
		// hint for the sales side rather than part of the identity.
		Build:      r.build,
		DKPVersion: r.dkpVersion,
		Seq:        seq,
		JTI:        newJTI(),
		IssuedAt:   now,
		Metrics:    values,
		Records:    accepted,
		Key:        priv,
		IncludeJWK: !registered,
	})
}

// newJTI is the unique id of one registration request.
func newJTI() string { return uuid.New().String() }

// horizonOf is the nearest future expiry among the active records, which is the
// point the consumption trend is extrapolated to. Without one, a month ahead.
func horizonOf(res licensing.Result, now time.Time) time.Time {
	var nearest *time.Time
	for _, rec := range res.Records {
		if !licensing.Active(rec, now) || rec.ExpireAt == nil || !rec.ExpireAt.After(now) {
			continue
		}
		if nearest == nil || rec.ExpireAt.Before(*nearest) {
			nearest = rec.ExpireAt
		}
	}
	if nearest == nil {
		return now.Add(defaultHorizon)
	}
	return *nearest
}

// requeueAfter is the soonest of the resync, the next observation and the next
// point where the policy changes by itself.
func requeueAfter(res licensing.Result, journal *licensing.Journal, now time.Time) time.Duration {
	next := resyncPeriod

	if n := len(journal.Samples); n > 0 {
		if d := journal.Samples[n-1].At.Add(minSampleAge).Sub(now); d > 0 && d < next {
			next = d
		}
	}
	for _, seg := range res.Timeline {
		if d := seg.From.Sub(now); d > 0 {
			if d < next {
				next = d
			}
			break
		}
	}

	// A breakpoint that is seconds away is not worth a hot loop around it.
	if next < time.Minute {
		next = time.Minute
	}
	return next
}
