//go:build !integration

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

package waypointcontroller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

func withAllowedRoutesFrom(from string) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		if i.Spec.AllowedRoutes == nil {
			i.Spec.AllowedRoutes = &networkv1alpha1.AllowedRoutesConfig{}
		}
		if i.Spec.AllowedRoutes.Namespaces == nil {
			i.Spec.AllowedRoutes.Namespaces = &networkv1alpha1.RouteNamespacesConfig{}
		}
		i.Spec.AllowedRoutes.Namespaces.From = &from
	}
}

func withAllowedRoutesSelector(selector *metav1.LabelSelector) func(*networkv1alpha1.WaypointInstance) {
	return func(i *networkv1alpha1.WaypointInstance) {
		if i.Spec.AllowedRoutes == nil {
			i.Spec.AllowedRoutes = &networkv1alpha1.AllowedRoutesConfig{}
		}
		if i.Spec.AllowedRoutes.Namespaces == nil {
			i.Spec.AllowedRoutes.Namespaces = &networkv1alpha1.RouteNamespacesConfig{}
		}
		i.Spec.AllowedRoutes.Namespaces.Selector = selector
	}
}

func TestBuildAllowedRouteNamespaces(t *testing.T) {
	all := gatewayv1.NamespacesFromAll
	same := gatewayv1.NamespacesFromSame

	cases := []struct {
		name        string
		inst        *networkv1alpha1.WaypointInstance
		defaultFrom *gatewayv1.FromNamespaces
		wantFrom    gatewayv1.FromNamespaces
		wantSel     *metav1.LabelSelector
		wantErr     bool
	}{
		{
			name:        "nil allowedRoutes uses default",
			inst:        newInstance("main", "ns"),
			defaultFrom: &same,
			wantFrom:    gatewayv1.NamespacesFromSame,
			wantSel:     nil,
		},
		{
			name:        "nil allowedRoutes with Same default",
			inst:        newInstance("main", "ns"),
			defaultFrom: &same,
			wantFrom:    gatewayv1.NamespacesFromSame,
			wantSel:     nil,
		},
		{
			name:        "from All",
			inst:        newInstance("main", "ns", withAllowedRoutesFrom("All")),
			defaultFrom: &all,
			wantFrom:    gatewayv1.NamespacesFromAll,
			wantSel:     nil,
		},
		{
			name:        "from Same",
			inst:        newInstance("main", "ns", withAllowedRoutesFrom("Same")),
			defaultFrom: &all,
			wantFrom:    gatewayv1.NamespacesFromSame,
			wantSel:     nil,
		},
		{
			name:        "from Selector without selector field should error",
			inst:        newInstance("main", "ns", withAllowedRoutesFrom("Selector")),
			defaultFrom: &all,
			wantErr:     true,
		},
		{
			name: "from Selector with matchLabels",
			inst: newInstance("main", "ns",
				withAllowedRoutesFrom("Selector"),
				withAllowedRoutesSelector(&metav1.LabelSelector{
					MatchLabels: map[string]string{"team": "istio"},
				}),
			),
			defaultFrom: &all,
			wantFrom:    gatewayv1.NamespacesFromSelector,
			wantSel: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "istio"},
			},
		},
		{
			name: "from Selector with matchExpressions",
			inst: newInstance("main", "ns",
				withAllowedRoutesFrom("Selector"),
				withAllowedRoutesSelector(&metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod"}},
					},
				}),
			),
			defaultFrom: &all,
			wantFrom:    gatewayv1.NamespacesFromSelector,
			wantSel: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: metav1.LabelSelectorOpIn, Values: []string{"prod"}},
				},
			},
		},
		{
			name: "from Selector with both matchLabels and matchExpressions",
			inst: newInstance("main", "ns",
				withAllowedRoutesFrom("Selector"),
				withAllowedRoutesSelector(&metav1.LabelSelector{
					MatchLabels: map[string]string{"team": "istio"},
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "env", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"dev"}},
					},
				}),
			),
			defaultFrom: &all,
			wantFrom:    gatewayv1.NamespacesFromSelector,
			wantSel: &metav1.LabelSelector{
				MatchLabels: map[string]string{"team": "istio"},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"dev"}},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildAllowedRouteNamespaces(tc.inst, tc.defaultFrom)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("buildAllowedRouteNamespaces returned nil")
			}
			if got.From == nil {
				t.Fatal("From is nil")
			}
			if *got.From != tc.wantFrom {
				t.Errorf("From = %q, want %q", *got.From, tc.wantFrom)
			}

			switch {
			case tc.wantSel == nil && got.Selector != nil:
				t.Errorf("Selector = %+v, want nil", got.Selector)
			case tc.wantSel != nil && got.Selector == nil:
				t.Errorf("Selector = nil, want %+v", tc.wantSel)
			case tc.wantSel != nil && got.Selector != nil:
				if len(tc.wantSel.MatchLabels) != len(got.Selector.MatchLabels) {
					t.Errorf("MatchLabels len = %d, want %d", len(got.Selector.MatchLabels), len(tc.wantSel.MatchLabels))
				}
				for k, v := range tc.wantSel.MatchLabels {
					if got.Selector.MatchLabels[k] != v {
						t.Errorf("MatchLabels[%q] = %q, want %q", k, got.Selector.MatchLabels[k], v)
					}
				}
				if len(tc.wantSel.MatchExpressions) != len(got.Selector.MatchExpressions) {
					t.Errorf("MatchExpressions len = %d, want %d", len(got.Selector.MatchExpressions), len(tc.wantSel.MatchExpressions))
				}
				for i, expr := range tc.wantSel.MatchExpressions {
					gotExpr := got.Selector.MatchExpressions[i]
					if expr.Key != gotExpr.Key {
						t.Errorf("MatchExpressions[%d].Key = %q, want %q", i, gotExpr.Key, expr.Key)
					}
					if expr.Operator != gotExpr.Operator {
						t.Errorf("MatchExpressions[%d].Operator = %q, want %q", i, gotExpr.Operator, expr.Operator)
					}
					if len(expr.Values) != len(gotExpr.Values) {
						t.Errorf("MatchExpressions[%d].Values len = %d, want %d", i, len(gotExpr.Values), len(expr.Values))
					}
					for j, v := range expr.Values {
						if gotExpr.Values[j] != v {
							t.Errorf("MatchExpressions[%d].Values[%d] = %q, want %q", i, j, gotExpr.Values[j], v)
						}
					}
				}
			}
		})
	}
}
