package dao

import (
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/gomega"

	"upmeter/pkg/check"
	"upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/migrations"
)

var t360 = time.Unix(360, 0)

func Test_Downtime30s_CRUD(t *testing.T) {
	g := NewWithT(t)
	var err error

	dbCtx := context.NewDbContext()
	err = dbCtx.Connect("file::memory:")
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrations.MigrateDatabase(dbCtx, "../migrations/server")
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	dao30s := NewEpisodeDao30s(daoCtx)

	episodes := []check.Episode{
		{
			ProbeRef: check.ProbeRef{
				Group: "nginx",
				Probe: "main",
			},
			TimeSlot: t360,
			Up:       0,
		},
		{
			ProbeRef: check.ProbeRef{
				Group: "nginx",
				Probe: "redirect",
			},
			TimeSlot: t360,
			Up:       22,
		},
		{
			ProbeRef: check.ProbeRef{
				Group: "api",
				Probe: "configmap",
			},
			TimeSlot: t360,
			Up:       12,
		},
	}

	// 1. Seed some records
	err = dao30s.SaveBatch(episodes)
	g.Expect(err).ShouldNot(HaveOccurred(), "episodes should be saved to db")

	// 2. Read them back
	list, err := dao30s.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	// 3. Update Duration for each record
	for _, downtime := range list {
		downtime.Episode.Up = 100
		err := dao30s.Update(downtime.Rowid, downtime.Episode)
		g.Expect(err).ShouldNot(HaveOccurred(), "should update downtime with new duration")
	}

	// 4. Get updated records
	list, err = dao30s.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	// 5. Check that duration value is updated
	for _, downtime := range list {
		g.Expect(downtime.Episode.Up).Should(BeEquivalentTo(100), "should have updated duration")
	}

	// 6. Select group and probe names
	probeRefs, err := dao30s.ListGroupProbe()
	g.Expect(err).ShouldNot(HaveOccurred(), "should return a list of group and probe")
	g.Expect(probeRefs).Should(HaveLen(len(episodes)))
	g.Expect(probeRefs[0].Group).Should(Equal("api"))

	// 6. Delete everything earlier than timestamp
	err = dao30s.DeleteEarlierThen(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should delete earlier records")

	// 7. Check that nothing is deleted
	list, err = dao30s.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list episodes after deletion")
	g.Expect(list).Should(HaveLen(len(episodes)), "should return same quantity")

	// 8. Delete everything earlier than timestamp "in future"
	err = dao30s.DeleteEarlierThen(t360.Add(30 * time.Second))
	g.Expect(err).ShouldNot(HaveOccurred(), "should delete all records")

	// 9. Check that nothing is deleted
	list, err = dao30s.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list episodes after deletion")
	g.Expect(len(list)).Should(Equal(0), "should return empty list")
}

func Test_Downtime30s_FileWrite(t *testing.T) {
	g := NewWithT(t)
	var err error

	dbFile := fmt.Sprintf("test-%d.db.sqlite", time.Now().Unix())

	dbCtx := context.NewDbContext()
	err = dbCtx.Connect(dbFile)
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrations.MigrateDatabase(dbCtx, "../migrations/server")
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx := dbCtx.Start()
	dao30s := NewEpisodeDao30s(daoCtx)

	episodes := []check.Episode{
		{
			ProbeRef: check.ProbeRef{
				Group: "nginx",
				Probe: "main",
			},
			TimeSlot: t360,
			Up:       0,
		},
		{
			ProbeRef: check.ProbeRef{
				Group: "nginx",
				Probe: "redirect",
			},
			TimeSlot: t360,
			Up:       22,
		},
		{
			ProbeRef: check.ProbeRef{
				Group: "api",
				Probe: "configmap",
			},
			TimeSlot: t360,
			Up:       12,
		},
	}

	// 1. Seed some records
	err = dao30s.SaveBatch(episodes)
	g.Expect(err).ShouldNot(HaveOccurred(), "episodes should be saved to db")
	daoCtx.Stop()

	// 2. Read them back in a parallel connection
	dbReaderCtx := context.NewDbContext()
	err = dbReaderCtx.Connect(fmt.Sprintf("test-%d.db.sqlite", time.Now().Unix()))
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx2 := dbCtx.Start()
	dao30s2 := NewEpisodeDao30s(daoCtx2)

	list, err := dao30s2.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	daoCtx2.Stop()
}

func Test_Downtime30s_Transaction_FileWrite(t *testing.T) {
	g := NewWithT(t)
	var err error

	dbFile := fmt.Sprintf("test-%d-tx.db.sqlite", time.Now().Unix())

	dbCtx := context.NewDbContext()
	err = dbCtx.Connect(dbFile)
	g.Expect(err).ShouldNot(HaveOccurred())

	err = migrations.MigrateDatabase(dbCtx, "../migrations/server")
	g.Expect(err).ShouldNot(HaveOccurred())

	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx := dbCtx.Start()
	txCtx, err := daoCtx.BeginTransaction()
	g.Expect(err).ShouldNot(HaveOccurred())

	dao30s := NewEpisodeDao30s(txCtx)

	episodes := []check.Episode{
		{
			ProbeRef: check.ProbeRef{
				Group: "nginx",
				Probe: "main",
			},
			TimeSlot: t360,
			Up:       0,
		},
		{
			ProbeRef: check.ProbeRef{
				Group: "nginx",
				Probe: "redirect",
			},
			TimeSlot: t360,
			Up:       22,
		},
		{
			ProbeRef: check.ProbeRef{
				Group: "api",
				Probe: "configmap",
			},
			TimeSlot: t360,
			Up:       12,
		},
	}

	// 1. Seed some records
	err = dao30s.SaveBatch(episodes)
	if err != nil {
		rollErr := txCtx.Rollback()
		g.Expect(rollErr).ShouldNot(HaveOccurred(), "rollback should not fail after save error %v", err)
	}
	g.Expect(err).ShouldNot(HaveOccurred(), "episodes should be saved to db")
	err = txCtx.Commit()
	g.Expect(err).ShouldNot(HaveOccurred(), "commit should not fail")

	// dao instance with txCtx should not be used after rollback or commit:
	// "sql: transaction has already been committed or rolled back"

	// 2. Read them back via the same connection.
	dao30s = NewEpisodeDao30s(daoCtx)
	list, err := dao30s.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	// 3. Read them back via a new connection pool.
	dbReaderCtx := context.NewDbContext()
	err = dbReaderCtx.Connect(fmt.Sprintf("test-%d.db.sqlite", time.Now().Unix()))
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx2 := dbCtx.Start()
	dao30s2 := NewEpisodeDao30s(daoCtx2)

	list, err = dao30s2.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	daoCtx2.Stop()
}
