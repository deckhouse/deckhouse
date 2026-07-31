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

package helm

import (
	"fmt"
	"sort"
	"strings"

	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

// ErrParameterInjection is returned when a project parameter does not stay a value in the rendered
// manifests but becomes part of their structure.
var ErrParameterInjection = fmt.Errorf("a project parameter changes the structure of the rendered manifests")

// lineBreaks are the characters that end a YAML scalar. A parameter carrying one of them can close
// the value it was substituted into and have the rest of itself read as manifest structure -- a new
// key, or, after a document separator, a whole new object. The rendered manifests are applied by a
// ServiceAccount bound to cluster-admin, so such an object is applied with cluster-admin.
var lineBreaks = []string{"\n", "\r", "\u0085", "\u2028", "\u2029"}

// ensureParametersStayValues refuses parameters that render into structure instead of into values.
//
// Quoting every substitution in the shipped templates fixes those templates. It cannot fix the ones
// a cluster administrator writes: a ProjectTemplate is free-form text, and its author is not obliged
// to know that an unquoted substitution ends at the first line break. This check is what covers them,
// and it needs no knowledge of the template at all.
//
// The method: render the template twice, once with the parameters as given and once with every line
// break in them replaced by a space, then compare the two outputs as sets of objects, with the same
// replacement applied to every string inside. For an input that carries no line breaks the two
// renders are identical by construction, so there is nothing to report; for an input whose line
// breaks only ever land inside a value -- a quoted substitution, a block scalar -- the outputs differ
// in that value alone, and the replacement makes them equal again. What is left is the case where the
// value produced structure, and that is exactly what is refused.
func (c *Client) ensureParametersStayValues(project *v1alpha3.Project, template *v1alpha1.ProjectTemplate) error {
	if !carriesLineBreak(project.Spec.Parameters) {
		return nil
	}

	sanitized := *project
	sanitized.Spec.Parameters, _ = replaceLineBreaks(project.Spec.Parameters).(map[string]any)

	actual, err := c.renderTemplate(project, template)
	if err != nil {
		return fmt.Errorf("render the project: %w", err)
	}

	expected, err := c.renderTemplate(&sanitized, template)
	if err != nil {
		// The parameters render only because of their line breaks, which is the injection this
		// check exists for -- a template that needs a line break to produce valid YAML would be
		// broken for every ordinary input.
		return fmt.Errorf("%w: %w", ErrParameterInjection, err)
	}

	actualObjects, err := objectDigests(actual)
	if err != nil {
		return fmt.Errorf("read the rendered manifests: %w", err)
	}

	expectedObjects, err := objectDigests(expected)
	if err != nil {
		// The manifests parse only while the line breaks are there. A substitution that needs them
		// to produce valid YAML is an unquoted one, which is the whole problem.
		return fmt.Errorf("%w: %w", ErrParameterInjection, err)
	}

	if extra := extraObjects(actualObjects, expectedObjects); len(extra) > 0 {
		return fmt.Errorf("%w: %s", ErrParameterInjection, strings.Join(extra, ", "))
	}

	return nil
}

// extraObjects describes every object the first render produces that the second one does not.
func extraObjects(actual, expected map[string]string) []string {
	var extra []string
	for digest, description := range actual {
		if _, ok := expected[digest]; !ok {
			extra = append(extra, description)
		}
	}
	sort.Strings(extra)

	return extra
}

// objectDigests maps every rendered object to a canonical form of itself, keyed so that duplicates
// of one object collapse the same way in both renders.
func objectDigests(manifests string) (map[string]string, error) {
	digests := make(map[string]string)

	for _, raw := range releaseutil.SplitManifests(manifests) {
		object := new(unstructured.Unstructured)
		if err := yaml.Unmarshal([]byte(raw), object); err != nil {
			return nil, err
		}
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			continue
		}

		stripped, _ := replaceLineBreaks(object.Object).(map[string]any)
		canonical, err := yaml.Marshal(stripped)
		if err != nil {
			return nil, err
		}

		digests[string(canonical)] = fmt.Sprintf("%s %s/%s", object.GetKind(), object.GetAPIVersion(), object.GetName())
	}

	return digests, nil
}

// carriesLineBreak reports whether any string anywhere in the parameters contains a line break.
func carriesLineBreak(value any) bool {
	switch typed := value.(type) {
	case string:
		for _, br := range lineBreaks {
			if strings.Contains(typed, br) {
				return true
			}
		}
	case map[string]any:
		for key, nested := range typed {
			if carriesLineBreak(key) || carriesLineBreak(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if carriesLineBreak(nested) {
				return true
			}
		}
	}

	return false
}

// replaceLineBreaks rebuilds the value with every line break in every string replaced by a space.
func replaceLineBreaks(value any) any {
	switch typed := value.(type) {
	case string:
		for _, br := range lineBreaks {
			typed = strings.ReplaceAll(typed, br, " ")
		}

		return typed
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, nested := range typed {
			strippedKey, _ := replaceLineBreaks(key).(string)
			result[strippedKey] = replaceLineBreaks(nested)
		}

		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, nested := range typed {
			result = append(result, replaceLineBreaks(nested))
		}

		return result
	}

	return value
}
