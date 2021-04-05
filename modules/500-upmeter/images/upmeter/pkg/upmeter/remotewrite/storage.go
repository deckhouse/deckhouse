package remotewrite

import (
	"time"

	"upmeter/pkg/check"
	dbcontext "upmeter/pkg/upmeter/db/context"
	"upmeter/pkg/upmeter/db/dao"
)

type SyncIdentifier string // not to mess up with other string arguments

type storage struct {
	dao          *dao.ExportEpisodesDAO
	originsCount int
}

func newStorage(ctx *dbcontext.DbContext, originsCount int) *storage {
	return &storage{
		dao:          dao.NewExportEpisodesDAO(ctx),
		originsCount: originsCount,
	}
}

func (s *storage) Add(syncID SyncIdentifier, origin string, episodes []*check.DowntimeEpisode) error {
	var entities []dao.ExportEpisodeEntity
	for _, ep := range episodes {
		entity := dao.ExportEpisodeEntity{
			Episode: *ep,
			SyncID:  string(syncID),
		}
		entity.AddOrigin(origin)
		entities = append(entities, entity)
	}

	return s.dao.Save(entities)
}

func (s *storage) Get(syncID SyncIdentifier) ([]*check.DowntimeEpisode, error) {
	entities, err := s.dao.GetEarliestEpisodes(string(syncID), s.originsCount)
	if err != nil {
		return nil, err
	}

	episodes := make([]*check.DowntimeEpisode, 0)
	for i := range entities {
		episodes = append(episodes, &entities[i].Episode)
	}

	return episodes, nil
}

func (s *storage) Delete(syncID SyncIdentifier, slot time.Time) error {
	return s.dao.Delete(string(syncID), slot)
}
