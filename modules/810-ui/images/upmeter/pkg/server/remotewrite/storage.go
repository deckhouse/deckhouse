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

package remotewrite

import (
	"time"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/db/dao"
)

type SyncIdentifier string // not to mess up with other string arguments

type storage struct {
	dao          *dao.ExportDAO
	originsCount int
}

func newStorage(ctx *dbcontext.DbContext, originsCount int) *storage {
	return &storage{
		dao:          dao.NewExportEpisodesDAO(ctx),
		originsCount: originsCount,
	}
}

func (s *storage) Add(syncID SyncIdentifier, origin string, episodes []*check.Episode) error {
	var entities []dao.ExportEntity
	for _, ep := range episodes {
		entity := dao.ExportEntity{
			Episode: *ep,
			SyncID:  string(syncID),
		}
		entity.AddOrigin(origin)
		entities = append(entities, entity)
	}

	return s.dao.Save(entities)
}

func (s *storage) Get(syncID SyncIdentifier) ([]*check.Episode, error) {
	entities, err := s.dao.GetEarliestEpisodes(string(syncID), s.originsCount)
	if err != nil {
		return nil, err
	}

	episodes := make([]*check.Episode, 0)
	for i := range entities {
		episodes = append(episodes, &entities[i].Episode)
	}

	return episodes, nil
}

func (s *storage) Delete(syncID SyncIdentifier, slot time.Time) error {
	return s.dao.DeleteUpTo(string(syncID), slot)
}
