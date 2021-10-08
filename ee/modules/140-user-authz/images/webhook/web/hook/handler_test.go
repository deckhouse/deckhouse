/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"log"
	"regexp"
	"testing"
)

func TestAuthorizeRequest(t *testing.T) {
	tc := []struct {
		Name         string
		User         string
		Attributes   WebhookResourceAttributes
		ResultStatus WebhookRequestStatus
	}{
		{
			Name: "Namespaced",
			User: "normal",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "Namespaced Restricted",
			User: "normal",
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
			Name: "Namespaced Restricted Allowed",
			User: "system-allowed",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "Namespaced Limited",
			User: "limited",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "test-abc-def",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "Namespaced Limited and denied namespace",
			User: "limited",
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
			Name: "Namespaced Limited with unlimited namespace regex",
			User: "limited-with-unlimited-regex",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "any-other-namespace",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "Namespaced limited with system",
			User: "limited-and-system-allowed",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "default",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "ClusterScoped",
			User: "normal",
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
			Name: "ClusterScoped but namespaced",
			User: "normal",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name: "ClusterScoped but namespaced and all namespaces are allowed",
			User: "system-allowed",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{},
		},
		{
			Name: "ClusterScoped but namespaced and limited namespaces",
			User: "limited",
			Attributes: WebhookResourceAttributes{
				Group:     "test",
				Version:   "v1",
				Resource:  "object1",
				Namespace: "",
			},
			ResultStatus: WebhookRequestStatus{
				Denied: true,
				Reason: "making cluster scoped requests for namespaced resources is not allowed",
			},
		},
		{
			Name:         "Non Resource allowed",
			User:         "normal",
			Attributes:   WebhookResourceAttributes{},
			ResultStatus: WebhookRequestStatus{},
		},
	}

	for _, testCase := range tc {
		t.Run(testCase.Name, func(t *testing.T) {
			nsRegex, _ := regexp.Compile("test-.*")
			allRegex, _ := regexp.Compile(".*")

			handler := &Handler{
				logger: &log.Logger{},
				cache: &dummyCache{
					data: map[string]map[string]bool{
						"test/v1": {
							"object1": true,
							"object2": false,
						},
					},
				},
				directory: map[string]map[string]DirectoryEntry{
					"User": {
						"normal": {},
						"system-allowed": {
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
					User:               testCase.User,
					ResourceAttributes: testCase.Attributes,
				},
				Status: WebhookRequestStatus{},
			}

			req = handler.authorizeRequest(req)
			if req.Status.Denied != testCase.ResultStatus.Denied {
				t.Fatalf("denied: got %v | expected %v", req.Status.Denied, testCase.ResultStatus.Denied)
			}

			if req.Status.Reason != testCase.ResultStatus.Reason {
				t.Fatalf("reason: got %v | expected %v", req.Status.Reason, testCase.ResultStatus.Reason)
			}
		})
	}
}

type dummyCache struct {
	data map[string]map[string]bool
}

func (d *dummyCache) Get(api, key string) (bool, error) {
	return d.data[api][key], nil
}
