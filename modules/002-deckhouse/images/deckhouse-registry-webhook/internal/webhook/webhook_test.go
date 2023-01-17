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

package webhook

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
)

type FakeRegistryClient struct{}

func (r FakeRegistryClient) CheckImage(registry, image string, authCfg authn.AuthConfig) error {
	if authCfg.Username != "valid" {
		return fmt.Errorf("Auth failed")
	}

	return nil
}

func TestWebhook(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook")
}

const admisstionReviewJSONTemplate = `
{
  "kind": "AdmissionReview",
  "apiVersion": "admission.k8s.io/v1",
  "request": {
    "uid": "12345678-1234-1234-1234-123456789012",
    "name": "test",
    "namespace": "default",
    "operation": "CREATE",
    "object": {
      "kind": "Secret",
      "apiVersion": "v1",
      "metadata": {
        "name": "test",
        "namespace": "default",
        "uid": "69eb5e7f-eae6-4f42-af0a-f83fe36ee5c4",
        "managedFields": []
      },
      "data": {
        {{- if .DockerConfigB64 }}
        ".dockerconfigjson": "{{ .DockerConfigB64 }}",
        {{- end }}
        "scheme": "{{ .Scheme }}",
        "address": "{{ .Address }}",
        "path": "{{ .Path }}"
      },
      "type": "{{ .SecretType }}"
    },
    "options": {}
  }
}
`

type templateParams struct {
	SecretType       string
	DockerConfigJSON string
	DockerConfigB64  string
	Scheme           string
	Address          string
	Path             string
}

func AdmisstionJSON(params templateParams) string {
	var output bytes.Buffer
	if params.SecretType == "" {
		params.SecretType = "kubernetes.io/dockerconfigjson"
	}
	if params.DockerConfigB64 == "" {
		params.DockerConfigB64 = base64.StdEncoding.EncodeToString([]byte(params.DockerConfigJSON))
	}
	params.Scheme = base64.StdEncoding.EncodeToString([]byte(params.Scheme))
	params.Path = base64.StdEncoding.EncodeToString([]byte(params.Path))
	params.Address = base64.StdEncoding.EncodeToString([]byte(params.Address))
	t := template.Must(template.New("").Parse(admisstionReviewJSONTemplate))
	_ = t.Execute(&output, params)

	return output.String()
}

type wanted struct {
	BodySubstring    string
	StatusCode       int
	AdmissionAllowed bool
}

var _ = Describe("ValidatingWebhook", func() {
	Context("Test Webhook Run", func() {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, time.Second*1)
		defer cancel()
		r := FakeRegistryClient{}
		vw := NewValidatingWebhook(":36363", "test-tag", "", "", r)
		err := vw.Run(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("Test Webhook Handler", func() {
		r := FakeRegistryClient{}
		vw := NewValidatingWebhook(":36363", "test-tag", "", "", r)
		DescribeTable("",
			func(admissionReview string, want *wanted) {
				r := httptest.NewRequest(http.MethodPost, "/validate", strings.NewReader(admissionReview))
				w := httptest.NewRecorder()
				vw.ValidatingWebhook(w, r)
				resp := w.Result()
				body, err := io.ReadAll(resp.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(want.StatusCode))
				Expect(string(body)).To(ContainSubstring(want.BodySubstring))
				if resp.StatusCode == http.StatusOK {
					review := &admissionv1.AdmissionReview{}
					err := json.Unmarshal(body, review)
					Expect(err).NotTo(HaveOccurred())
					Expect(review.Response.UID).To(Equal(types.UID("12345678-1234-1234-1234-123456789012")))
					Expect(review.Response.Allowed).To(Equal(want.AdmissionAllowed))
				}
			},
			Entry("Invalid admission review",
				"{}",
				&wanted{
					BodySubstring: "bad admission review",
					StatusCode:    http.StatusBadRequest,
				}),
			Entry("Secret with wrong type",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: "",
					SecretType:       "Opaque",
					Address:          "registry.example.com",
					Path:             "/path",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "secret should be kubernetes.io/dockerconfigjson type",
					StatusCode:       http.StatusOK,
				}),
			Entry("Field .dockerconfigjson is missed in the secret",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: "",
					Address:          "registry.example.com",
					Path:             "/path",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "secret must contain the '.dockerconfigjson' field",
					StatusCode:       http.StatusOK,
				}),
			Entry("Bad .dockerconfigjson data",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{"aaa": "bbb"}`, // {"aaa":"bbb"}
					Address:          "registry.example.com",
					Path:             "/path",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "bad docker config",
					StatusCode:       http.StatusOK,
				}),
			Entry("Empty auths",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { } }`,
					Address:          "registry.example.com",
					Path:             "/path",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "bad docker config",
					StatusCode:       http.StatusOK,
				}),
			Entry("Valid Secret with invalid creds",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { "registry.example.com": { "auth": "aW52YWxpZDppbnZhbGlkCg==" } } }`, // invalid:invalid
					Address:          "registry.example.com",
					Path:             "/path",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "Auth failed",
					StatusCode:       http.StatusOK,
				}),
			Entry("Valid Secret with working creds",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { "registry.example.com": { "auth": "dmFsaWQ6dmFsaWQK" } } }`, // valid:valid
					Address:          "registry.example.com",
					Path:             "/path",
				}),
				&wanted{
					AdmissionAllowed: true,
					BodySubstring:    "",
					StatusCode:       http.StatusOK,
				}),
			Entry("Path field is empty",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { "registry.example.com": { "auth": "dmFsaWQ6dmFsaWQK" } } }`, // valid:valid
					Address:          "registry.example.com",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "secret must contain the 'path' field and it must be non-empty",
					StatusCode:       http.StatusOK,
				}),
			Entry("Address field is empty",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { "registry.example.com": { "auth": "dmFsaWQ6dmFsaWQK" } } }`, // valid:valid
					Path:             "/path",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "secret must contain the 'address' field and it must be non-empty",
					StatusCode:       http.StatusOK,
				}),
			Entry("Valid Secret with working creds + address + path",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { "registry.example.com": { "auth": "dmFsaWQ6dmFsaWQK" } } }`, // valid:valid
					Address:          "registry.example.com",
					Path:             "/sys/deckhouse-oss",
				}),
				&wanted{
					AdmissionAllowed: true,
					BodySubstring:    "",
					StatusCode:       http.StatusOK,
				}),
			Entry("Valid Secret with working creds + scheme + address + path",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { "registry.example.com": { "auth": "dmFsaWQ6dmFsaWQK" } } }`, // valid:valid
					Scheme:           "http",
					Address:          "registry.example.com",
					Path:             "/sys/deckhouse-oss",
				}),
				&wanted{
					AdmissionAllowed: true,
					BodySubstring:    "",
					StatusCode:       http.StatusOK,
				}),
			Entry("Address field with bad data",
				AdmisstionJSON(templateParams{
					DockerConfigJSON: `{ "auths": { "registry.example.com": { "auth": "dmFsaWQ6dmFsaWQK" } } }`, // valid:valid
					Scheme:           "http",
					Address:          "registry.example.com\n",
					Path:             "/sys/deckhouse-oss",
				}),
				&wanted{
					AdmissionAllowed: false,
					BodySubstring:    "invalid control character in URL",
					StatusCode:       http.StatusOK,
				}),
		)
	})
})
