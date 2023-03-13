/*
Copyright 2022 Flant JSC

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

package kinds

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const (
	constraintChecksumAnnotation = "security.deckhouse.io/constraints-checksum"
	mutationChecksumAnnotation   = "security.deckhouse.io/mutations-checksum"
)

type KindTracker struct {
	client      *kubernetes.Clientset
	cmNamespace string
	cmName      string

	latestConstraintsChecksum string
	latestMutationsChecksum   string
}

func NewKindTracker(client *kubernetes.Clientset, cmNS, cmName string) *KindTracker {
	return &KindTracker{
		client:      client,
		cmNamespace: cmNS,
		cmName:      cmName,
	}
}

type resourceWithMatch interface {
	GetMatchKinds() []gatekeeper.MatchKind
}

func deduplicateKinds[T resourceWithMatch](matchResources []T) (map[string]gatekeeper.MatchKind /*kinds checksum*/, string) {
	if len(matchResources) == 0 {
		return nil, ""
	}

	// deduplicate
	m := make(map[string]gatekeeper.MatchKind, 0)
	hasher := sha256.New()

	for _, resource := range matchResources {
		for _, k := range resource.GetMatchKinds() {
			sort.Strings(k.APIGroups)
			sort.Strings(k.Kinds)
			key := fmt.Sprintf("%s:%s", strings.Join(k.APIGroups, ","), strings.Join(k.Kinds, ","))

			if _, ok := m[key]; !ok {
				m[key] = k
				hasher.Write([]byte(key))
			}
		}
	}

	return m, fmt.Sprintf("%x", hasher.Sum(nil))
}

func (kt *KindTracker) UpdateTrackedObjects(constraints []gatekeeper.Constraint, mutations []gatekeeper.Mutation) {
	deduplicatedConstraintKinds, cchecksum := deduplicateKinds(constraints)
	deduplicatedMutateKinds, mchecksum := deduplicateKinds(mutations)

	if cchecksum == kt.latestConstraintsChecksum && mchecksum == kt.latestMutationsChecksum {
		return
	}

	klog.Info("Checksums are not equal. Updating")

	cm, err := kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Get(context.TODO(), kt.cmName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			err = kt.createCM()
			if err != nil {
				klog.Errorf("Create kinds cm failed: %s", err)
				return
			}
		} else {
			klog.Errorf("Get kinds cm failed: %s", err)
			return
		}
	}
	if len(cm.Annotations) == 0 {
		cm.Annotations = make(map[string]string, 0)
	}
	if len(cm.Data) == 0 {
		cm.Data = make(map[string]string)
	}

	constraintKinds := make([]gatekeeper.MatchKind, 0, len(deduplicatedConstraintKinds))
	for _, k := range deduplicatedConstraintKinds {
		constraintKinds = append(constraintKinds, k)
	}

	mutationKinds := make([]gatekeeper.MatchKind, 0, len(deduplicatedMutateKinds))
	for _, m := range deduplicatedMutateKinds {
		mutationKinds = append(mutationKinds, m)
	}

	cm.Annotations[constraintChecksumAnnotation] = cchecksum
	cm.Annotations[mutationChecksumAnnotation] = mchecksum

	// convert kinds to the resources
	resourceConstraintsData, resourceMutationsData, err := kt.convertKinds(constraintKinds, mutationKinds)
	if err != nil {
		klog.Errorf("Convert kinds to resources failed. Try later")
		return
	}
	cm.Data["validate-resources.yaml"] = string(resourceConstraintsData)
	cm.Data["mutate-resources.yaml"] = string(resourceMutationsData)

	_, err = kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Update(context.TODO(), cm, v1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update tracked objects failed: %s", err)
		return
	}

	kt.latestConstraintsChecksum = cchecksum
	kt.latestMutationsChecksum = mchecksum
}

func (kt *KindTracker) convertKinds(constraintKinds, mutateKinds []gatekeeper.MatchKind) ( /*constraintData*/ []byte /*mutateData*/, []byte, error) {
	apiRes, err := restmapper.GetAPIGroupResources(kt.client.Discovery())
	if err != nil {
		return nil, nil, err
	}

	rmatch := resourceMatcher{
		apiGroupResources: apiRes,
		mapper:            restmapper.NewDiscoveryRESTMapper(apiRes),
	}

	constraintData, err := rmatch.convertKindsToResource(constraintKinds)
	if err != nil {
		return nil, nil, err
	}

	mutateData, err := rmatch.convertKindsToResource(mutateKinds)
	if err != nil {
		return nil, nil, err
	}

	return constraintData, mutateData, nil
}

func (kt *KindTracker) FindInitialChecksum() error {
	cm, err := kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Get(context.TODO(), kt.cmName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Info("Track objects configmap not found. Creating.")
			err = kt.createCM()
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	kt.latestConstraintsChecksum = cm.Annotations[constraintChecksumAnnotation]
	kt.latestMutationsChecksum = cm.Annotations[mutationChecksumAnnotation]

	return nil
}

func (kt *KindTracker) createCM() error {
	cm := &corev1.ConfigMap{
		TypeMeta: v1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      kt.cmName,
			Namespace: kt.cmNamespace,
			Labels: map[string]string{
				"owner": "constraint-exporter",
			},
			Annotations: nil,
		},
		Data: nil,
	}

	_, err := kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Create(context.TODO(), cm, v1.CreateOptions{})

	return err
}

type matchResource struct {
	APIGroups []string `json:"apiGroups"`
	Resources []string `json:"resources"`
}

type resourceMatcher struct {
	apiGroupResources []*restmapper.APIGroupResources
	mapper            meta.RESTMapper
}

func (rm resourceMatcher) findGVKsForWildcard(kind string) []schema.GroupVersionKind {
	matchGVKs := make([]schema.GroupVersionKind, 0)

	for _, apiGroupRes := range rm.apiGroupResources {
	versionLoop:
		for version, apiResources := range apiGroupRes.VersionedResources {
			for _, apiRes := range apiResources {
				if apiRes.Kind == kind {
					gvk := schema.GroupVersionKind{
						Group:   apiGroupRes.Group.Name,
						Kind:    apiRes.Kind,
						Version: version,
					}
					matchGVKs = append(matchGVKs, gvk)
					break versionLoop
				}
			}
		}
	}

	return matchGVKs
}

func (rm resourceMatcher) convertKindsToResource(kinds []gatekeeper.MatchKind) ([]byte, error) {
	res := make([]matchResource, 0, len(kinds))

	for _, mk := range kinds {
		uniqGroups := make(map[string]struct{})
		uniqResources := make(map[string]struct{})

		for _, apiGroup := range mk.APIGroups {
			for _, kind := range mk.Kinds {
				if apiGroup == "*" {
					gvks := rm.findGVKsForWildcard(kind)
					for _, gvk := range gvks {
						restMapping, err := rm.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
						if err != nil {
							// skip outdated resources, like extensions/Ingress
							klog.Warningf("Skip wildcard resource mapping. Group: %q, Kind: %q, Version: %q. Error: %q", gvk.Group, gvk.Kind, gvk.Version, err)
							continue
						}

						uniqGroups[restMapping.Resource.Group] = struct{}{}
						uniqResources[restMapping.Resource.Resource] = struct{}{}
					}
				} else {
					restMapping, err := rm.mapper.RESTMapping(schema.GroupKind{
						Group: apiGroup,
						Kind:  kind,
					})
					if err != nil {
						// skip outdated resources, like extensions/Ingress
						klog.Warningf("Skip resource mapping. Group: %q, Kind: %q. Error: %q", apiGroup, kind, err)
						continue
					}

					uniqGroups[restMapping.Resource.Group] = struct{}{}
					uniqResources[restMapping.Resource.Resource] = struct{}{}
				}
			}
		}

		groups := make([]string, 0, len(mk.APIGroups))
		resources := make([]string, 0, len(mk.Kinds))

		for k := range uniqGroups {
			groups = append(groups, k)
		}

		for k := range uniqResources {
			resources = append(resources, k)
		}

		res = append(res, matchResource{
			APIGroups: groups,
			Resources: resources,
		})
	}

	return yaml.Marshal(res)
}
