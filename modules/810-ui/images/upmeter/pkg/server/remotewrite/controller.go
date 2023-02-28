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
	"context"
	"fmt"
	"time"

	kube "github.com/flant/kube-client/client"
	log "github.com/sirupsen/logrus"

	"d8.io/upmeter/pkg/check"
	dbcontext "d8.io/upmeter/pkg/db/context"
	"d8.io/upmeter/pkg/monitor/remotewrite"
)

type Exporter interface {
	Export(origin string, episodes []*check.Episode, slotSize time.Duration) error
}

// ControllerConfig configures and creates a Controller
type ControllerConfig struct {
	// collect/export period should be less than episodes update period to catch up with data after downtimes
	Period time.Duration

	// monitoring config objects in kubernetes
	Kubernetes kube.Client

	// read metrics and track exporter state in the DB
	DbCtx        *dbcontext.DbContext
	OriginsCount int
	UserAgent    string

	Logger *log.Logger
}

func (cc *ControllerConfig) Controller() *Controller {
	var (
		kubeMonLogger    = cc.Logger.WithField("who", "kubeMonitor")
		syncLogger       = cc.Logger.WithField("who", "syncers")
		controllerLogger = cc.Logger.WithField("who", "controller")
	)

	kubeMonitor := remotewrite.NewMonitor(cc.Kubernetes, kubeMonLogger)
	storage := newStorage(cc.DbCtx, cc.OriginsCount)
	syncers := newSyncers(storage, cc.Period, syncLogger)

	controller := &Controller{
		kubeMonitor: kubeMonitor,
		syncers:     syncers,
		logger:      controllerLogger,
		userAgent:   cc.UserAgent,
	}

	return controller
}

// Controller links metrics syncers with configs from CR monitor
type Controller struct {
	kubeMonitor *remotewrite.Monitor
	userAgent   string
	syncers     *syncers
	logger      *log.Entry
}

func (c *Controller) Start(ctx context.Context) error {
	headers := map[string]string{"User-Agent": c.userAgent}

	// Monitor tracks the exporter configuration in kubernetes. It is important to subscribe (add event callback)
	// before monitor starts because informers are created during monitor.Start(ctx) call.
	c.logger.Debugln("subscribing to k8s events")
	c.kubeMonitor.Subscribe(&updateHandler{
		syncers: c.syncers,
		logger:  c.logger.WithField("who", "updateHandler"),
		headers: headers,
	})

	c.logger.Debugln("starting k8s monitor")
	err := c.kubeMonitor.Start(ctx)
	if err != nil {
		return fmt.Errorf("cannot start monitor: %v", err)
	}

	// ID syncers runs and stops metrics exporters. Here we read configs and add them one by one.
	c.logger.Debugln("getting k8s CRs list")
	rws, err := c.kubeMonitor.List()
	if err != nil {
		return fmt.Errorf("cannot get initial list of upmeterremotewrite objects: %v", err)
	}

	c.logger.Debugf("found %d k8s CRs", len(rws))
	for _, rw := range rws {
		c.logger.Debugf("adding %q syncer", rw.Name)
		err = c.syncers.Add(ctx, newExportConfig(rw, headers))
		if err != nil {
			c.kubeMonitor.Stop()
			c.syncers.stop()
			return fmt.Errorf("cannot add remote_write syncer %q: %v", rw.Name, err)
		}
	}

	c.logger.Debugln("starting syncers")
	err = c.syncers.start(ctx)
	if err != nil {
		return fmt.Errorf("cannot start syncers: %v", err)
	}

	return nil
}

func (c *Controller) Export(origin string, episodes []*check.Episode, slotSize time.Duration) error {
	return c.syncers.AddEpisodes(origin, episodes, slotSize)
}

func (c *Controller) Stop() {
	c.kubeMonitor.Stop()
	c.syncers.stop()
}

// updateHandler implements the interface required to subscribe to object changes in CR monitor
type updateHandler struct {
	syncers *syncers
	logger  *log.Entry
	headers map[string]string
}

func (s *updateHandler) OnAdd(rw *remotewrite.RemoteWrite) {
	err := s.syncers.Add(context.Background(), newExportConfig(rw, s.headers))
	if err != nil {
		s.logger.Errorf("cannot add remote_write exporter %q: %v", rw.Name, err)
	}
	s.logger.Infof("added remote_write exporter %q", rw.Name)
}

func (s *updateHandler) OnModify(rw *remotewrite.RemoteWrite) {
	err := s.syncers.Add(context.Background(), newExportConfig(rw, s.headers))
	if err != nil {
		s.logger.Errorf("cannot update remote_write exporter %q: %v", rw.Name, err)
	}
	s.logger.Infof("updated remote_write exporter %q", rw.Name)
}

func (s *updateHandler) OnDelete(rw *remotewrite.RemoteWrite) {
	config := newExportConfig(rw, s.headers)
	s.syncers.Delete(config) // TODO: ctx? final exporter requests can take some time
	s.logger.Infof("deleted remote_write exporter %q", rw.Name)
}
