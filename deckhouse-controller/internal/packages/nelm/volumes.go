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

// The render patches resize the volume claim templates, but the API server rejects the change and
// the claims made from them are not in the release. So before nelm installs it, the claims are
// grown here and the StatefulSet is deleted, its pods left running, for nelm to create it again.
// nelm's delete-propagation annotation would stay in the release and orphan the pods on uninstall.

package nelm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/werf/nelm/v2/pkg/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// statefulSetKind is the kind whose disks come from its volume claim templates.
const statefulSetKind = "StatefulSet"

// expandClaims grows the disks of an application before nelm installs its release: only an
// application sizes them, and with the resource requests disabled the release is applied as before.
func (s *Service) expandClaims(ctx context.Context, namespace string, pkg Package, renderedManifests string) error {
	if _, ok := pkg.(application); !ok || !app.ResourceRequestsEnabled() {
		return nil
	}

	return expandStatefulSetClaims(ctx, s.kube, s.logger, namespace, renderedManifests, resourceRequests(pkg))
}

// expandStatefulSetClaims grows the bound claims of every rendered StatefulSet in the cluster to
// the sizes spec.resourceRequests sets for its templates, and recreates the ones whose templates
// grow. Growing is idempotent, so a failure part way leaves the rest to a later apply.
func expandStatefulSetClaims(ctx context.Context, client kubernetes.Interface, logger *log.Logger, namespace, renderedManifests string, requests []resourcesize.Request) error {
	if !slices.ContainsFunc(requests, hasDiskSizes) {
		return nil
	}

	statefulSets, err := decodeStatefulSets(renderedManifests, namespace)
	if err != nil {
		return err
	}

	for _, request := range requests {
		// A request for a StatefulSet the chart does not render is ignored, as the CR field documents.
		rendered, ok := statefulSets[request.Name]
		if !ok || !hasDiskSizes(request) || keptByPolicy(rendered.Metadata.Annotations) {
			continue
		}

		live, err := client.AppsV1().StatefulSets(rendered.Metadata.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			// nelm creates it with the rendered templates, and its controller makes the claims from them.
			continue
		}

		if err != nil {
			return fmt.Errorf("get StatefulSet %s: %w", request.Name, err)
		}

		sizes, resized, err := requestedSizes(live, request)
		if err != nil {
			return err
		}

		// Recreated with a new selector, the StatefulSet would leave its running pods to nobody.
		// Without a resize nothing is recreated, and nelm fails on the selector as before.
		if resized && !equality.Semantic.DeepEqual(rendered.Spec.Selector, live.Spec.Selector) {
			return fmt.Errorf("StatefulSet %s: its selector changes along with the sizes of its volume claim templates, which cannot be applied together",
				request.Name)
		}

		for _, template := range sizes {
			if err := patchClaimSizes(ctx, client, logger, live, template.name, template.size); err != nil {
				return err
			}
		}

		// The claims are grown first: one the API server refuses leaves the StatefulSet in place.
		if resized {
			if err := recreateStatefulSet(ctx, client, logger, live); err != nil {
				return err
			}
		}
	}

	return nil
}

// hasDiskSizes reports whether the request sets the size of a volume claim template of a
// StatefulSet.
func hasDiskSizes(request resourcesize.Request) bool {
	return request.Kind == statefulSetKind && slices.ContainsFunc(request.VolumeClaimTemplates, func(t resourcesize.VolumeClaimTemplate) bool {
		return t.Storage != ""
	})
}

// renderedStatefulSet is the part of a rendered StatefulSet the step compares with the cluster.
type renderedStatefulSet struct {
	Metadata struct {
		Name        string            `json:"name"`
		Namespace   string            `json:"namespace"`
		Annotations map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Selector *metav1.LabelSelector `json:"selector"`
	} `json:"spec"`
}

// decodeStatefulSets decodes the StatefulSets of a rendered multi-document YAML manifest, by
// name. One without a namespace is in the release namespace.
func decodeStatefulSets(renderedManifests, namespace string) (map[string]*renderedStatefulSet, error) {
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedManifests), 4096)

	statefulSets := make(map[string]*renderedStatefulSet)

	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return statefulSets, nil
			}

			// A document nelm renders but the step cannot read would leave its disks unchecked.
			return nil, fmt.Errorf("decode rendered manifests: %w", err)
		}

		// An empty document or one of comments only is no object; the kind is read first, as the
		// spec of another kind, a Service say, does not decode as a StatefulSet's.
		meta := new(typeMeta)
		if len(raw) == 0 || json.Unmarshal(raw, meta) != nil || meta.APIVersion != workloadAPIVersion || meta.Kind != statefulSetKind {
			continue
		}

		sts := new(renderedStatefulSet)
		if err := json.Unmarshal(raw, sts); err != nil {
			return nil, fmt.Errorf("decode StatefulSet: %w", err)
		}

		if sts.Metadata.Namespace == "" {
			sts.Metadata.Namespace = namespace
		}

		statefulSets[sts.Metadata.Name] = sts
	}
}

// templateSize is the size the request sets for a volume claim template of the live StatefulSet.
type templateSize struct {
	name string
	size resource.Quantity
}

// requestedSizes returns the sizes the request sets for the templates of the live StatefulSet, and
// whether one grows its template. A template cannot shrink: the replicas made from now on would get
// smaller disks than the ones running.
func requestedSizes(sts *appsv1.StatefulSet, request resourcesize.Request) ([]templateSize, bool, error) {
	var (
		sizes   []templateSize
		resized bool
	)

	for _, template := range request.VolumeClaimTemplates {
		i := slices.IndexFunc(sts.Spec.VolumeClaimTemplates, func(t corev1.PersistentVolumeClaim) bool {
			return t.Name == template.Name
		})
		// An empty size is not set, and a template the StatefulSet does not have is ignored.
		if template.Storage == "" || i < 0 {
			continue
		}

		size, err := resource.ParseQuantity(template.Storage)
		if err != nil {
			return nil, false, fmt.Errorf("StatefulSet %s: parse size of volume claim template %s: %w", sts.Name, template.Name, err)
		}

		from := *sts.Spec.VolumeClaimTemplates[i].Spec.Resources.Requests.Storage()
		if size.Cmp(from) < 0 {
			return nil, false, fmt.Errorf("StatefulSet %s: volume claim template %s cannot shrink from %s to %s",
				sts.Name, template.Name, from.String(), size.String())
		}

		resized = resized || size.Cmp(from) > 0
		sizes = append(sizes, templateSize{name: template.Name, size: size})
	}

	return sizes, resized, nil
}

// recreateStatefulSet deletes the StatefulSet, its pods left running, and waits until it is gone;
// the one nelm creates adopts the pods, so an unchanged pod template rolls nothing out.
func recreateStatefulSet(ctx context.Context, client kubernetes.Interface, logger *log.Logger, sts *appsv1.StatefulSet) error {
	err := client.AppsV1().StatefulSets(sts.Namespace).Delete(ctx, sts.Name, metav1.DeleteOptions{
		// Orphan keeps the pods, and the claims too, whatever the retention policy says.
		PropagationPolicy: ptr.To(metav1.DeletePropagationOrphan),
		// The decision holds for this StatefulSet, not for one created in its place since.
		Preconditions: metav1.NewUIDPreconditions(string(sts.UID)),
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete StatefulSet %s: %w", sts.Name, err)
	}

	logger.Info("delete StatefulSet to recreate it with resized volume claim templates",
		slog.String("namespace", sts.Namespace), slog.String("name", sts.Name))

	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		live, err := client.AppsV1().StatefulSets(sts.Namespace).Get(ctx, sts.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		return err == nil && live.UID != sts.UID, err
	})
	if err != nil {
		// The apply deadline cancels with a cause naming what the apply waited for.
		if ctx.Err() != nil {
			err = context.Cause(ctx)
		}

		return fmt.Errorf("wait for StatefulSet %s to be deleted: %w", sts.Name, err)
	}

	return nil
}

// patchClaimSizes sets size on the bound claims made from a template of the StatefulSet, found as
// its controller makes them: named <template>-<StatefulSet>-<ordinal>, labeled with the
// matchLabels of the selector. Every ordinal counts, the claims a scale down left behind too.
func patchClaimSizes(ctx context.Context, client kubernetes.Interface, logger *log.Logger, sts *appsv1.StatefulSet, template string, size resource.Quantity) error {
	var matchLabels map[string]string
	if sts.Spec.Selector != nil {
		matchLabels = sts.Spec.Selector.MatchLabels
	}

	list, err := client.CoreV1().PersistentVolumeClaims(sts.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(matchLabels).String(),
	})
	if err != nil {
		return fmt.Errorf("list PersistentVolumeClaims of StatefulSet %s: %w", sts.Name, err)
	}

	name := regexp.MustCompile(`^` + regexp.QuoteMeta(template+"-"+sts.Name+"-") + `\d+$`)
	patch := fmt.Sprintf(`{"spec":{"resources":{"requests":{%q:%q}}}}`, corev1.ResourceStorage, size.String())

	for i := range list.Items {
		claim := &list.Items[i]
		if !name.MatchString(claim.Name) || claim.Spec.Resources.Requests.Storage().Cmp(size) >= 0 {
			continue
		}

		// The API server lets only a bound claim change its size.
		if claim.Status.Phase != corev1.ClaimBound {
			logger.Info("skip unbound PersistentVolumeClaim",
				slog.String("namespace", claim.Namespace), slog.String("name", claim.Name),
				slog.String("phase", string(claim.Status.Phase)))

			continue
		}

		_, err := client.CoreV1().PersistentVolumeClaims(claim.Namespace).Patch(ctx, claim.Name, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
		if err != nil {
			return expansionRefused(ctx, client, claim, size, err)
		}

		logger.Info("expand PersistentVolumeClaim",
			slog.String("namespace", claim.Namespace), slog.String("name", claim.Name), slog.String("size", size.String()))
	}

	return nil
}

// expansionRefused names the StorageClass of a claim the API server refused to grow when the class
// is the cause, which it reads only then; any other refusal, a quota say, is returned as it came.
func expansionRefused(ctx context.Context, client kubernetes.Interface, claim *corev1.PersistentVolumeClaim, size resource.Quantity, err error) error {
	name := ptr.Deref(claim.Spec.StorageClassName, "")
	if name == "" {
		return fmt.Errorf("PersistentVolumeClaim %s cannot be expanded: it has no StorageClass", claim.Name)
	}

	class, getErr := client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(getErr):
		return fmt.Errorf("PersistentVolumeClaim %s cannot be expanded: StorageClass %s not found", claim.Name, name)
	case getErr == nil && !ptr.Deref(class.AllowVolumeExpansion, false):
		return fmt.Errorf("PersistentVolumeClaim %s cannot be expanded: StorageClass %s does not allow volume expansion", claim.Name, name)
	}

	return fmt.Errorf("expand PersistentVolumeClaim %s to %s: %w", claim.Name, size.String(), err)
}

// keptByPolicy reports whether a resource policy of the chart keeps nelm from creating, updating or
// recreating the object. The step must leave it as it is too, or it would apply a change nelm
// would not, or delete a StatefulSet nelm does not create again.
func keptByPolicy(annotations map[string]string) bool {
	// werf's annotation takes precedence over Helm's, as nelm reads them.
	value, ok := annotations[common.AnnotationKeyHumanWerfResourcePolicy]
	if !ok {
		value = annotations[common.AnnotationKeyHumanResourcePolicy]
	}

	for _, policy := range strings.Split(value, ",") {
		switch common.ResourcePolicy(strings.TrimSpace(policy)) {
		case common.ResourcePolicySkipCreate, common.ResourcePolicySkipUpdate, common.ResourcePolicySkipRecreate:
			return true
		}
	}

	return false
}
