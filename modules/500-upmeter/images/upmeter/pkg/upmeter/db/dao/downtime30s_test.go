package dao

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/gomega"

	"upmeter/pkg/probe/types"
	"upmeter/pkg/upmeter/db/util"
)

func Test_Downtime30s_CRUD(t *testing.T) {
	g := NewWithT(t)
	var err error

	dao := NewDowntime30sDao()
	err = util.Connect(":memory:", func(dbh *sql.DB) {
		dao.Dbh = dbh
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(dao.Dbh).ShouldNot(BeNil())

	episodes := []types.DowntimeEpisode{
		{
			ProbeRef: types.ProbeRef{
				Group: "nginx",
				Probe: "main",
			},
			TimeSlot:       360,
			SuccessSeconds: 0,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "nginx",
				Probe: "redirect",
			},
			TimeSlot:       360,
			SuccessSeconds: 22,
		},
		{
			ProbeRef: types.ProbeRef{
				Group: "api",
				Probe: "configmap",
			},
			TimeSlot:       360,
			SuccessSeconds: 12,
		},
	}

	// 1. Seed some records
	err = dao.SaveBatch(episodes)
	g.Expect(err).ShouldNot(HaveOccurred(), "episodes should be saved to db")

	// 2. Read them back
	list, err := dao.ListByTimestamp(360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	// 3. Update Duration for each record
	for _, downtime := range list {
		downtime.DowntimeEpisode.SuccessSeconds = 100
		err := dao.Update(downtime.Rowid, downtime.DowntimeEpisode)
		g.Expect(err).ShouldNot(HaveOccurred(), "should update downtime with new duration")
	}

	// 4. Get updated records
	list, err = dao.ListByTimestamp(360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	// 5. Check that duration value is updated
	for _, downtime := range list {
		g.Expect(downtime.DowntimeEpisode.SuccessSeconds).Should(BeEquivalentTo(100), "should have updated duration")
	}

	// 6. Select group and probe names
	probeRefs, err := dao.ListGroupProbe()
	g.Expect(err).ShouldNot(HaveOccurred(), "should return a list of group and probe")
	g.Expect(probeRefs).Should(HaveLen(len(episodes)))
	g.Expect(probeRefs[0].Group).Should(Equal("api"))

	// 6. Delete everything earlier than timestamp
	err = dao.DeleteEarlierThen(360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should delete earlier records")

	// 7. Check that nothing is deleted
	list, err = dao.ListByTimestamp(360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list episodes after deletion")
	g.Expect(list).Should(HaveLen(len(episodes)), "should return same quantity")

	// 8. Delete everything earlier than timestamp "in future"
	err = dao.DeleteEarlierThen(390)
	g.Expect(err).ShouldNot(HaveOccurred(), "should delete all records")

	// 9. Check that nothing is deleted
	list, err = dao.ListByTimestamp(360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list episodes after deletion")
	g.Expect(len(list)).Should(Equal(0), "should return empty list")
}
