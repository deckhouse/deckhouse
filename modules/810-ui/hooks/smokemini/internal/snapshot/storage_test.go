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
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

func Test_NewPvcTermination(t *testing.T) {
	type args struct {
		name       string
		deletionTs *metav1.Time
	}
	tests := []struct {
		name string
		args args
		want PvcTermination
	}{
		{
			name: "pending pod",
			args: args{name: "xx"},
			want: PvcTermination{Name: "xx", IsTerminating: false},
		},
		{
			name: "not pending pod",
			args: args{name: "yy", deletionTs: &metav1.Time{Time: time.Now()}},
			want: PvcTermination{Name: "yy", IsTerminating: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := newPvcObject(tt.args.name, tt.args.deletionTs)

			parsed, err := NewPvcTermination(obj)

			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, parsed)
			}
		})
	}
}

func newPvcObject(name string, deletionTs *metav1.Time) *unstructured.Unstructured {
	pvc := &v1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			DeletionTimestamp: deletionTs,
		},
		Spec:   v1.PersistentVolumeClaimSpec{},
		Status: v1.PersistentVolumeClaimStatus{},
	}

	manifest, err := json.Marshal(pvc)
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
