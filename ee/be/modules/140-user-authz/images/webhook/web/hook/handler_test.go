/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"fmt"
	"io"
	"log"
	"regexp"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAuthorizeRequest(t *testing.T) {
	tc := []struct {
		Name         string
		Group        []string
		Attributes   WebhookResourceAttributes
		ResultStatus WebhookRequestStatus
		Namespaces   []runtime.Object
	}{
		{
			Name:  "Namespaced",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced Restricted",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name:  "Namespaced Restricted System Allowed",
			Group: []string{"system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced One Not Limited Group And One Restricted",
			Group: []string{"limited", "system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "not-allowed-by-default",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced Limited",
			Group: []string{"limited"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test-abc-def",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced Limited and denied namespace",
			Group: []string{"limited"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "any-other-namespace",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name:  "Namespaced Limited with unlimited namespace regex",
			Group: []string{"limited-with-unlimited-regex"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "any-other-namespace",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Namespaced limited with system",
			Group: []string{"limited-and-system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name:  "Cluster scoped. Group and version are empty, search in the v1 apiVersion. Allowed.",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "",
				Version:   "",
				Resource:  "namespaces",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Cluster scoped. Group and version are empty, search in the v1 apiVersion. Denied.",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "",
				Version:   "",
				Resource:  "services",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "ClusterScoped",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object2",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Cluster scoped. Without version. Version exists",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "",
				Resource:  "object2",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Cluster scoped. Without version. Version does not exists",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "not.exists",
				Version:   "",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "webhook: kubernetes api request error",
			},
		},
		{
			Name:  "ClusterScoped but namespaced",
			Group: []string{"normal"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "ClusterScoped but namespaced and all namespaces are allowed",
			Group: []string{"system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "ClusterScoped but namespaced and limited namespaces",
			Group: []string{"limited"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "ClusterScoped One Not Limited Group And One Restricted",
			Group: []string{"limited", "system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:         "Non Resource allowed",
			Group:        []string{"normal"},
			Attributes:   WebhookResourceAttributes{},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name:  "Limited with NamespaceSelectors and namespace doesn't exist",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "namespaces \"namespace-selector-test\" not found",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and labels match",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace-selector-test",
						Labels: map[string]string{
							"match":      "true",
							"expression": "allow",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and labels don't match",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace-selector-test",
						Labels: map[string]string{
							"match": "false",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, and matches limitNamespaces",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test-abc-def",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, and matches NamespaceSelectors",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "namespace-selector-test",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "namespace-selector-test",
						Labels: map[string]string{
							"match":      "true",
							"expression": "match",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces (system regex), wants d8-system namespace without AllowAccessToSystemNamespaces",
			Group: []string{"limited-with-system-regex", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "d8-system",
						Labels: map[string]string{},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system namespace without AllowAccessToSystemNamespaces",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "d8-system",
						Labels: map[string]string{},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system namespace without AllowAccessToSystemNamespaces but the namespace has the labels",
			Group: []string{"limited", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
						Labels: map[string]string{
							"match":      "true",
							"expression": "match",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system with AllowAccessToSystemNamespaces",
			Group: []string{"limited-and-system-allowed", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
						Labels: map[string]string{
							"match":      "true",
							"expression": "allow",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelectors and limitNamespaces, wants d8-system with AllowAccessToSystemNamespaces but labels don't match",
			Group: []string{"limited-and-system-allowed", "limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
						Labels: map[string]string{
							"match": "true",
						},
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "user has no access to the namespace",
			},
		},
		{
			Name:  "Limited with MatchAny NamespaceSelector and limitNamespaces, wants d8-system",
			Group: []string{"limited", "limited-with-match-any-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "d8-system",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with MatchAny NamespaceSelector and limitNamespaces, wants across all namespaces",
			Group: []string{"limited", "limited-with-match-any-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
		{
			Name:  "Limited with NamespaceSelector, wants across all namespaces",
			Group: []string{"limited-namespace-selector"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "Limited with NamespaceSelector and unlimited limitNamespaces, wants across all namespaces",
			Group: []string{"limited-namespace-selector", "limited-with-unlimited-regex"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster-scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:  "Limited with NamespaceSelector and unlimited limitNamespaces plus system, wants across all namespaces",
			Group: []string{"limited-namespace-selector", "limited-with-unlimited-regex-and-system-allowed"},
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			Namespaces: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "d8-system",
					},
				},
			},
			ResultStatus: WebhookRequestStatus{
				Denied: false,
				Reason: "",
			},
		},
	}

	for _, testCase := range tc {
		t.Run(testCase.Name, func(t *testing.T) {
			nsRegex, _ := regexp.Compile("^test-.*$")
			allRegex, _ := regexp.Compile("^.*$")
			systemRegex, _ := regexp.Compile("^d8-.*$")
			namespaceSelector := &NamespaceSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"match": "true",
					},
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "expression",
							Operator: "In",
							Values:   []string{"match", "allow"},
						},
					},
				},
			}
			namespaceSelectorMatchAny := &NamespaceSelector{
				MatchAny: true,
			}

			handler := &Handler{
				logger:     log.New(io.Discard, "", 0),
				kubeclient: fake.NewSimpleClientset(testCase.Namespaces...),
				cache: &dummyCache{
					data: map[string]map[string]bool{
						"test/v1": {
							"object1": true,
							"object2": false,
						},
						"v1": {
							"namespaces": false,
							"services":   true,
						},
					},
					preferredVersions: map[string]string{
						"test": "v1",
					},
				},
				directory: map[string]map[string]DirectoryEntry{
					"Group": {
						"normal": {
							NamespaceFiltersAbsent: true,
						},
						"system-allowed": {
							NamespaceFiltersAbsent:        true,
							AllowAccessToSystemNamespaces: true,
						},
						"limited": {
							LimitNamespaces: []*regexp.Regexp{nsRegex},
						},
						"limited-with-system-regex": {
							LimitNamespaces: []*regexp.Regexp{systemRegex},
						},
						"limited-with-unlimited-regex": {
							LimitNamespaces: []*regexp.Regexp{allRegex},
						},
						"limited-and-system-allowed": {
							LimitNamespaces:               []*regexp.Regexp{nsRegex},
							AllowAccessToSystemNamespaces: true,
						},
						"limited-with-unlimited-regex-and-system-allowed": {
							LimitNamespaces:               []*regexp.Regexp{allRegex},
							AllowAccessToSystemNamespaces: true,
						},
						"limited-namespace-selector": {
							NamespaceSelectors: []*NamespaceSelector{
								namespaceSelector,
							},
						},
						"limited-with-match-any-namespace-selector": {
							NamespaceSelectors: []*NamespaceSelector{
								namespaceSelectorMatchAny,
							},
						},
					},
				},
			}

			req := &WebhookRequest{
				Spec: WebhookResourceSpec{
					User:               "test",
					Group:              testCase.Group,
					ResourceAttributes: testCase.Attributes,
				},
				Status: WebhookRequestStatus{},
			}

			req = handler.authorizeRequest(req)
			if req.Status.Denied != testCase.ResultStatus.Denied {
				t.Errorf("denied: got %v | expected %v", req.Status.Denied, testCase.ResultStatus.Denied)
			}

			if req.Status.Reason != testCase.ResultStatus.Reason {
				t.Errorf("reason: got %q | expected %q", req.Status.Reason, testCase.ResultStatus.Reason)
			}
		})
	}
}

type dummyCache struct {
	data              map[string]map[string]bool
	preferredVersions map[string]string
}

func (d *dummyCache) Get(api, key string) (bool, error) {
	return d.data[api][key], nil
}

func (d *dummyCache) GetPreferredVersion(group string) (string, error) {
	if v, ok := d.preferredVersions[group]; ok {
		return v, nil
	}

	return "", fmt.Errorf("not found")
}

func (d *dummyCache) Check() error {
	return nil
}

func TestWrapRegexpTest(t *testing.T) {
	tc := []struct {
		Name   string
		Input  string
		Output string
	}{
		{
			Name:   "Wrap",
			Input:  ".*",
			Output: "^.*$",
		},
		{
			Name:   "Wrap tail",
			Input:  "^.*",
			Output: "^.*$",
		},
		{
			Name:   "Wrap head",
			Input:  ".*$",
			Output: "^.*$",
		},
		{
			Name:   "No wrap",
			Input:  "^.*$",
			Output: "^.*$",
		},
	}

	for _, testCase := range tc {
		t.Run(testCase.Name, func(t *testing.T) {
			res := wrapRegex(testCase.Input)
			if testCase.Output != res {
				t.Fatalf("got %q, expected %q", res, testCase.Output)
			}
		})
	}
}
