package db

import (
	"database/sql"
	"github.com/google/martian/log"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/gomega"

	"upmeter/pkg/probe/types"
)

func Test_Fill_RandomDB_For_Today(t *testing.T) {
	//t.SkipNow()
	g := NewWithT(t)
	var err error

	dao30s := NewDowntime30sDao()
	dao5m := NewDowntime5mDao()
	err = Connect("random.db.sqlite", func(dbh *sql.DB) {
		dao30s.Dbh = dbh
		dao5m.Dbh = dbh
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(dao30s.Dbh).ShouldNot(BeNil())
	g.Expect(dao5m.Dbh).ShouldNot(BeNil())

	groupProbes := map[string][]string{
		"control-plane": {
			"access",
			"basic",
			"control-plane-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	firstTs := ((time.Now().Unix() - (24 * 60 * 60)) / 300) * 300
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			log.Infof("gen episodes for %/%s", groupName, probeName)

			// 30 sec
			tsCount := 24 * 60 * 2
			for i := 0; i < tsCount; i++ {
				downtime := types.DowntimeEpisode{
					ProbeRef: types.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(30*i),
					FailSeconds:    int64(i % 30),
					SuccessSeconds: int64(30 - i%30),
				}
				dao30s.Save(downtime)
			}

			// 5min
			step5m := 5 * 60
			tsCount = 24 * 60 * 60 / step5m
			for i := 0; i < tsCount; i++ {
				downtime := types.DowntimeEpisode{
					ProbeRef: types.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(step5m*i),
					FailSeconds:    int64(i % step5m),
					SuccessSeconds: int64(step5m - i%step5m),
				}
				dao5m.Save(downtime)
			}

		}
	}

}

func Test_Fill_30s_OneDay(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dao := NewDowntime30sDao()
	err = Connect("oneday30s.db.sqlite", func(dbh *sql.DB) {
		dao.Dbh = dbh
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(dao.Dbh).ShouldNot(BeNil())

	groupProbes := map[string][]string{
		"control-plane": {
			"access",
			"basic",
			"control-plane-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	firstTs := time.Now().Unix() - (24 * 60 * 60)
	tsCount := 24 * 60 * 2
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			for i := 0; i < tsCount; i++ {
				downtime := types.DowntimeEpisode{
					ProbeRef: types.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(30*i),
					FailSeconds:    int64(i % 30),
					SuccessSeconds: int64(30 - i%30),
				}
				dao.Save(downtime)
			}
		}
	}

}

func Test_FillOneDay(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dao := NewDowntime5mDao()
	err = Connect("oneday.db.sqlite", func(dbh *sql.DB) {
		dao.Dbh = dbh
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(dao.Dbh).ShouldNot(BeNil())

	groupProbes := map[string][]string{
		"control-plane": {
			"access",
			"basic",
			"control-plane-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	firstTs := time.Now().Unix() - (24 * 60 * 60)
	tsCount := 24 * (60 / 5)
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			for i := 0; i < tsCount; i++ {
				downtime := types.DowntimeEpisode{
					ProbeRef: types.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(300*i),
					FailSeconds:    int64(i % 300),
					SuccessSeconds: int64(300 - i%300),
				}
				dao.Save(downtime)
			}
		}
	}

}

func Test_Fill_Year(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dao := NewDowntime5mDao()
	err = Connect("year.db.sqlite", func(dbh *sql.DB) {
		dao.Dbh = dbh
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(dao.Dbh).ShouldNot(BeNil())

	groupProbes := map[string][]string{
		"control-plane": {
			"access",
			"basic",
			"control-plane-manager",
			"namespace",
			"scheduler",
		},
		"synthetic": {
			"access",
			"dns",
			"neighbor",
			"neighbor-via-service",
		},
	}

	firstTs := time.Now().Unix() - (365 * 24 * 60 * 60)
	tsCount := 365 * 24 * (60 / 5)
	for groupName, probeNames := range groupProbes {
		for _, probeName := range probeNames {
			for i := 0; i < tsCount; i++ {
				downtime := types.DowntimeEpisode{
					ProbeRef: types.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(300*i),
					FailSeconds:    int64(i % 300),
					SuccessSeconds: int64(300 - i%300),
				}
				dao.Save(downtime)
			}
		}
	}

}
