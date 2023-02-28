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
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/prometheus/prompb"

	"d8.io/upmeter/pkg/check"
)

func convEpisodes2Timeseries(timeslot time.Time, episodes []*check.Episode, commonLabels []*prompb.Label) []*prompb.TimeSeries {
	tss := make([]*prompb.TimeSeries, 0)

	for _, ep := range episodes {
		var labels []*prompb.Label
		labels = append(labels, episodeLabels(ep)...)
		labels = append(labels, commonLabels...)

		nodata := ep.NoData
		fail := ep.Down
		unknown := ep.Unknown
		success := ep.Up

		tss = append(tss,
			statusTimeseries(timeslot, success, withLabel(labels, &prompb.Label{Name: "status", Value: "up"})),
			statusTimeseries(timeslot, fail, withLabel(labels, &prompb.Label{Name: "status", Value: "down"})),
			statusTimeseries(timeslot, unknown, withLabel(labels, &prompb.Label{Name: "status", Value: "unknown"})),
			statusTimeseries(timeslot, nodata, withLabel(labels, &prompb.Label{Name: "status", Value: "nodata"})),
		)
	}
	return tss
}

func statusTimeseries(timeslot time.Time, value time.Duration, labels []*prompb.Label) *prompb.TimeSeries {
	return &prompb.TimeSeries{
		Labels: labels,
		Samples: []prompb.Sample{
			{
				Timestamp: timeslot.Unix() * 1e3, // milliseconds
				Value:     float64(value.Milliseconds()),
			},
		},
	}
}

func withLabel(originalLabels []*prompb.Label, statusLabel *prompb.Label) []*prompb.Label {
	labels := make([]*prompb.Label, len(originalLabels), len(originalLabels)+1)
	copy(labels, originalLabels)
	labels = append(labels, statusLabel)

	return labels
}

func episodeLabels(ep *check.Episode) []*prompb.Label {
	return []*prompb.Label{
		{
			Name:  "__name__",
			Value: "statustime",
		},
		{
			Name:  "probe_ref",
			Value: ep.ProbeRef.Id(),
		},
		{
			Name:  "probe",
			Value: ep.ProbeRef.Probe,
		},
		{
			Name:  "group",
			Value: ep.ProbeRef.Group,
		},
	}
}

func stringifyTimeseries(tss []*prompb.TimeSeries, name string) string {
	b := strings.Builder{}
	for _, ts := range tss {
		b.WriteString("\n" + name + "   ")
		b.WriteString(stringifyLabels(ts.Labels))
		for _, s := range ts.Samples {
			stamp := time.Unix(s.Timestamp/1000, 0).Format("15:04:05")
			b.WriteString(fmt.Sprintf("    %s  %0.f", stamp, s.Value))
		}
	}
	return b.String()
}

func stringifyLabels(labels []*prompb.Label) string {
	var ref, status, name string
	for _, lbl := range labels {
		if lbl.Name == "probe_ref" {
			ref = lbl.Value
			continue
		}
		if lbl.Name == "status" {
			status = lbl.Value
		}

		if lbl.Name == "__name__" {
			name = lbl.Value
		}
	}
	return fmt.Sprintf("__name__=%s ref=%s status=%s", name, ref, status)
}
