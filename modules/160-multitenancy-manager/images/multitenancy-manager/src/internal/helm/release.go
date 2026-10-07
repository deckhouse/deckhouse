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
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
)

const (
	// nativeChartTemplate is the only template of the chart a natively rendered release is built
	// from (see buildEmptyChart).
	nativeChartTemplate = "templates/manifests.yaml"
	// helmTextChartTemplate is the template of the chart a Helm text is rendered with: it passes the
	// resourcesTemplate text from the release values through the Helm template engine (helmlib).
	helmTextChartTemplate = "templates/template.yaml"

	// resourcePolicy is the annotation Helm reads before it deletes an object that left the release;
	// with resourcePolicyKeep it leaves the object in place.
	resourcePolicy     = "helm.sh/resource-policy"
	resourcePolicyKeep = "keep"
)

// ProjectRelease is what the project manager reads of a deployed revision of a project release: how
// it was rendered and which objects its manifest holds.
type ProjectRelease struct {
	// Revision is the version of the release.
	Revision int
	// HelmText reports that the revision was rendered from the resourcesTemplate of a template, by
	// the chart of helmlib. A natively rendered revision is not, and neither is one of another chart.
	HelmText bool
	// Administrators are the administrators in the parameters a Helm text was rendered with, which a
	// text written for the v1alpha2 Project layout read as a list of {subject, name}. They are empty
	// for any other revision, and for parameters of another shape.
	Administrators []v1alpha3.Administrator
	// Objects are the objects of the manifest, as the post-renderer pinned them.
	Objects []ReleaseObject
}

// ReleaseObject identifies one object of a release manifest, and says how the manifest marks it.
type ReleaseObject struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	// Heritage reports that the manifest labels the object heritage: multitenancy-manager, the way the
	// post-renderer labels every object it does not skip.
	Heritage bool
	// Keep reports that the manifest annotates the object helm.sh/resource-policy: keep.
	Keep bool
}

// ReleaseChange is what an upgrade does to a deployed release: the revision it replaces, and the
// objects the release holds once it is upgraded, the way the post-renderer passes them. Helm deletes
// every object of the deployed revision that is not among the targets, unless the live object carries
// helm.sh/resource-policy: keep.
type ReleaseChange struct {
	Deployed *ProjectRelease
	Targets  []ReleaseObject
	// Native reports that the upgrade renders the release natively, from the structured fields of a
	// template.
	Native bool
}

// BeforeUpgrade runs where an upgrade is about to replace a deployed revision, and an error stops the
// upgrade. It does not run on a first install, nor on a reconcile that finds the release up to date.
type BeforeUpgrade func(ctx context.Context, change ReleaseChange) error

// newProjectRelease reads a revision of a project release: which chart rendered it, the administrators
// of a Helm text, and the objects of the manifest.
func newProjectRelease(rel *release.Release) (*ProjectRelease, error) {
	summary := &ProjectRelease{Revision: rel.Version}
	if rel.Chart != nil {
		summary.HelmText = slices.ContainsFunc(rel.Chart.Templates, func(file *chart.File) bool {
			return file.Name == helmTextChartTemplate
		})
	}
	if summary.HelmText {
		// The values are those of buildValues: the project under project, keyed by its yaml tags.
		parameters, _, _ := unstructured.NestedFieldNoCopy(rel.Config, "project", "parameters")
		fields, _ := parameters.(map[string]any)
		summary.Administrators = ParameterAdministrators(fields)
	}

	objects, err := manifestObjects(rel.Manifest)
	if err != nil {
		return nil, fmt.Errorf("read the manifest of the '%s' release revision %d: %w", rel.Name, rel.Version, err)
	}
	summary.Objects = objects
	return summary, nil
}

// ParameterAdministrators reads the administrators parameter of the v1alpha2 Project layout: a list of
// {subject, name}. An entry of another shape names no administrator; keys next to subject and name
// are not read.
func ParameterAdministrators(parameters map[string]any) []v1alpha3.Administrator {
	entries, _ := parameters["administrators"].([]any)
	var administrators []v1alpha3.Administrator
	for _, entry := range entries {
		fields, _ := entry.(map[string]any)
		kind, _ := fields["subject"].(string)
		name, _ := fields["name"].(string)
		if kind == "" || name == "" {
			continue
		}
		administrators = append(administrators, v1alpha3.Administrator{Kind: kind, Name: name})
	}
	return administrators
}

// deployedRevision is the revision an upgrade replaces and diffs against, picked the way Helm picks
// it: the latest deployed one, or else the latest when it failed or was superseded. Without one the
// upgrade installs anew. releases are sorted newest first.
func deployedRevision(releases []*release.Release) *release.Release {
	if len(releases) == 0 {
		return nil
	}
	for _, revision := range releases {
		if revision.Info != nil && revision.Info.Status == release.StatusDeployed {
			return revision
		}
	}
	if latest := releases[0]; latest.Info != nil &&
		(latest.Info.Status == release.StatusFailed || latest.Info.Status == release.StatusSuperseded) {
		return latest
	}
	return nil
}

// releaseChange reads the deployed revision and lists the objects the release holds once it is
// upgraded from the chart and the values, or from the native manifests: what the post-renderer
// passes of their render, a kind the cluster does not serve left out as the upgrade leaves it out.
// The post-renderer runs on a copy of the project, so the status the upgrade fills stays as it is.
func (c *Client) releaseChange(
	project *v1alpha3.Project,
	deployed *release.Release,
	ch *chart.Chart,
	values map[string]any,
	manifests string,
	versions map[string]struct{},
) (ReleaseChange, error) {
	summary, err := newProjectRelease(deployed)
	if err != nil {
		return ReleaseChange{}, err
	}

	rendered := ""
	if manifests == "" {
		if rendered, err = renderChart(ch, values, project.Name); err != nil {
			return ReleaseChange{}, err
		}
	}
	post := newPostRenderer(project.DeepCopy(), versions, c.logger, false)
	post.manifests = manifests
	out, err := post.Run(bytes.NewBufferString(rendered))
	if err != nil {
		return ReleaseChange{}, fmt.Errorf("post render: %w", err)
	}
	targets, err := manifestObjects(out.String())
	if err != nil {
		return ReleaseChange{}, fmt.Errorf("read the post-rendered manifests: %w", err)
	}
	return ReleaseChange{Deployed: summary, Targets: targets, Native: manifests != ""}, nil
}

// manifestObjects lists the objects of a manifest, sorted by kind, namespace and name.
func manifestObjects(manifest string) ([]ReleaseObject, error) {
	docs := releaseutil.SplitManifests(manifest)
	objects := make([]ReleaseObject, 0, len(docs))
	for _, doc := range docs {
		var head struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
			Metadata   struct {
				Name        string            `json:"name"`
				Namespace   string            `json:"namespace"`
				Labels      map[string]string `json:"labels"`
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
		}
		if err := yaml.Unmarshal([]byte(doc), &head); err != nil {
			return nil, fmt.Errorf("parse a manifest document: %w", err)
		}
		if head.Kind == "" {
			continue
		}
		objects = append(objects, ReleaseObject{
			APIVersion: head.APIVersion,
			Kind:       head.Kind,
			Namespace:  head.Metadata.Namespace,
			Name:       head.Metadata.Name,
			Heritage:   head.Metadata.Labels[v1alpha3.ResourceLabelHeritage] == v1alpha3.ResourceHeritageMultitenancy,
			Keep:       head.Metadata.Annotations[resourcePolicy] == resourcePolicyKeep,
		})
	}
	slices.SortFunc(objects, func(a, b ReleaseObject) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name))
	})
	return objects, nil
}
