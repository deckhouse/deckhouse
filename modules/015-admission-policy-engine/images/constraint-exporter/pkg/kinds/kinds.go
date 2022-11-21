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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const (
	checksumAnnotation = "security.deckhouse.io/constraints-checksum"
)

type KindTracker struct {
	client      *kubernetes.Clientset
	cmNamespace string
	cmName      string

	trackKinds     bool
	trackResources bool

	latestChecksum string
}

func NewKindTracker(client *kubernetes.Clientset, cmNS, cmName string, trackKinds, trackResources bool) *KindTracker {
	return &KindTracker{
		client:         client,
		cmNamespace:    cmNS,
		cmName:         cmName,
		trackKinds:     trackKinds,
		trackResources: trackResources,
	}
}

func deduplicateKinds(constraints []gatekeeper.Constraint) (map[string]gatekeeper.MatchKind /*kinds checksum*/, string) {
	if len(constraints) == 0 {
		return nil, ""
	}

	// deduplicate
	m := make(map[string]gatekeeper.MatchKind, 0)
	hasher := sha256.New()

	for _, con := range constraints {
		for _, k := range con.Spec.Match.Kinds {
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

func (kt *KindTracker) UpdateTrackedObjects(constraints []gatekeeper.Constraint) {
	if len(constraints) == 0 {
		return
	}

	deduplicated, checksum := deduplicateKinds(constraints)

	if len(deduplicated) == 0 {
		return
	}

	if checksum == kt.latestChecksum {
		return
	}

	klog.Info("Checksum is not equal. Updating")

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

	kinds := make([]gatekeeper.MatchKind, 0, len(deduplicated))
	for _, k := range deduplicated {
		kinds = append(kinds, k)
	}

	if len(cm.Annotations) == 0 {
		cm.Annotations = make(map[string]string, 0)
	}

	cm.Annotations[checksumAnnotation] = checksum
	if len(cm.Data) == 0 {
		cm.Data = make(map[string]string)
	}

	if kt.trackKinds {
		data, _ := yaml.Marshal(kinds)
		cm.Data["validate-kinds.yaml"] = string(data)
	}

	if kt.trackResources {
		// convert kinds to the resources
		resourceData, err := kt.convertToResources(kinds)
		if err != nil {
			klog.Errorf("Convert kinds to resources failed. Try later")
			return
		}
		cm.Data["validate-resources.yaml"] = string(resourceData)
	}

	_, err = kt.client.CoreV1().ConfigMaps(kt.cmNamespace).Update(context.TODO(), cm, v1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update tracked objects failed: %s", err)
		return
	}
	kt.latestChecksum = checksum
}

func (kt *KindTracker) convertToResources(kinds []gatekeeper.MatchKind) ([]byte, error) {
	apiRes, err := restmapper.GetAPIGroupResources(kt.client.Discovery())
	if err != nil {
		return nil, err
	}

	rmapper := restmapper.NewDiscoveryRESTMapper(apiRes)

	res := make([]matchResource, 0, len(kinds))

	for _, mk := range kinds {
		uniqGroups := make(map[string]struct{})
		uniqResources := make(map[string]struct{})

		for _, apiGroup := range mk.APIGroups {
			for _, kind := range mk.Kinds {
				rm, err := rmapper.RESTMapping(schema.GroupKind{
					Group: apiGroup,
					Kind:  kind,
				})
				if err != nil {
					// skip outdated resources, like extensions/Ingress
					klog.Warningf("Skip resource mapping. Group: %q, Kind: %q. Error: %q", apiGroup, kind, err)
					continue
				}

				uniqGroups[rm.Resource.Group] = struct{}{}
				uniqResources[rm.Resource.Resource] = struct{}{}
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

	v, ok := cm.Annotations[checksumAnnotation]
	if !ok {
		return nil
	}

	kt.latestChecksum = v
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
