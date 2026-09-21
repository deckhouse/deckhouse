//go:build validation
// +build validation

/*
Copyright 2026 Flant JSC

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

package rbacv2

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// platformAggregationLabelRe matches the label the platform roles aggregate by. Whatever object
// carries it is poured into the role of that lineage by the aggregation controller, which reads
// nothing else -- not the kind label, not the name. That is why the label may only appear on the
// objects the contract test in this package checks.
var platformAggregationLabelRe = regexp.MustCompile(`rbac\.deckhouse\.io/aggregate-to-[a-z0-9-]+-as`)

// Directories whose objects the RBACv2 contract covers: the role model itself and the one-release
// compatibility aliases, which deliberately live beside it so the strict contract test skips them.
var aggregationLabelAllowedDirs = []string{
	string(filepath.Separator) + "templates" + string(filepath.Separator) + "rbacv2" + string(filepath.Separator),
	string(filepath.Separator) + "templates" + string(filepath.Separator) + "rbacv2-compat" + string(filepath.Separator),
}

// TestNoPlatformAggregationLabelOutsideRBACv2 keeps the platform aggregation label out of every
// template that is not part of the role model. A ClusterRole in a module's rbac-for-us.yaml with
// rbac.deckhouse.io/aggregate-to-namespace-as: admin would be merged into d8:namespace:admin for
// every holder in the cluster, and the contract test would never see it: it walks templates/rbacv2
// only. The admission webhook refuses such an object from a user; this test refuses it from a module.
func TestRBACv2AggregationLabelPlacementValidation(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	var offenders []string
	for _, dir := range []string{"modules", "ee"} {
		base := filepath.Join(root, dir)
		if _, statErr := os.Stat(base); os.IsNotExist(statErr) {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				// Module sources, test fixtures and vendored charts are not templates the platform renders.
				switch d.Name() {
				case "images", "testdata", "docs":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".tpl") {
				return nil
			}
			if !strings.Contains(path, string(filepath.Separator)+"templates"+string(filepath.Separator)) {
				return nil
			}
			for _, allowed := range aggregationLabelAllowedDirs {
				if strings.Contains(path, allowed) {
					return nil
				}
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if platformAggregationLabelRe.Match(data) {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("platform aggregation label rbac.deckhouse.io/aggregate-to-<lineage>-as found outside templates/rbacv2 (%d):\n  %s\n"+
			"An object carrying it is merged into the platform role of that lineage. Move it under templates/rbacv2 so the contract test covers it, or drop the label.",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}
