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
)

func TestAuthorizeRequest(t *testing.T) {
	tc := []struct {
		Name         string
		Group        []string
		Attributes   WebhookResourceAttributes
		ResultStatus WebhookRequestStatus
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
			ResultStatus: WebhookRequestStatus{},
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
				Reason: "making cluster scoped requests for namespaced resources are not allowed",
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
				Reason: "making cluster scoped requests for namespaced resources are not allowed",
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
				Reason: "making cluster scoped requests for namespaced resources are not allowed",
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
	}

	for _, testCase := range tc {
		t.Run(testCase.Name, func(t *testing.T) {
			nsRegex, _ := regexp.Compile("^test-.*$")
			allRegex, _ := regexp.Compile("^.*$")

			handler := &Handler{
				logger: log.New(io.Discard, "", 0),
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
							LimitNamespacesAbsent: true,
						},
						"system-allowed": {
							LimitNamespacesAbsent:         true,
							AllowAccessToSystemNamespaces: true,
						},
						"limited": {
							LimitNamespaces: []*regexp.Regexp{nsRegex},
						},
						"limited-with-unlimited-regex": {
							LimitNamespaces: []*regexp.Regexp{allRegex},
						},
						"limited-and-system-allowed": {
							LimitNamespaces:               []*regexp.Regexp{nsRegex},
							AllowAccessToSystemNamespaces: true,
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
