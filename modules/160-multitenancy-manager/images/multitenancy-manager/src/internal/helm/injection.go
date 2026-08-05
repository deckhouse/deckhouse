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
	"controller/apis/deckhouse.io/v1alpha2"
)

// ErrParameterInjection is returned when a project parameter does not stay a value in the rendered
// manifests but becomes part of their structure.
var ErrParameterInjection = errors.New("a project parameter changes the structure of the rendered manifests")

// lineBreakToSpace replaces the characters that end a YAML scalar. A parameter carrying one of them
// can close the value it was substituted into and have the rest of itself read as manifest structure
// -- a new key, or, after a document separator, a whole new object. The rendered manifests are
// applied by a ServiceAccount bound to cluster-admin, so such an object is applied with cluster-admin.
var lineBreakToSpace = strings.NewReplacer("\n", " ", "\r", " ", "\u0085", " ", "\u2028", " ", "\u2029", " ")

// ensureParametersStayValues renders the template and checks that no parameter became structure in
// it. Callers that have the render already should use ensureRenderedParametersStayValues.
func (c *Client) ensureParametersStayValues(project *v1alpha2.Project, template *v1alpha1.ProjectTemplate) error {
	if _, carries := sanitizedParameters(project); !carries {
		return nil
	}

	actual, err := c.renderTemplate(project, template)
	if err != nil {
		return err
	}

	return c.ensureRenderedParametersStayValues(project, template, actual)
}

// ensureRenderedParametersStayValues refuses parameters that render into structure instead of into
// values, given the manifests the project renders into as it stands.
//
// Quoting every substitution in the shipped templates fixes those templates. It cannot fix the ones
// a cluster administrator writes: a ProjectTemplate is free-form text, and its author is not obliged
// to know that an unquoted substitution ends at the first line break. This check is what covers them
// -- that one escape, in any template, without reading the template.
//
// The method: render the template a second time with every line break in the parameters replaced by
// a space, then compare the two outputs as sets of objects, with the same replacement applied to
// every string inside. For an input that carries no line breaks the two renders are identical by
// construction, so there is nothing to report; for an input whose line breaks only ever land inside
// a value -- a quoted substitution, a block scalar -- the outputs differ in that value alone, and the
// replacement makes them equal again. What is left is the case where the value produced structure,
// and that is what is refused, in either direction: an object the parameter conjured and an object
// it made disappear are equally interesting, since suppressing the Isolated NetworkPolicy or the
// ResourceQuota buys an attacker as much as adding a ClusterRoleBinding.
//
// What this does not cover: a substitution inside a YAML flow context ({...}, [...]) needs no line
// break to add a key or a list item, and such a parameter passes here untouched. It is bounded --
// a new document requires a --- at the start of a line, so a flow-style payload cannot conjure an
// object of its own -- but it is not nothing.
//
// A template that fans a parameter out on its line breaks (splitList "\n") or reads YAML out of one
// (fromYaml) is refused as well: it does turn the parameter into structure, and nothing here can
// tell that structure from an injected one. That is why the check runs where a release is about to
// be applied rather than on every reconcile -- a project that already reconciled is left alone.
func (c *Client) ensureRenderedParametersStayValues(project *v1alpha2.Project, template *v1alpha1.ProjectTemplate, actual string) error {
	parameters, carries := sanitizedParameters(project)
	if !carries {
		return nil
	}

	sanitized := project.DeepCopy()
	sanitized.Spec.Parameters = parameters

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

	if added := extraObjects(actualObjects, expectedObjects); len(added) > 0 {
		return fmt.Errorf("%w, adding: %s", ErrParameterInjection, strings.Join(added, ", "))
	}

	if removed := extraObjects(expectedObjects, actualObjects); len(removed) > 0 {
		return fmt.Errorf("%w, removing: %s", ErrParameterInjection, strings.Join(removed, ", "))
	}

	return nil
}

// sanitizedParameters returns the parameters with their line breaks replaced, and whether they
// carried any. Nothing in them that can end a scalar means the second render would repeat the first,
// so the check has nothing to compare.
func sanitizedParameters(project *v1alpha2.Project) (map[string]any, bool) {
	// A project without parameters is answered separately: the rewrite turns no parameters into
	// empty ones, and DeepEqual does not call an empty map equal to a nil one.
	if len(project.Spec.Parameters) == 0 {
		return nil, false
	}

	parameters := rewriteStringsIn(project.Spec.Parameters, lineBreakToSpace.Replace)

	return parameters, !reflect.DeepEqual(parameters, project.Spec.Parameters)
}

// extraObjects describes every object the first map has and the second does not.
func extraObjects(first, second map[string]string) []string {
	var extra []string
	for canonical, description := range first {
		if _, ok := second[canonical]; !ok {
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
