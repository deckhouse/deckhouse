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
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

// ErrParameterInjection is returned when a project parameter does not stay a value in the rendered
// manifests but becomes part of their structure.
var ErrParameterInjection = errors.New("a project parameter changes the structure of the rendered manifests")

// lineBreakToSpace replaces the characters that end a YAML scalar. A parameter carrying one of them
// can close the value it was substituted into and have the rest of itself read as manifest structure
// -- a new key, or, after a document separator, a whole new object. The rendered manifests are
// applied by a ServiceAccount bound to cluster-admin, so such an object is applied with cluster-admin.
var lineBreakToSpace = strings.NewReplacer("\n", " ", "\r", " ", "\u0085", " ", "\u2028", " ", "\u2029", " ")

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
	// A project without parameters has nothing that could become structure. Checked separately
	// because the rewrite below turns no parameters into empty ones, which do not compare equal.
	if len(project.Spec.Parameters) == 0 {
		return nil
	}

	parameters := rewriteStringsIn(project.Spec.Parameters, lineBreakToSpace.Replace)
	if reflect.DeepEqual(parameters, project.Spec.Parameters) {
		// Nothing in them can end a scalar, so nothing in them can produce structure and the second
		// render would be the same as the first.
		return nil
	}

	sanitized := project.DeepCopy()
	sanitized.Spec.Parameters = parameters

	actual, err := c.renderTemplate(project, template)
	if err != nil {
		return err
	}

	expected, err := c.renderTemplate(sanitized, template)
	if err != nil {
		// The parameters render only because of their line breaks, which is the injection this
		// check exists for -- a template that needs a line break to produce valid YAML would be
		// broken for every ordinary input.
		return fmt.Errorf("%w: %w", ErrParameterInjection, err)
	}

	actualObjects, err := canonicalObjects(actual)
	if err != nil {
		return fmt.Errorf("read the rendered manifests: %w", err)
	}

	expectedObjects, err := canonicalObjects(expected)
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
	for canonical, description := range actual {
		if _, ok := expected[canonical]; !ok {
			extra = append(extra, description)
		}
	}
	slices.Sort(extra)

	return extra
}

// canonicalObjects maps the canonical form of every rendered object to a description of it. Keying
// by the form itself makes duplicates of one object collapse the same way in both renders.
func canonicalObjects(manifests string) (map[string]string, error) {
	objects := make(map[string]string)

	for _, raw := range releaseutil.SplitManifests(manifests) {
		object := new(unstructured.Unstructured)
		if err := yaml.Unmarshal([]byte(raw), object); err != nil {
			return nil, fmt.Errorf("parse a rendered object: %w", err)
		}
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			continue
		}

		canonical, err := yaml.Marshal(rewriteStringsIn(object.Object, collapseSpace))
		if err != nil {
			return nil, fmt.Errorf("canonicalize the %s %q: %w", object.GetKind(), object.GetName(), err)
		}

		objects[string(canonical)] = fmt.Sprintf("%s %s/%s", object.GetKind(), object.GetAPIVersion(), object.GetName())
	}

	return objects, nil
}

// collapseSpace makes every run of whitespace a single space.
//
// Comparing the two renders needs the strings on both sides to survive a YAML round trip the same
// way, and they do not: a multi-line value comes back from a block scalar without its final line
// break, while the same value with its line breaks already replaced comes back as a plain scalar
// that kept the space in their place. That difference is whitespace and nothing else, whereas
// injection shows up as keys, list items and objects -- structure, which this leaves alone.
func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// rewriteStringsIn walks a decoded YAML map and applies rewrite to every string in it, keys
// included. Anything that is not a string, a map or a list is left as it is.
func rewriteStringsIn(value map[string]any, rewrite func(string) string) map[string]any {
	result := make(map[string]any, len(value))
	for key, nested := range value {
		result[rewrite(key)] = rewriteStrings(nested, rewrite)
	}

	return result
}

func rewriteStrings(value any, rewrite func(string) string) any {
	switch typed := value.(type) {
	case string:
		return rewrite(typed)
	case map[string]any:
		return rewriteStringsIn(typed, rewrite)
	case []any:
		result := make([]any, 0, len(typed))
		for _, nested := range typed {
			result = append(result, rewriteStrings(nested, rewrite))
		}

		return result
	}

	return value
}
