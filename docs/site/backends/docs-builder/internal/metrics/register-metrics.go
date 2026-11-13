// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package metrics

import (
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

const (
	// Rendering a module version on demand.
	DocsBuilderOnDemandRenderTotal           = "docs_builder_ondemand_render_total"
	DocsBuilderOnDemandRenderDurationSeconds = "docs_builder_ondemand_render_duration_seconds"
	DocsBuilderOnDemandCacheVersions         = "docs_builder_ondemand_cache_versions"
	DocsBuilderOnDemandCacheBytes            = "docs_builder_ondemand_cache_bytes"
	DocsBuilderOnDemandCachePinnedBytes      = "docs_builder_ondemand_cache_pinned_bytes"
	DocsBuilderOnDemandCachePinnedBudget     = "docs_builder_ondemand_cache_pinned_budget_bytes"
	DocsBuilderOnDemandCacheReadersBytes     = "docs_builder_ondemand_cache_readers_bytes"
	DocsBuilderOnDemandCacheReadersBudget    = "docs_builder_ondemand_cache_readers_budget_bytes"
	DocsBuilderOnDemandCacheVolumeBytes      = "docs_builder_ondemand_cache_volume_bytes"
	DocsBuilderOnDemandCacheKindVersions     = "docs_builder_ondemand_cache_kind_versions"
	DocsBuilderOnDemandS3UploadFailures      = "docs_builder_ondemand_s3_upload_failures_total"
	DocsBuilderOnDemandVersionsListSeconds   = "docs_builder_ondemand_versions_list_duration_seconds"
	DocsBuilderOnDemandS3Ready               = "docs_builder_ondemand_s3_ready"
	DocsBuilderOnDemandStageSeconds          = "docs_builder_ondemand_stage_duration_seconds"
	DocsBuilderOnDemandRegistryRequests      = "docs_builder_ondemand_registry_requests_total"
	// The replicas sharing S3: the leader reads the registry, the others
	// follow it.
	DocsBuilderOnDemandLeader            = "docs_builder_ondemand_leader"
	DocsBuilderOnDemandFollowFallback    = "docs_builder_ondemand_follow_fallback"
	DocsBuilderOnDemandFollowMissing     = "docs_builder_ondemand_follow_missing_versions"
	DocsBuilderOnDemandPinSuccessSeconds = "docs_builder_ondemand_pin_last_success_timestamp_seconds"
	DocsBuilderOnDemandStaleArchives     = "docs_builder_ondemand_stale_archives"
	DocsBuilderOnDemandRerenderFailed    = "docs_builder_ondemand_rerender_failed_archives"
	DocsBuilderOnDemandModulesListFailed = "docs_builder_ondemand_modules_list_failed"
	DocsBuilderOnDemandFailedBranches    = "docs_builder_ondemand_failed_branches"
)

func RegisterMetrics(mStorage *metricsstorage.MetricStorage) error {
	// On-demand renders: status is done, failed or not_found.
	_, err := mStorage.RegisterCounter(
		DocsBuilderOnDemandRenderTotal,
		[]string{"status"},
		options.WithHelp("Total number of on-demand module version renders (done/failed/not_found)"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterHistogram(
		DocsBuilderOnDemandRenderDurationSeconds,
		[]string{"status"},
		[]float64{1, 2.5, 5, 10, 20, 40, 80},
		options.WithHelp("On-demand module version render duration in seconds, from the request to the published pages"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterGauge(
		DocsBuilderOnDemandCacheVersions,
		nil,
		options.WithHelp("Number of module versions in the on-demand cache"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterGauge(
		DocsBuilderOnDemandCacheBytes,
		nil,
		options.WithHelp("Total size of module versions in the on-demand cache in bytes"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterGauge(
		DocsBuilderOnDemandCachePinnedBytes,
		nil,
		options.WithHelp("Size of the pinned module versions in the on-demand cache in bytes: those of the search and the release channels, never evicted"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterGauge(
		DocsBuilderOnDemandCachePinnedBudget,
		nil,
		options.WithHelp("Room in bytes the pinned module versions have on the cache volume: its size less the room of the versions readers open"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterGauge(
		DocsBuilderOnDemandCacheKindVersions,
		[]string{"kind"},
		options.WithHelp("Number of module versions in the on-demand cache by kind: target (the version of a module the search is built from), channel (another version of one of its release channels), reader (opened by readers, evicted first)"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterCounter(
		DocsBuilderOnDemandS3UploadFailures,
		nil,
		options.WithHelp("Total number of rendered module versions that could not be uploaded to S3 while the bucket is ready"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterHistogram(
		DocsBuilderOnDemandVersionsListSeconds,
		[]string{"status"},
		[]float64{0.1, 0.25, 0.5, 1, 2.5, 5},
		options.WithHelp("Duration of listing module versions in the registry in seconds (ok/fail)"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterHistogram(
		DocsBuilderOnDemandStageSeconds,
		[]string{"stage", "image"},
		[]float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20},
		options.WithHelp("Duration of a stage of a module version render in seconds: download (image is release, module or cold for S3), render and publish"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterCounter(
		DocsBuilderOnDemandRegistryRequests,
		[]string{"request", "code"},
		options.WithHelp("Requests of docs-builder to the module registries by kind (ping, token, manifest_head, manifest, blob, tags, other) and status class"),
	)
	if err != nil {
		return err
	}

	_, err = mStorage.RegisterGauge(
		DocsBuilderOnDemandS3Ready,
		nil,
		options.WithHelp("Whether the S3 storage for rendered module versions (the system one or the bucket of the user) is usable (1) or not (0)"),
	)
	if err != nil {
		return err
	}

	for name, help := range map[string]string{
		DocsBuilderOnDemandCacheReadersBytes:  "Size of the module versions readers opened in the on-demand cache in bytes: those not pinned, evicted when idle or above their budget",
		DocsBuilderOnDemandCacheReadersBudget: "Room in bytes the module versions readers open have in the on-demand cache: the least recently opened ones are evicted above it",
		DocsBuilderOnDemandCacheVolumeBytes:   "Size of the on-demand cache volume in bytes (--ondemand-cache-size); not reported when it is not given",
		DocsBuilderOnDemandLeader:             "Whether this replica holds the Lease and reads the registry for the replicas sharing S3 (1) or not (0)",
		DocsBuilderOnDemandFollowFallback:     "Whether this replica reads the registry itself because it could not read the versions of the leader from S3 (1) or not (0)",
		DocsBuilderOnDemandFollowMissing:      "Module versions the leader recorded in S3, the targets of the search and the versions of their release channels, that this replica found no archive of there when it last took them: its search, llms.txt and channels go without them; 0 on the leader",
		DocsBuilderOnDemandPinSuccessSeconds:  "When the versions of the search and of the release channels were last resolved and rendered, Unix time; until the first such pass of the pod, when it started reading the registry",
		DocsBuilderOnDemandStaleArchives:      "Archives in S3 rendered by an older template and not rendered anew yet",
		DocsBuilderOnDemandRerenderFailed:     "Archives in S3 the current template could not render anew: they are served as rendered by an older one",
		DocsBuilderOnDemandModulesListFailed:  "Whether the last build of the page of the list of modules failed (1), so the previous page is served, or not (0)",
		DocsBuilderOnDemandFailedBranches:     "Development branches of modules (ModulePullOverride) that could not be rendered for a reason that stays until the branch changes: no documentation, a Hugo error",
	} {
		if _, err := mStorage.RegisterGauge(name, nil, options.WithHelp(help)); err != nil {
			return err
		}
	}

	return nil
}
