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

package entity

import (
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/db"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
	"d8.io/upmeter/pkg/db/migrations"
)

func saveEpisodesTx(t *testing.T, dbctx *dbcontext.DbContext, episodes []check.Episode) []*check.Episode {
	t.Helper()
	var saved []*check.Episode
	err := db.WithTx(dbctx, func(tx *dbcontext.DbContext) error {
		var e error
		saved, e = Save30sEpisodes(tx, episodes)
		return e
	})
	if err != nil {
		t.Fatalf("Save30sEpisodes failed: %v", err)
	}
	return saved
}

func update5mTx(t *testing.T, dbctx *dbcontext.DbContext, episodes30s []*check.Episode) []*check.Episode {
	t.Helper()
	var saved []*check.Episode
	err := db.WithTx(dbctx, func(tx *dbcontext.DbContext) error {
		var e error
		saved, e = Update5mEpisodes(tx, episodes30s)
		return e
	})
	if err != nil {
		t.Fatalf("Update5mEpisodes failed: %v", err)
	}
	return saved
}

// Two agents send the same (slot, probe). The stored 30s episode must be the Combine of the two.
func Test_Save30sEpisodes_Batch_CombinesAcrossAgents(t *testing.T) {
	g := NewWithT(t)
	dbctx := migrations.GetTestMemoryDatabase(t, "../../db/migrations/server")

	slot := time.Now().Truncate(30 * time.Second)
	ref := check.ProbeRef{Group: "nginx", Probe: "main"}

	epA := check.Episode{ProbeRef: ref, TimeSlot: slot, Up: 20 * time.Second, Down: 10 * time.Second}
	epB := check.Episode{ProbeRef: ref, TimeSlot: slot, Up: 25 * time.Second, Down: 5 * time.Second}

	saveEpisodesTx(t, dbctx, []check.Episode{epA})
	saved := saveEpisodesTx(t, dbctx, []check.Episode{epB})

	want := epB.Combine(epA, 30*time.Second)

	g.Expect(saved).To(HaveLen(1))
	g.Expect(*saved[0]).To(Equal(want), "returned episode must be the combined one")

	conn := dbctx.Start()
	defer conn.Stop()
	stored, err := dao.NewEpisodeDao30s(conn).ListEpisodesBySlot(slot)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(stored).To(ConsistOf(want), "stored episode must be the combined one")
}

// A batch carrying several slots must persist every slot in one go.
func Test_Save30sEpisodes_Batch_MultipleSlots(t *testing.T) {
	g := NewWithT(t)
	dbctx := migrations.GetTestMemoryDatabase(t, "../../db/migrations/server")

	ref := check.ProbeRef{Group: "nginx", Probe: "main"}
	slot0 := time.Now().Truncate(30 * time.Second)
	slot1 := slot0.Add(30 * time.Second)
	slot2 := slot0.Add(60 * time.Second)

	batch := []check.Episode{
		{ProbeRef: ref, TimeSlot: slot0, Up: 30 * time.Second},
		{ProbeRef: ref, TimeSlot: slot1, Up: 30 * time.Second},
		{ProbeRef: ref, TimeSlot: slot2, Up: 30 * time.Second},
	}

	saved := saveEpisodesTx(t, dbctx, batch)
	g.Expect(saved).To(HaveLen(3))

	conn := dbctx.Start()
	defer conn.Stop()
	dao30 := dao.NewEpisodeDao30s(conn)
	for _, slot := range []time.Time{slot0, slot1, slot2} {
		stored, err := dao30.ListEpisodesBySlot(slot)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(stored).To(HaveLen(1), "slot %s must be stored", slot)
	}
}

// 5m episode must aggregate the 30s sub-slots of its window.
func Test_Update5mEpisodes_Batch_Aggregates(t *testing.T) {
	g := NewWithT(t)
	dbctx := migrations.GetTestMemoryDatabase(t, "../../db/migrations/server")

	ref := check.ProbeRef{Group: "nginx", Probe: "main"}
	// Two 30s sub-slots within the same 5m window.
	window := time.Now().Truncate(5 * time.Minute)
	sub0 := window
	sub1 := window.Add(30 * time.Second)

	batch := []check.Episode{
		{ProbeRef: ref, TimeSlot: sub0, Up: 30 * time.Second},
		{ProbeRef: ref, TimeSlot: sub1, Up: 20 * time.Second, Down: 10 * time.Second},
	}

	saved30s := saveEpisodesTx(t, dbctx, batch)
	saved5m := update5mTx(t, dbctx, saved30s)

	g.Expect(saved5m).To(HaveLen(1))
	got := *saved5m[0]

	g.Expect(got.ProbeRef).To(Equal(ref))
	g.Expect(got.TimeSlot.Unix()).To(Equal(window.Unix()), "5m slot must be the truncated window start")
	g.Expect(got.Up).To(Equal(50 * time.Second))
	g.Expect(got.Down).To(Equal(10 * time.Second))
	g.Expect(got.NoData).To(Equal(5*time.Minute - 60*time.Second))

	// Re-running with no new data must be a no-op (ErrNotChanged path → nothing returned).
	saved5mAgain := update5mTx(t, dbctx, saved30s)
	g.Expect(saved5mAgain).To(BeEmpty(), "unchanged 5m episodes must not be rewritten")
}
