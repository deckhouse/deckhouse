/*
Copyright 2023 Flant JSC

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

package dao

import (
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/migrations"
)

// getFileDatabase bootstraps the server database in memory; it is used to generate random data for further usage
func getFileDatabase(t *testing.T, path string) *dbcontext.DbContext {
	return migrations.GetTestFileDatabase(t, path, "../migrations/server")
}

// getTestDatabase bootstraps the server database in memory
func getTestDatabase(t *testing.T) *dbcontext.DbContext {
	return migrations.GetTestMemoryDatabase(t, "../migrations/server")
}

var t360 = time.Unix(360, 0)

func Test_episodes30s_CRUD(t *testing.T) {
	g := NewWithT(t)
	var err error

	dbCtx := getTestDatabase(t)

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
	for _, episode := range list {
		episode.Episode.Up = 100
		err := dao30s.Update(episode.Rowid, episode.Episode)
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

	// 6. Delete everything earlier than the slot
	err = dao30s.DeleteUpTo(t360.Add(-30 * time.Second))
	g.Expect(err).ShouldNot(HaveOccurred(), "should delete earlier records")

	// 7. Check that nothing is deleted
	list, err = dao30s.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list episodes after deletion")
	g.Expect(list).Should(HaveLen(len(episodes)), "should return same quantity")

	// 8. Delete everything at the slot and earlier
	err = dao30s.DeleteUpTo(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should delete all records")

	// 9. Check that episodes are deleted
	list, err = dao30s.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list episodes after deletion")
	g.Expect(len(list)).Should(Equal(0), "should return empty list")
}

func Test_episodes30s_FileWrite(t *testing.T) {
	g := NewWithT(t)
	var err error

	dbCtx := getTestDatabase(t)

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

	daoCtx2 := dbCtx.Start()
	dao30s2 := NewEpisodeDao30s(daoCtx2)

	list, err := dao30s2.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	daoCtx2.Stop()
}

func Test_episodes30s_Transaction_FileWrite(t *testing.T) {
	g := NewWithT(t)
	var err error

	dbCtx := getTestDatabase(t)

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

	daoCtx2 := dbCtx.Start()
	dao30s2 := NewEpisodeDao30s(daoCtx2)

	list, err = dao30s2.ListBySlot(t360)
	g.Expect(err).ShouldNot(HaveOccurred(), "should list saved episodes")
	g.Expect(len(list)).Should(Equal(len(episodes)), "should return same quantity")

	daoCtx2.Stop()
}
