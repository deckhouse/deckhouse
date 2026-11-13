// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ondemand

import (
	"context"
	"errors"
	"time"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/metrics"
)

// Stage of a version render, shown on the placeholder page.
type Stage string

const (
	StageQueued   Stage = "queued"
	StageDownload Stage = "download"
	StageRender   Stage = "render"
	StageDone     Stage = "done"
	StageFailed   Stage = "failed"
)

// Reasons of a failed render, shown to the reader instead of the error: the
// error names registries, repositories and cluster objects, and goes to the
// log only.
const (
	ReasonTimeout  = "timeout"
	ReasonRegistry = "registry"
	ReasonBuild    = "build"
	ReasonBusy     = "busy"
)

func reason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return ReasonTimeout
	// The image is fetched, its files are the reason.
	case kindOf(err) == unrenderable:
		return ReasonBuild
	case errors.Is(err, ErrFetch):
		return ReasonRegistry
	default:
		return ReasonBuild
	}
}

// RenderFunc renders a module version into outDir/<lang>/, using workDir for
// temporary files, and reports its progress through stage. withAI adds the AI
// export (Markdown, llms.txt) to the pages. A version that does not exist is
// an error wrapping ErrVersionNotFound.
type RenderFunc func(ctx context.Context, module, version, workDir, outDir string, withAI bool, stage func(Stage)) error

// ObserveStage reports a stage of a render that succeeded, with ms; image is
// the kind of image a download came from, "cold" for the cold cache.
func ObserveStage(ms *metricsstorage.MetricStorage, stage, image string, start time.Time) {
	if ms == nil {
		return
	}
	ms.HistogramObserve(metrics.DocsBuilderOnDemandStageSeconds, time.Since(start).Seconds(), map[string]string{"stage": stage, "image": image}, nil)
}
