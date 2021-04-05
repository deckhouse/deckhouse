package dao

import (
	"fmt"
	"sort"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/rand"

	"upmeter/pkg/check"
	"upmeter/pkg/upmeter/db/context"
)

func Test_ExportEpisodeDAO_Get_NilIfDontExist(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	episodes, err := storage.GetEarliestEpisodes("nonexistent", originsCount)

	g.Expect(episodes).To(BeNil(), "should return nil for nonexistent episodes")
	g.Expect(err).Should(HaveOccurred(), "should return error for nonexistent episodes")
	g.Expect(err).Should(Equal(ErrNotFound), "should be particular error for nonexistent episodes")
}

func Test_ExportEpisodeDAO_Save_CreatesIfDontExist(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	saved, _ := genExportEntities(genOpts{n: 7})

	err := storage.Save(saved)

	g.Expect(err).ShouldNot(HaveOccurred())
}

func Test_ExportEpisodeDAO_SaveSavesGetGets(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	entities, opts := genExportEntities(genOpts{n: 7, origins: randOrigins(originsCount)})
	// Create
	err := storage.Save(entities)
	g.Expect(err).ShouldNot(HaveOccurred(), "should store data successfully")

	// Check what is stored
	fetched, err := storage.GetEarliestEpisodes(*opts.syncID, originsCount)
	g.Expect(err).ShouldNot(HaveOccurred(), "should retrieve data without error")
	g.Expect(fetched).NotTo(BeNil(), "should retrieve non-nil data")

	assertEqualLists(g, fetched, entities)
}

func Test_ExportEpisodeDAO_Get_GetsRepeatedly(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	entities, opts := genExportEntities(genOpts{n: 9, origins: randOrigins(originsCount)})

	// Create
	storage.Save(entities)

	// Check what is stored
	fetched, err := storage.GetEarliestEpisodes(*opts.syncID, originsCount)
	g.Expect(err).ShouldNot(HaveOccurred(), "should retrieve data without error")
	g.Expect(fetched).NotTo(BeNil(), "should retrieve non-nil data")
	assertEqualLists(g, fetched, entities)

	// Check again what is stored
	fetched, err = storage.GetEarliestEpisodes(*opts.syncID, originsCount)
	g.Expect(err).ShouldNot(HaveOccurred(), "should retrieve data without error")
	g.Expect(fetched).NotTo(BeNil(), "should retrieve non-nil data")
	assertEqualLists(g, fetched, entities)
}

func Test_ExportEpisodeDAO_Delete_Deletes(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	entities, opts := genExportEntities(genOpts{n: 9, origins: randOrigins(originsCount)})

	// Create
	err := storage.Save(entities)
	g.Expect(err).ShouldNot(HaveOccurred(), "no error on creation")

	// Delete
	err = storage.Delete(*opts.syncID, opts.slot)
	g.Expect(err).ShouldNot(HaveOccurred(), "no error on deletion")

	// Verify we cannot get it anymore
	fetched, err := storage.GetEarliestEpisodes(*opts.syncID, originsCount)
	g.Expect(fetched).To(BeNil(), "should return nil, not slice")
	g.Expect(err).Should(HaveOccurred(), "should return error for nonexistent episodes")
	g.Expect(err).Should(Equal(ErrNotFound), "should be particular error for nonexistent episodes")
}

func Test_ExportEpisodeDAO_Delete_LetsResurrect(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	entities, opts := genExportEntities(genOpts{n: 9, origins: randOrigins(originsCount)})
	slot := time.Unix(entities[0].Episode.TimeSlot, 0)

	// Create
	err := storage.Save(entities)
	g.Expect(err).ShouldNot(HaveOccurred(), "no error on creation")

	// Delete
	err = storage.Delete(*opts.syncID, slot)
	g.Expect(err).ShouldNot(HaveOccurred(), "no error on deletion")

	// Create again
	err = storage.Save(entities)
	g.Expect(err).ShouldNot(HaveOccurred(), "no error on later creation")

	// Verify existence
	fetched, _ := storage.GetEarliestEpisodes(*opts.syncID, originsCount)
	assertEqualLists(g, fetched, entities)
}

func Test_ExportEpisodeDAO_Save_DoesNotDuplicate(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	entities, opts := genExportEntities(genOpts{n: 7, origins: randOrigins(originsCount)})

	storage.Save(entities)
	storage.Save(entities)

	fetched, _ := storage.GetEarliestEpisodes(*opts.syncID, originsCount)
	assertEqualLists(g, fetched, entities)
}

func Test_ExportEpisodeDAO_Save_UpdatesOnRepeatedUniqueData(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	entities, opts := genExportEntities(genOpts{n: 20, origins: randOrigins(originsCount)})
	// keep all the same except time counters and slot
	first, second := entities[:10], entities[10:]
	for i := range first {
		second[i].Episode.ProbeRef = first[i].Episode.ProbeRef
	}

	storage.Save(first)
	storage.Save(second)

	fetched, _ := storage.GetEarliestEpisodes(*opts.syncID, originsCount)
	assertEqualLists(g, fetched, second)
}

func Test_ExportEpisodeDAO_Save_DoesNotIntersectBySyncID(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	ents1, opts1 := genExportEntities(genOpts{
		n:         13,
		origins:   randOrigins(originsCount),
		slotInt64: time.Now().Unix()})
	ents2, opts2 := genExportEntities(genOpts{
		n:         17,
		origins:   randOrigins(originsCount),
		slotInt64: opts1.slotInt64})

	storage.Save(ents1)
	storage.Save(ents2)

	fetched1, _ := storage.GetEarliestEpisodes(*opts1.syncID, originsCount)
	assertEqualLists(g, fetched1, ents1)

	fetched2, _ := storage.GetEarliestEpisodes(*opts2.syncID, originsCount)
	assertEqualLists(g, fetched2, ents2)
}

func Test_ExportEpisodeDAO_Get_AlwaysFetchesEarliest(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 1

	// Setup: generate data
	earlyEntities, opts1 := genExportEntities(genOpts{n: 13, origins: randOrigins(originsCount), slotInt64: 1000})
	middleEntities, opts2 := genExportEntities(genOpts{n: 17, origins: randOrigins(originsCount), slotInt64: 2000, syncID: opts1.syncID})
	lateEntities, _ := genExportEntities(genOpts{n: 11, origins: randOrigins(originsCount), slotInt64: 3000, syncID: opts1.syncID})

	// Save
	storage.Save(lateEntities)   // 1
	storage.Save(earlyEntities)  // 2 the sequence is important, "early" is saved in the middle
	storage.Save(middleEntities) // 3

	// Assert the fetching order
	earlyFetched, _ := storage.GetEarliestEpisodes(*opts1.syncID, originsCount)
	assertEqualLists(g, earlyFetched, earlyEntities)
	storage.Delete(*opts1.syncID, opts1.slot)

	middleFetched, _ := storage.GetEarliestEpisodes(*opts1.syncID, originsCount)
	assertEqualLists(g, middleFetched, middleEntities)
	storage.Delete(*opts2.syncID, opts2.slot)

	lateFetched, _ := storage.GetEarliestEpisodes(*opts1.syncID, originsCount)
	assertEqualLists(g, lateFetched, lateEntities)
}

func Test_ExportEpisodeDAO_MultipleOrigins_Save_MergesOrigins(t *testing.T) {
	g := NewWithT(t)

	tt := []struct {
		desiredCount  int
		expectedCount int
		origins       []string
	}{
		{
			desiredCount:  1,
			expectedCount: 1,
			origins:       []string{"a"},
		}, {
			desiredCount:  1,
			expectedCount: 2,
			origins:       []string{"a", "b"},
		}, {
			desiredCount:  2,
			expectedCount: 2,
			origins:       []string{"a", "b"},
		}, {
			desiredCount:  2,
			expectedCount: 2, // duplicates
			origins:       []string{"a", "b", "a"},
		}, {
			desiredCount:  2,
			expectedCount: 4,
			origins:       []string{"a", "b", "c", "d"},
		}, {
			desiredCount:  3,
			expectedCount: 3,
			origins:       []string{"a", "b", "c"},
		}, {
			desiredCount:  3,
			expectedCount: 4,
			origins:       []string{"d", "a", "b", "c"},
		}, {
			desiredCount:  3,
			expectedCount: 5,
			origins:       []string{"e", "d", "a", "b", "c"},
		}, {
			desiredCount:  3,
			expectedCount: 4, // duplicates
			origins:       []string{"b", "d", "a", "b", "c"},
		},
	}

	for _, t := range tt {
		storage := newExportEpisodesDAO()
		saved, opts := genExportEntities(genOpts{n: 1})

		// Save all origins sequentially
		for _, o := range t.origins {
			saved[0].Origins = newSet(o)
			storage.Save(saved)
		}

		// Verify we have both origins
		got, _ := storage.GetEarliestEpisodes(*opts.syncID, t.desiredCount)

		desc := fmt.Sprintf("desired=%d, expected=%d, included=%s",
			t.desiredCount, t.expectedCount, newSet(t.origins...).String())

		g.Expect(len(got)).To(Equal(1), "should be one record for "+desc)
		g.Expect(got[0].Origins.Size()).To(Equal(t.expectedCount), "should have expected origins size for "+desc)
	}
}

func Test_ExportEpisodeDAO_MultipleOrigins_Get_PrefersEarliestUnfulfilledDespiteLaterFulfilled(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()

	var (
		originsCount = 3
		nEarly       = 2

		lateSlot      = time.Now()
		fulfilledSlot = lateSlot.Add(-10 * time.Minute)
		earlySlot     = fulfilledSlot.Add(-10 * time.Minute)

		lateOrigins      = newSet("3", "5")
		fulfilledOrigins = newSet("1", "2", "3")
		earlyOrigins     = newSet("1")
	)

	// Setup: generate data
	late, opts1 := genExportEntities(genOpts{
		n:         4,
		slotInt64: lateSlot.Unix(),
		origins:   &lateOrigins,
	})
	syncId := *opts1.syncID
	fulfilled, _ := genExportEntities(genOpts{
		syncID:    &syncId,
		n:         7,
		slotInt64: fulfilledSlot.Unix(),
		origins:   &fulfilledOrigins,
	})
	early, _ := genExportEntities(genOpts{
		syncID:    &syncId,
		n:         nEarly,
		slotInt64: earlySlot.Unix(),
		origins:   &earlyOrigins,
	})

	storage.Save(late)
	storage.Save(early)
	storage.Save(fulfilled)

	// Verify we have both origins
	got, _ := storage.GetEarliestEpisodes(syncId, originsCount)

	g.Expect(got).To(HaveLen(nEarly), "should find only one entity")
	g.Expect(got[0].Episode.TimeSlot).To(Equal(earlySlot.Unix()), "should have earliest timeslot")
	g.Expect(got[0].Origins.Size()).To(Equal(earlyOrigins.Size()), "should have unfulfilled origins")
}

func Test_ExportEpisodeDAO_MultipleOrigins_Get_ReturnsNothingWhenNoneFulfilled(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 3

	// Setup: generate data
	slot := time.Now().Unix() // must be fresh, not to match -24h threshold
	saved, opts := genExportEntities(genOpts{n: 10, origins: randOrigins(2), slotInt64: slot})

	storage.Save(saved)

	// Verify we have both origins
	got, err := storage.GetEarliestEpisodes(*opts.syncID, originsCount)

	g.Expect(got).To(BeNil(), "should return nil in place of entries")
	g.Expect(err).To(HaveOccurred(), "should return error")
	g.Expect(err).To(Equal(ErrNotFound), `should return "not found" error`)

}

func Test_ExportEpisodeDAO_MultipleOrigins_Get_ReturnsExpiredEvenWhenNoneFulfilled(t *testing.T) {
	g := NewWithT(t)
	storage := newExportEpisodesDAO()
	originsCount := 3

	// Setup: generate data
	var (
		h22ago      = time.Now().Add(-22 * time.Hour).Unix()
		h23ago      = time.Now().Add(-23 * time.Hour).Unix()
		hExpiredAgo = time.Now().Add(-24 * time.Hour).Unix()
		nExpired    = 5
	)

	late, opts := genExportEntities(genOpts{
		n:         4,
		slotInt64: h22ago,
		origins:   randOrigins(1),
	})
	mid, _ := genExportEntities(genOpts{
		n:         6,
		slotInt64: h23ago,
		origins:   randOrigins(1),
		syncID:    opts.syncID,
	})
	expired, _ := genExportEntities(genOpts{
		n:         nExpired,
		slotInt64: hExpiredAgo,
		origins:   randOrigins(1),
		syncID:    opts.syncID,
	})

	storage.Save(late)
	storage.Save(expired)
	storage.Save(mid)

	// Verify we have both origins
	got, err := storage.GetEarliestEpisodes(*opts.syncID, originsCount)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(HaveLen(nExpired), "should find expired N entities")
	g.Expect(got[0].Episode.TimeSlot).To(Equal(hExpiredAgo), "should have minimal timeslot")
}

// UTILS

func assertEqualLists(g *WithT, got, want []ExportEpisodeEntity) {
	g.Expect(len(got)).To(Equal(len(want)), "should fetch the same number as was saved")

	sort.Sort(ByProbeRef(got))
	sort.Sort(ByProbeRef(want))

	for i := 0; i < len(got); i++ {
		assertExportEpisodesEqual(g, got[i], want[i])
	}
}

// ByProbeRef implements sort.Interface based on the probe reference.
type ByProbeRef []ExportEpisodeEntity

func (a ByProbeRef) Len() int { return len(a) }
func (a ByProbeRef) Less(i, j int) bool {
	return a[i].Episode.ProbeRef.Id() < a[j].Episode.ProbeRef.Id()
}
func (a ByProbeRef) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// assertExportEpisodesEqual does not check ID field
func assertExportEpisodesEqual(g *WithT, e1, e2 ExportEpisodeEntity) {

	g.Expect(e1.Origins.String()).To(Equal(e2.Origins.String()))
	g.Expect(e1.SyncID).To(Equal(e2.SyncID))

	a := e1.Episode
	b := e2.Episode

	g.Expect(a.ProbeRef.Group).To(Equal(b.ProbeRef.Group))
	g.Expect(a.ProbeRef.Probe).To(Equal(b.ProbeRef.Probe))
	g.Expect(a.TimeSlot).To(Equal(b.TimeSlot))

	g.Expect(a.SuccessSeconds).To(Equal(b.SuccessSeconds))
	g.Expect(a.FailSeconds).To(Equal(b.FailSeconds))
	g.Expect(a.UnknownSeconds).To(Equal(b.UnknownSeconds))
	g.Expect(a.NoDataSeconds).To(Equal(b.NoDataSeconds))

}

func setSlot(entities []ExportEpisodeEntity, slot int64) {
	for i := range entities {
		entities[i].Episode.TimeSlot = slot
	}
}

type genOpts struct {
	n         int
	syncID    *string
	slotInt64 int64
	slot      time.Time
	origins   *set
}

func genExportEntities(opts genOpts) ([]ExportEpisodeEntity, genOpts) {

	if opts.syncID == nil {
		syncID := rand.String(5)
		opts.syncID = &syncID
	}

	// generate
	var entities []ExportEpisodeEntity
	for _, ep := range newRandomDowntimeEpisodes(opts.n) {
		entity := ExportEpisodeEntity{
			Episode: *ep,
			SyncID:  *opts.syncID,
		}
		if opts.origins != nil {
			for o := range *opts.origins {
				entity.AddOrigin(o)
			}
		}
		entities = append(entities, entity)
	}

	if opts.slotInt64 > 0 {
		setSlot(entities, opts.slotInt64)
	} else {
		opts.slotInt64 = entities[0].Episode.TimeSlot
	}
	opts.slot = time.Unix(opts.slotInt64, 0)

	return entities, opts
}

func newRandomDowntimeEpisodes(n int) []*check.DowntimeEpisode {

	// shared data
	var slotSize int64 = 30
	slot := time.Now().Truncate(time.Duration(slotSize) * time.Second).Unix()

	episodes := make([]*check.DowntimeEpisode, 0)
	for ; n > 0; n-- {
		// different data
		var (
			group = rand.String(3)
			probe = rand.String(7)

			success = rand.Int63nRange(0, slotSize)
			fail    = rand.Int63nRange(0, slotSize-success)
			unknown = rand.Int63nRange(0, slotSize-success-fail)
			nodata  = slotSize - success - fail - unknown
		)

		ep := check.DowntimeEpisode{
			SuccessSeconds: success,
			FailSeconds:    fail,
			UnknownSeconds: unknown,
			NoDataSeconds:  nodata,
			ProbeRef:       check.ProbeRef{Group: group, Probe: probe},
			TimeSlot:       slot,
		}

		episodes = append(episodes, &ep)
	}

	return episodes
}

//nolint:unparam
func newExportEpisodesDAO() *ExportEpisodesDAO {
	db := context.NewDbContext()
	// ctx.Connect("file::memory:?cache=shared")
	// err := db.Connect("file:export.sqlite")
	err := db.Connect("file::memory:")
	if err != nil {
		panic(err)
	}

	createCtx := db.Start()
	defer createCtx.Stop()

	_, err = createCtx.StmtRunner().Exec(sqlCreateExportTable)
	if err != nil {
		panic(err)
	}

	return NewExportEpisodesDAO(db)
}

func randOrigins(n int) *set {
	s := newSet()
	for n > 0 {
		s.Add(rand.String(6))
		n--
	}
	return &s
}
