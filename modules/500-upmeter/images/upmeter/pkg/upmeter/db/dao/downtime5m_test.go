package dao

import (
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"upmeter/pkg/checks"
	"upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/migrations"
)

func Test_Fill_RandomDB_For_Today(t *testing.T) {
	// Uncomment to generate test data for webui.
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dbCtx := context.NewDbContext()
	err = dbCtx.Connect("random.db.sqlite")
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	dao30s := NewDowntime30sDao(daoCtx)
	dao5m := NewDowntime5mDao(daoCtx)

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
			log.Infof("gen episodes for %s/%s", groupName, probeName)

			// 30 sec
			tsCount := 24 * 60 * 2
			for i := 0; i < tsCount; i++ {
				downtime := checks.DowntimeEpisode{
					ProbeRef: checks.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(30*i),
					FailSeconds:    int64(i%30 - i%7 - i%3),
					SuccessSeconds: int64(30 - i%30 - i%7 - i%3),
					Unknown:        int64(i % 7),
					NoData:         int64(i % 3),
				}
				dao30s.Insert(downtime)
			}

			// 5min
			step5m := 5 * 60
			tsCount = 24 * 60 * 60 / step5m
			for i := 0; i < tsCount; i++ {
				downtime := checks.DowntimeEpisode{
					ProbeRef: checks.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(step5m*i),
					FailSeconds:    int64(i % step5m),
					SuccessSeconds: int64(step5m - i%step5m),
				}
				dao5m.Insert(downtime)
			}

		}
	}

}

// Test_Fill_30s_OneDay fills a database with random data to measure a size for 1 day.
func Test_Fill_30s_OneDay(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dbCtx := context.NewDbContext()
	err = dbCtx.Connect("oneday30s.db.sqlite")
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	migrator := migrations.NewMigratorService()
	migrator.Apply(dbCtx)

	dao30s := NewDowntime30sDao(daoCtx)

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
				downtime := checks.DowntimeEpisode{
					ProbeRef: checks.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(30*i),
					FailSeconds:    int64(i%30 - i%4 - i%5),
					SuccessSeconds: int64(30 - i%30 - i%4 - i%5),
					Unknown:        int64(i % 4),
					NoData:         int64(i % 5),
				}
				err = dao30s.Insert(downtime)
				g.Expect(err).ShouldNot(HaveOccurred())
			}
		}
	}
}

func Test_FillOneDay(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dbCtx := context.NewDbContext()
	err = dbCtx.Connect("oneday5m.db.sqlite")
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	migrator := migrations.NewMigratorService()
	migrator.Apply(dbCtx)

	dao5m := NewDowntime5mDao(daoCtx)

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
				downtime := checks.DowntimeEpisode{
					ProbeRef: checks.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(300*i),
					FailSeconds:    int64(i%300 - i%37 - i%13),
					SuccessSeconds: int64(300 - i%300 - i%37 - i%13),
					Unknown:        int64(i % 37),
					NoData:         int64(i % 13),
				}
				dao5m.Insert(downtime)
			}
		}
	}

}

func Test_Fill_Year(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)
	var err error

	dbCtx := context.NewDbContext()
	err = dbCtx.Connect("year.db.sqlite")
	g.Expect(err).ShouldNot(HaveOccurred())

	daoCtx := dbCtx.Start()
	defer daoCtx.Stop()

	migrator := migrations.NewMigratorService()
	migrator.Apply(dbCtx)

	dao5m := NewDowntime5mDao(daoCtx)

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
				downtime := checks.DowntimeEpisode{
					ProbeRef: checks.ProbeRef{
						Group: groupName,
						Probe: probeName,
					},
					TimeSlot:       firstTs + int64(300*i),
					FailSeconds:    int64(i%300 - i%31 - i%17),
					SuccessSeconds: int64(300 - i%300 - i%31 - i%17),
					Unknown:        int64(i % 17),
					NoData:         int64(i % 31),
				}
				dao5m.Insert(downtime)
			}
		}
	}

}
