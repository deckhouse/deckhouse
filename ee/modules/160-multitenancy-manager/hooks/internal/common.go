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
)

const (
	Namespace = "d8-multitenancy-manager"

	ProjectTemplateKind = "ProjectTemplate"
	ProjectTypeKind     = "ProjectType"
	ProjectKind         = "Project"

	ProjectTemplateAPIVersion = "deckhouse.io/v1alpha1"
	ProjectTypeAPIVersion     = "deckhouse.io/v1alpha1"
	ProjectAPIVersion         = "deckhouse.io/v1alpha2"

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

func LoadOpenAPISchema(properties map[string]interface{}) (*spec.Schema, error) {
	d, err := json.Marshal(properties)
	if err != nil {
		if jsonErr, ok := err.(*json.SyntaxError); ok {
			problemPart := d[jsonErr.Offset-10 : jsonErr.Offset+10]
			err = fmt.Errorf("%w ~ error near '%s' (offset %d)", err, problemPart, jsonErr.Offset)
		}
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

func patchStatus(patcher *object_patch.PatchCollector, kind, objectName string, patch interface{}, version string) {
	patcher.MergePatch(patch, version, kind, "", objectName, object_patch.WithSubresource("/status"))
}

func stringOrNil(s string) interface{} {
	if s != "" {
		return s
	}
	return nil
}
