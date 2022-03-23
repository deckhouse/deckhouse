/*
Copyright 2021 Flant JSC

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

package scheduler

import (
	"reflect"
	"testing"
	"time"

	"github.com/flant/shell-operator/pkg/metric_storage"

	"d8.io/upmeter/pkg/agent/manager"
	"d8.io/upmeter/pkg/check"
)

func TestScheduler_convert(t *testing.T) {
	type fields struct {
		probeManager *manager.Manager
		metrics      *metric_storage.MetricStorage
		recv         chan check.Result
		series       map[string]*check.StatusSeries
		results      map[string]*check.ProbeResult
		exportPeriod time.Duration
		scrapePeriod time.Duration
		seriesSize   int
		send         chan []check.Episode
		stop         chan struct{}
		done         chan struct{}
	}
	type args struct {
		start time.Time
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []check.Episode
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Scheduler{
				probeManager: tt.fields.probeManager,
				metrics:      tt.fields.metrics,
				recv:         tt.fields.recv,
				series:       tt.fields.series,
				results:      tt.fields.results,
				exportPeriod: tt.fields.exportPeriod,
				scrapePeriod: tt.fields.scrapePeriod,
				seriesSize:   tt.fields.seriesSize,
				send:         tt.fields.send,
				stop:         tt.fields.stop,
				done:         tt.fields.done,
			}
			got, err := e.convert(tt.args.start)
			if (err != nil) != tt.wantErr {
				t.Errorf("Scheduler.convert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Scheduler.convert() = %v, want %v", got, tt.want)
			}
		})
	}
}
