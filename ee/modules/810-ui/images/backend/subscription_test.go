package main

import (
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_parseIdentifierGroupResource(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		wantGr  schema.GroupResource
		wantErr bool
	}{
		{
			name:    "empty string",
			wantErr: true,
		},
		{
			name:   "nodes",
			arg:    "nodes",
			wantGr: schema.GroupResource{Resource: "nodes"},
		},
		{
			name:   "deployments.apps",
			arg:    "deployments.apps",
			wantGr: schema.GroupResource{Group: "apps", Resource: "deployments"},
		},
		{
			name:   "openstackinstanceclasses.deckhouse.io",
			arg:    "openstackinstanceclasses.deckhouse.io",
			wantGr: schema.GroupResource{Group: "deckhouse.io", Resource: "openstackinstanceclasses"},
		},
		{
			name:   "servicemonitors.monitoring.coreos.com",
			arg:    "servicemonitors.monitoring.coreos.com",
			wantGr: schema.GroupResource{Group: "monitoring.coreos.com", Resource: "servicemonitors"},
		},
		{
			name:   "ingresses.networking.k8s.io",
			arg:    "ingresses.networking.k8s.io",
			wantGr: schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotGr, err := parseIdentifierGroupResource(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIdentifierGroupResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotGr, tt.wantGr) {
				gotStr := fmt.Sprintf("(g:%q, r:%q)", gotGr.Group, gotGr.Resource)
				wantStr := fmt.Sprintf("(g:%q, r:%q)", tt.wantGr.Group, tt.wantGr.Resource)
				t.Errorf("parseIdentifierGroupResource() = %v, want %v", gotStr, wantStr)
			}
		})
	}
}
