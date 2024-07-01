/*
Copyright 2024 Flant JSC

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

package roles

import (
	"slices"
	"strings"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
)

// skipCheckWildcards is exclusion rules for wildcard verification
//
// the key is the file name
// value is an array of rule names that allow wildcards
//
// !!IMPORTANT NOTE!!: will be fixed by separated issues
var skipCheckWildcards = map[string][]string{
	// Confirmed excludes
	"admission-policy-engine/templates/rbac-for-us.yaml": {
		// Some resources are created dynamically from CR. See more details in the target file
		"d8:admission-policy-engine:gatekeeper",
	},

	// Have to be reviewed
	"deckhouse/templates/webhook-handler/rbac-for-us.yaml": {
		"d8:deckhouse:webhook-handler",
	},
	"cloud-provider-aws/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-aws:cloud-controller-manager",
	},
	"cloud-provider-azure/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-azure:cloud-controller-manager",
	},
	"cloud-provider-gcp/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-gcp:cloud-controller-manager",
	},
	"cloud-provider-yandex/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-yandex:cloud-controller-manager",
	},
	"local-path-provisioner/templates/rbac-for-us.yaml": {
		"d8:local-path-provisioner",
	},
	"istio/templates/kiali/rbac-for-us.yaml": {
		"d8:istio:kiali",
	},
	"istio/templates/operator/rbac-for-us.yaml": {
		"d8:istio:operator",
	},
	"user-authn/templates/dex/rbac-for-us.yaml": {
		"d8:user-authn:dex:crd",
		"dex",
	},
	"operator-prometheus/templates/rbac-for-us.yaml": {
		"d8:operator-prometheus",
	},
	"prometheus-metrics-adapter/templates/rbac-for-us.yaml": {
		"d8:prometheus-metrics-adapter:horizontal-pod-autoscaler-external-metrics",
	},
	"vertical-pod-autoscaler/templates/rbac-for-us.yaml": {
		"d8:vertical-pod-autoscaler:controllers-reader",
	},
	"ingress-nginx/templates/kruise/rbac-for-us.yaml": {
		"d8:ingress-nginx:kruise-role",
	},
	"cilium-hubble/templates/ui/rbac-for-us.yaml": {
		"d8:cilium-hubble:ui:reader",
	},
	"okmeter/templates/rbac-for-us.yaml": {
		"d8:okmeter",
	},
	"openvpn/templates/openvpn/rbac-for-us.yaml": {
		"openvpn",
	},
	"upmeter/templates/upmeter-agent/rbac-for-us.yaml": {
		"upmeter-agent",
		"d8:upmeter:upmeter-agent",
	},
	"upmeter/templates/upmeter/rbac-for-us.yaml": {
		"d8:upmeter:upmeter",
	},
	"documentation/templates/rbac-for-us.yaml": {
		"documentation:leases-edit",
	},
	"delivery/templates/argocd/application-controller/rbac-for-us.yaml": {
		"d8:delivery:argocd:application-controller",
	},
	"delivery/templates/argocd/server/rbac-for-us.yaml": {
		"d8:delivery:argocd:server",
	},
	"cloud-provider-openstack/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-openstack:cloud-controller-manager",
	},
	"cloud-provider-vcd/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-vcd:cloud-controller-manager",
	},
	"cloud-provider-vsphere/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-vsphere:cloud-controller-manager",
	},
	"cloud-provider-zvirt/templates/cloud-controller-manager/rbac-for-us.yaml": {
		"d8:cloud-provider-zvirt:cloud-controller-manager",
	},
}

// ObjectRolesWildcard is a linter for checking the presence
// of a wildcard in a Role and ClusterRole
func ObjectRolesWildcard(object storage.StoreObject) errors.LintRuleError {
	// check only `rbac-for-us.yaml` files
	if !strings.HasSuffix(object.ShortPath(), "rbac-for-us.yaml") {
		return errors.EmptyRuleError
	}

	// check Role and ClusterRole for wildcards
	objectKind := object.Unstructured.GetKind()
	switch objectKind {
	case "Role", "ClusterRole":
		return checkRoles(object)
	default:
		return errors.EmptyRuleError
	}
}

func checkRoles(object storage.StoreObject) errors.LintRuleError {
	// check rules for skip
	for path, rules := range skipCheckWildcards {
		if strings.EqualFold(object.Path, path) {
			if slices.Contains(rules, object.Unstructured.GetName()) {
				return errors.EmptyRuleError
			}
		}
	}

	converter := runtime.DefaultUnstructuredConverter

	role := new(rbac.Role)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), role)
	if err != nil {
		panic(err)
	}

	for _, rule := range role.Rules {
		var objs []string
		if slices.Contains(rule.APIGroups, "*") {
			objs = append(objs, "apiGroups")
		}
		if slices.Contains(rule.Resources, "*") {
			objs = append(objs, "resources")
		}
		if slices.Contains(rule.Verbs, "*") {
			objs = append(objs, "verbs")
		}
		if len(objs) > 0 {
			return errors.NewLintRuleError(
				"WILDCARD001",
				object.Identity(),
				object.Path,
				strings.Join(objs, ", ")+" contains a wildcards. Replace them with an explicit list of resources",
			)
		}
	}

	return errors.EmptyRuleError
}
