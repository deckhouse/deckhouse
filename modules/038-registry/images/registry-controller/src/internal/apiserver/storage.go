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

package apiserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apiserver/pkg/registry/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	registryimagev1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/registry.deckhouse.io/v1alpha1"

	"github.com/deckhouse/registry-controller/internal/images"
)

var log = ctrl.Log.WithName("registry-images")

// Collector is what the images are assembled from — see images.Collector.
type Collector interface {
	Collect(ctx context.Context) ([]images.Answer, error)
}

// ImageStorage serves RegistryImage: get and list, nothing else. Every request asks the replicas, so
// nothing is stored and nothing goes stale beyond what each replica's index says.
type ImageStorage struct {
	Collector Collector
}

var (
	_ rest.Storage                  = (*ImageStorage)(nil)
	_ rest.Scoper                   = (*ImageStorage)(nil)
	_ rest.Getter                   = (*ImageStorage)(nil)
	_ rest.Lister                   = (*ImageStorage)(nil)
	_ rest.SingularNameProvider     = (*ImageStorage)(nil)
	_ rest.ShortNamesProvider       = (*ImageStorage)(nil)
	_ rest.CategoriesProvider       = (*ImageStorage)(nil)
	_ rest.TableConvertor           = (*ImageStorage)(nil)
	_ rest.GroupVersionKindProvider = (*ImageStorage)(nil)
)

func resourceName() string { return registryimagev1alpha1.RegistryImageResource }

func (s *ImageStorage) New() runtime.Object     { return &registryimagev1alpha1.RegistryImage{} }
func (s *ImageStorage) NewList() runtime.Object { return &registryimagev1alpha1.RegistryImageList{} }
func (s *ImageStorage) Destroy()                {}
func (s *ImageStorage) NamespaceScoped() bool   { return false }

func (s *ImageStorage) GetSingularName() string { return registryimagev1alpha1.RegistryImageSingular }

func (s *ImageStorage) ShortNames() []string {
	return []string{registryimagev1alpha1.RegistryImageShortName}
}

func (s *ImageStorage) Categories() []string { return []string{"registry"} }

func (s *ImageStorage) GroupVersionKind(schema.GroupVersion) schema.GroupVersionKind {
	return registryimagev1alpha1.GroupVersion.WithKind(registryimagev1alpha1.RegistryImageKind)
}

// Get is the image by its name: the digest with a dash for the colon.
func (s *ImageStorage) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	digest, ok := registryimagev1alpha1.DigestOf(name)
	if !ok {
		return nil, apierrors.NewNotFound(registryimagev1alpha1.Resource(resourceName()), name)
	}
	all, err := s.collect(ctx)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Digest == digest {
			return &all[i], nil
		}
	}
	return nil, apierrors.NewNotFound(registryimagev1alpha1.Resource(resourceName()), name)
}

// selectableFields are the fields an image list can be selected by.
var selectableFields = []string{"metadata.name", "state", "inSet"}

// convertFieldLabel lets the generic list handler accept selectableFields, which it refuses otherwise.
func convertFieldLabel(label, value string) (string, string, error) {
	if !slices.Contains(selectableFields, label) {
		return "", "", fmt.Errorf("%q is not a known field selector: only %s", label, strings.Join(selectableFields, ", "))
	}
	return label, value, nil
}

// List is every image the selectors pick, all of them in one answer whatever the limit.
//
// The limit is ignored, as the API conventions allow a server to: the collection is assembled whole on
// every request, so there is no revision to hand out a continue token against, and a truncated list
// without one is read as the whole list. kubectl asks for 500 at a time, and a cache holds more images
// than that — the rest, the broken ones among them, would silently not be there.
func (s *ImageStorage) List(ctx context.Context, opts *metainternalversion.ListOptions) (runtime.Object, error) {
	if opts == nil {
		opts = &metainternalversion.ListOptions{}
	}
	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	field := opts.FieldSelector
	if field == nil {
		field = fields.Everything()
	}
	for _, requirement := range field.Requirements() {
		if !slices.Contains(selectableFields, requirement.Field) {
			return nil, apierrors.NewBadRequest(fmt.Sprintf(
				"cannot list %s by %q: only %s are supported", resourceName(), requirement.Field, strings.Join(selectableFields, ", ")))
		}
	}

	all, err := s.collect(ctx)
	if err != nil {
		return nil, err
	}
	list := &registryimagev1alpha1.RegistryImageList{
		TypeMeta: metav1.TypeMeta{
			APIVersion: registryimagev1alpha1.GroupVersion.String(),
			Kind:       registryimagev1alpha1.RegistryImageKind + "List",
		},
	}
	for i := range all {
		image := &all[i]
		if !matchesLabels(label, image) {
			continue
		}
		if !field.Matches(fields.Set{
			"metadata.name": image.Name,
			"state":         string(image.State),
			"inSet":         fmt.Sprint(image.InSet),
		}) {
			continue
		}
		list.Items = append(list.Items, *image)
	}
	return list, nil
}

// matchesLabels is whether the selector picks the image, with the module label standing for every module
// of the image.
//
// A label holds one value, and an image shared by modules carries the first of them; selected by it
// alone, an image of prometheus and userAuthz would not be found by userAuthz. So a requirement on the
// module is asked of each module: one that asks for a module holds when any of them is it, and one that
// rules a module out holds only when none of them is — `module!=prometheus` does not list an image
// prometheus shares with another module.
func matchesLabels(selector labels.Selector, image *registryimagev1alpha1.RegistryImage) bool {
	set := labels.Set(image.Labels)
	requirements, selectable := selector.Requirements()
	if !selectable {
		return selector.Matches(set)
	}

	var modules []string
	if module, labeled := set[registryimagev1alpha1.ModuleLabel]; labeled {
		modules = append(modules, module)
	}
	for _, owner := range image.Owners {
		if owner.Module != "" {
			modules = append(modules, images.LabelValue(owner.Module))
		}
	}

	for _, requirement := range requirements {
		if requirement.Key() != registryimagev1alpha1.ModuleLabel || len(modules) == 0 {
			if !requirement.Matches(set) {
				return false
			}
			continue
		}
		matchesModule := func(module string) bool {
			return requirement.Matches(labels.Set{registryimagev1alpha1.ModuleLabel: module})
		}
		switch requirement.Operator() {
		case selection.NotEquals, selection.NotIn, selection.DoesNotExist:
			if slices.ContainsFunc(modules, func(m string) bool { return !matchesModule(m) }) {
				return false
			}
		default:
			if !slices.ContainsFunc(modules, matchesModule) {
				return false
			}
		}
	}
	return true
}

// collect is every image the replicas know of, or why there is no answer to give.
//
// A replica that did not answer is Unknown on every image, as long as another one answered. When none
// did, the answer is an error rather than an empty list: "No resources found" says the cache holds
// nothing, which is not what is known — on a single master that is the state after every restart of
// the syncer until it has read its store.
func (s *ImageStorage) collect(ctx context.Context) ([]registryimagev1alpha1.RegistryImage, error) {
	answers, err := s.Collector.Collect(ctx)
	if errors.Is(err, images.ErrNoCache) {
		return nil, nil
	}
	if err != nil {
		return nil, apierrors.NewServiceUnavailable(err.Error())
	}

	var silent []string
	for _, answer := range answers {
		if answer.Err != nil {
			log.Error(answer.Err, "a storage replica did not answer for the images it holds", "node", answer.Node)
		}
		if answer.Err != nil || answer.Index == nil || !answer.Index.Ready {
			silent = append(silent, answer.Node+": "+images.SilentMessage(answer))
		}
	}
	if len(silent) == len(answers) {
		if len(answers) == 0 {
			return nil, apierrors.NewServiceUnavailable("no storage replica has reported itself yet")
		}
		return nil, apierrors.NewServiceUnavailable("no storage replica can say which images it holds: " +
			strings.Join(silent, "; "))
	}
	return images.Merge(answers), nil
}

// ConvertToTable is what `kubectl get registryimages` prints.
func (s *ImageStorage) ConvertToTable(_ context.Context, object runtime.Object, _ runtime.Object) (*metav1.Table, error) {
	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: "The digest of the image's manifest, with a dash for the colon."},
			{Name: "Module", Type: "string", Description: "The module the image belongs to."},
			{Name: "Image", Type: "string", Description: "The name of the image within the module."},
			{Name: "State", Type: "string", Description: "The worst state of the image among the replicas."},
			{Name: "Present", Type: "string", Description: "On how many replicas the image is whole."},
			{Name: "In-Set", Type: "boolean", Description: "Whether the cluster declares the image."},
			{Name: "Size", Type: "string", Description: "The full size of the image."},
			{Name: "Reclaimable", Type: "string", Description: "What removing the image alone would free.", Priority: 1},
			{Name: "Versions", Type: "string", Description: "The releases or module versions that declare the image.", Priority: 1},
			{Name: "Repository", Type: "string", Description: "Where in the cache the image is linked from.", Priority: 1},
		},
	}

	var items []registryimagev1alpha1.RegistryImage
	switch typed := object.(type) {
	case *registryimagev1alpha1.RegistryImage:
		items = []registryimagev1alpha1.RegistryImage{*typed}
	case *registryimagev1alpha1.RegistryImageList:
		items = typed.Items
		table.ListMeta = typed.ListMeta
	default:
		return nil, fmt.Errorf("cannot print %T", object)
	}

	for i := range items {
		image := &items[i]
		module, name, versions := "", "", ""
		if len(image.Owners) > 0 {
			owner := image.Owners[0]
			module, name = owner.Module, owner.Image
			if name == "" {
				name = strings.ToLower(string(owner.Kind))
			}
			versions = strings.Join(owner.Versions, ",")
			if len(image.Owners) > 1 {
				module += fmt.Sprintf(" (+%d)", len(image.Owners)-1)
			}
		}
		present := 0
		for _, replica := range image.Replicas {
			if replica.State == registryimagev1alpha1.ImagePresent {
				present++
			}
		}
		repository := ""
		if len(image.Repositories) > 0 {
			repository = image.Repositories[0]
		}
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells: []interface{}{
				image.Name, module, name, string(image.State),
				fmt.Sprintf("%d/%d", present, len(image.Replicas)), image.InSet,
				quantity(image.Size), quantity(image.ReclaimableSize), versions, repository,
			},
			Object: runtime.RawExtension{Object: image},
		})
	}
	return table, nil
}

// quantity prints a size in binary units, to a tenth.
func quantity(bytes *int64) string {
	if bytes == nil {
		return ""
	}
	value := float64(*bytes)
	for _, unit := range []string{"", "Ki", "Mi", "Gi", "Ti"} {
		if value < 1024 || unit == "Ti" {
			if unit == "" {
				return fmt.Sprintf("%d", *bytes)
			}
			return fmt.Sprintf("%.1f%s", value, unit)
		}
		value /= 1024
	}
	return ""
}
