/*
Copyright 2023 Flant JSC

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

package snapshot

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

type testStatefulSetObjectArgs struct {
	name             string
	node             string
	zone             string
	hasStorageClass  bool
	storageClassName string
	image            string
}

func Test_NewStatefulSet(t *testing.T) {
	tests := []struct {
		name string
		args testStatefulSetObjectArgs
		want StatefulSet
	}{
		{
			name: "with storage class",
			args: testStatefulSetObjectArgs{
				name:             "smoke-mini-b",
				node:             "worker-1",
				zone:             "zone-a",
				hasStorageClass:  true,
				storageClassName: "rbd-x",
				image:            "reg.io/repo/img:tag",
			},
			want: StatefulSet{
				Index:        "b",
				Zone:         "zone-a",
				Image:        "reg.io/repo/img:tag",
				Node:         "worker-1",
				StorageClass: "rbd-x",
			},
		},
		{
			name: "without storage class",
			args: testStatefulSetObjectArgs{
				name:  "smoke-mini-e",
				node:  "control-plane-1",
				zone:  "enoz",
				image: "d8.io/repo/img:tag",
			},
			want: StatefulSet{
				Index:        "e",
				Zone:         "enoz",
				Image:        "d8.io/repo/img:tag",
				Node:         "control-plane-1",
				StorageClass: "false",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := newStatefulSetObject(tt.args)

			parsed, err := NewStatefulSet(obj)

			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, parsed)
			}
		})
	}
}

func newStatefulSetObject(args testStatefulSetObjectArgs) *unstructured.Unstructured {
	sts := appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: args.name,
			Annotations: map[string]string{
				"node": args.node,
				"zone": args.zone,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Image: args.image,
					}},
				},
			},
			VolumeClaimTemplates: nil,
		},
	}

	if args.hasStorageClass {
		sts.Spec.VolumeClaimTemplates = []v1.PersistentVolumeClaim{{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "disk",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				StorageClassName: &args.storageClassName,
			},
		}}
	}

	manifest, err := json.Marshal(sts)
	if err != nil {
		panic(err)
	}

	decUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	if _, _, err := decUnstructured.Decode(manifest, nil, obj); err != nil {
		panic(err)
	}

	return obj
}
