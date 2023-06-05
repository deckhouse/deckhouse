/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/go-openapi/spec"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
)

const (
	Namespace         = "d8-multitenancy-manager"
	APIVersion        = "deckhouse.io/v1alpha1"
	ProjectTypeKind   = "ProjectType"
	ProjectKind       = "Project"
	PTValuesPath      = "projectTypes"
	ProjectValuesPath = "projects"
)

func ModuleQueue(q string) string {
	return path.Join("/modules/multitenancy-manager", q)
}

func ModuleValuePath(svp ...string) string {
	resultPath := []string{"multitenancyManager", "internal"}
	for _, p := range svp {
		resultPath = append(resultPath, strings.Trim(p, "."))
	}
	return strings.Join(resultPath, ".")
}

func LoadOpenAPISchema(s interface{}) (*spec.Schema, error) {
	properties := map[string]interface{}{
		"properties": s,
	}
	d, err := json.Marshal(properties)
	if err != nil {
		return nil, fmt.Errorf("json marshal spec.openAPI: %w", err)
	}

	schema := new(spec.Schema)
	if err := json.Unmarshal(d, schema); err != nil {
		return nil, fmt.Errorf("unmarshal spec.openAPI to spec.Schema: %w", err)
	}

	err = spec.ExpandSchema(schema, schema, nil)
	if err != nil {
		return nil, fmt.Errorf("expand the schema in spec.openAPI: %w", err)
	}

	return schema, nil
}

func SetProjectTypeStatus(patcher *object_patch.PatchCollector, ptName string, status bool, message string) {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"statusSummary": map[string]interface{}{
				"status":  status,
				"message": stringOrNil(message),
			},
		},
	}

	patchStatus(patcher, ProjectTypeKind, ptName, statusPatch)
}

func SetProjectStatus(patcher *object_patch.PatchCollector, projectName string, status bool, message string, conditions []v1alpha1.Condition) {
	uniqueConds := uniqueConditions(conditions)
	newConditions := make([]map[string]interface{}, 0, len(uniqueConds))
	for _, cond := range uniqueConds {
		newCond := map[string]interface{}{
			"name":    cond.Name,
			"status":  cond.Status,
			"message": stringOrNil(cond.Message),
		}

		newConditions = append(newConditions, newCond)
	}

	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"statusSummary": map[string]interface{}{
				"status":  status,
				"message": stringOrNil(message),
			},
			"conditions": newConditions,
		},
	}

	patchStatus(patcher, ProjectKind, projectName, statusPatch)
}

func uniqueConditions(conds []v1alpha1.Condition) []v1alpha1.Condition {
	uniqueConds := make(map[v1alpha1.Condition]bool)
	for _, c := range conds {
		if uniqueConds[c] {
			continue
		}
		uniqueConds[c] = true
	}

	result := make([]v1alpha1.Condition, 0, len(uniqueConds))
	for c := range uniqueConds {
		result = append(result, c)
	}
	return result
}

func patchStatus(patcher *object_patch.PatchCollector, kind, objectName string, patch interface{}) {
	patcher.MergePatch(patch, APIVersion, kind, "", objectName, object_patch.WithSubresource("/status"))
}

func stringOrNil(s string) interface{} {
	if s != "" {
		return s
	}
	return nil
}
