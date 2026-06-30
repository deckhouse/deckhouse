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

package config

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

// validateCNIBootstrap is the Layer-3 (domain) validator for CNI: filter the
// payload to cni-relevant docs, extract cluster intent without re-running
// OpenAPI validation (schema validation is Layer 2's job), then surface the
// CNI mismatch — if any — against the provider's cni-bootstrap.yml.
func validateCNIBootstrap(ctx context.Context, payload string, _ *SchemaStore, _ ...ValidateOption) *ValidationError {
	filtered := filterCNIRelevantDocs(payload)
	if filtered == "" {
		return nil
	}
	meta, err := ParseClusterPayload(ctx, filtered)
	if err != nil {
		// Filtered payload that does not parse structurally — nothing to
		// analyze. Layer 1/2 already saw the original payload.
		return nil
	}
	analysis, err := AnalyzeCNIBootstrap(ctx, meta, nil)
	if err != nil {
		// Installer-side error (missing/broken cni-bootstrap.yml, missing
		// cni-* schema, recommended MC failing OpenAPI). Log and skip — not
		// a user-input failure.
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("cni-bootstrap analysis skipped: %v", err))
		return nil
	}
	if analysis.SkipReason != "" || analysis.Matches {
		return nil
	}
	// Point the error at the offending user MC: it's the cni-* ModuleConfig the
	// user wrote (or, on DifferentModule, the one they should remove). Falls back
	// to the recommended MC's name when the user supplied none (shouldn't happen
	// here — Matches=true in that case — but keep the wire-format sane).
	e := Error{
		Group:    ModuleConfigGroup,
		Version:  ModuleConfigVersion,
		Kind:     ModuleConfigKind,
		Messages: []string{analysis.ReasonMessage},
	}
	if mc := analysis.ModuleConfig; mc != nil {
		switch {
		case mc.UserInput != nil:
			e.Name = mc.UserInput.GetName()
		case mc.Recommended != nil:
			e.Name = mc.Recommended.GetName()
		}
	}
	out := &ValidationError{}
	out.Append(cniMismatchReasonToErrorKind(analysis.MismatchReason), e)
	return out
}

// filterCNIRelevantDocs returns a multi-doc YAML containing only the documents
// the CNI validator needs: ClusterConfiguration, the provider's
// *ClusterConfiguration, and any cni-* ModuleConfig. Documents that fail to
// even index-parse are skipped silently — their structural problems belong to
// other validators. Returns "" when no ClusterConfiguration is present.
func filterCNIRelevantDocs(payload string) string {
	var hasCluster bool
	var keep []string
	for _, doc := range input.YAMLSplitRegexp.Split(strings.TrimSpace(payload), -1) {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var index namedIndex
		if err := yaml.Unmarshal([]byte(doc), &index); err != nil {
			continue
		}
		switch {
		case index.Kind == ClusterConfigurationKind:
			hasCluster = true
			keep = append(keep, doc)
		case index.Kind == ModuleConfigKind && strings.HasPrefix(index.Metadata.Name, "cni-"):
			keep = append(keep, doc)
		case strings.HasSuffix(index.Kind, "ClusterConfiguration") &&
			index.Kind != StaticClusterConfigurationKind &&
			index.Kind != InitConfigurationKind:
			keep = append(keep, doc)
		}
	}
	if !hasCluster {
		return ""
	}
	return strings.Join(keep, "\n---\n")
}

func cniMismatchReasonToErrorKind(r CNIBootstrapMismatchReason) ErrorKind {
	switch r {
	case CNIBootstrapMismatchReasonDifferentModule:
		return ErrKindCNIMismatch
	case CNIBootstrapMismatchReasonDifferentSettings:
		return ErrKindCNISettingsMismatch
	default:
		return ErrKindValidationFailed
	}
}
