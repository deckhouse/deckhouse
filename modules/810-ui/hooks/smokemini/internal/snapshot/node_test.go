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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

type testNodeObjectArgs struct {
	name          string
	zone          string
	isReady       bool
	isSchedulable bool
}

func Test_NewNode(t *testing.T) {
	tests := []struct {
		name string
		args testNodeObjectArgs
		want Node
	}{

		{
			name: "parses absent zone as default zone",
			args: testNodeObjectArgs{
				name: "master-0",
			},
			want: Node{
				Name: "master-0",
				Zone: defaultZone,
			},
		},
		{
			name: "passes zone as is",
			args: testNodeObjectArgs{
				name: "master-0",
				zone: "zone-a",
			},
			want: Node{
				Name: "master-0",
				Zone: "zone-a",
			},
		},
		{
			name: "schedulable when both ready and not unschedulable",
			args: testNodeObjectArgs{
				name:          "master-1",
				isReady:       true,
				isSchedulable: true,
			},
			want: Node{
				Name:        "master-1",
				Zone:        defaultZone,
				Schedulable: true,
			},
		},
		{
			name: "not schedulable when unschedulabe",
			args: testNodeObjectArgs{
				name:          "system-b",
				isReady:       true,
				isSchedulable: false,
			},
			want: Node{
				Name:        "system-b",
				Zone:        defaultZone,
				Schedulable: false,
			},
		},
		{
			name: "not schedulable when not ready",
			args: testNodeObjectArgs{
				name:          "system-x",
				isReady:       false,
				isSchedulable: true,
			},
			want: Node{
				Name:        "system-x",
				Zone:        defaultZone,
				Schedulable: false,
			},
		},
		{
			name: "not schedulable when both not ready and unschedulable",
			args: testNodeObjectArgs{
				name: "master-1",
			},
			want: Node{
				Name:        "master-1",
				Zone:        defaultZone,
				Schedulable: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := newNodeObject(tt.args)

			parsed, err := NewNode(obj)

			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, parsed)
			}
		})
	}
}

func newNodeObject(args testNodeObjectArgs) *unstructured.Unstructured {
	node := v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: args.name,
		},
		Spec: v1.NodeSpec{
			Unschedulable: !args.isSchedulable,
		},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{{
				Type:   v1.NodeReady,
				Status: v1.ConditionFalse,
			}},
		},
	}

	if args.isReady {
		node.Status.Conditions[0].Status = v1.ConditionTrue
	}

	if args.zone != "" {
		node.ObjectMeta.Labels = map[string]string{
			"failure-domain.beta.kubernetes.io/zone": args.zone,
		}
	}

	manifest, err := json.Marshal(node)
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
