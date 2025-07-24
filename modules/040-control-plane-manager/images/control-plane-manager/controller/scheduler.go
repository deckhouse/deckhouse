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

package main

import (
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var period = 1 * time.Minute

type Scheduler struct {
	ticker     *time.Ticker
	exit       <-chan struct{}
	promoteMtx *sync.Mutex
}

func NewScheduler(exitChan <-chan struct{}) *Scheduler {
	return &Scheduler{
		exit:       exitChan,
		promoteMtx: &sync.Mutex{},
	}
}

func (s *Scheduler) Run() {
	log.Info("starting periodic scheduler")
	s.ticker = time.NewTicker(period)

	for {
		select {
		case <-s.ticker.C:
			go s.runEtcdPromoteTask()
		case <-s.exit:
			s.ticker.Stop()
			log.Info("Stopping periodic scheduler")
			return
		}
	}
}

func (s *Scheduler) runEtcdPromoteTask() {
	s.promoteMtx.Lock()
	defer s.promoteMtx.Unlock()

	log.Info("[d8][etcd] periodic learner check")
	if err := etcdPromote(); err != nil {
		log.Errorf("[d8][etcd] periodic learner check failed: %v", err)
	} else {
		log.Info("[d8][etcd] periodic learner check successfully")
	}
}

func etcdPromote() error {
	etcd, err := NewEtcd()
	if err != nil {
		return err
	}
	defer etcd.client.Close()

	err = etcd.PromoteLearnersIfNeeded()
	if err != nil {
		return err
	}
	return nil
}
