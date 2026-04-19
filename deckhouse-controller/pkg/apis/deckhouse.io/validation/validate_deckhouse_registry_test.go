/*
Copyright 2025 Flant JSC

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

package validation_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/validation"
)

func createSecret(namespace, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func createAdmissionRev(obj interface{}) *admissionv1.AdmissionReview {
	raw, _ := json.Marshal(obj)
	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:    "test-uid",
			Object: runtime.RawExtension{Raw: raw},
		},
	}
}

func TestRegistrySecretHandler(t *testing.T) {
	validDockerCfg := map[string]interface{}{
		"auths": map[string]interface{}{
			"registry.example.com": map[string]string{
				"auth": base64.StdEncoding.EncodeToString([]byte("user:pass")),
			},
		},
	}
	validDockerCfgJSON, _ := json.Marshal(validDockerCfg)

	tests := []struct {
		name        string
		secret      *corev1.Secret
		wantAllowed bool
		wantMessage string
	}{
		{
			name: "irrelevant secret allowed",
			secret: createSecret("default", "other-secret", map[string][]byte{
				"some": []byte("data"),
			}),
			wantAllowed: true,
		},
		{
			name: "missing required field",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("my.registry"),
			}),
			wantAllowed: false,
			wantMessage: "Field 'path' is required",
		},
		{
			name: "empty address rejected",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address":           []byte("  "),
				"path":              []byte("repo"),
				".dockerconfigjson": validDockerCfgJSON,
			}),
			wantAllowed: false,
			wantMessage: "Field 'address' cannot be empty",
		},
		{
			name: "address contains whitespace rejected",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address":           []byte("registry with space"),
				"path":              []byte("repo"),
				".dockerconfigjson": validDockerCfgJSON,
			}),
			wantAllowed: false,
			wantMessage: "Field 'address' contains spaces",
		},
		{
			name: "invalid .dockerconfigjson JSON rejected",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address":           []byte("registry"),
				"path":              []byte("repo"),
				".dockerconfigjson": []byte("{not-json}"),
			}),
			wantAllowed: false,
			wantMessage: "not valid JSON",
		},
		{
			name: "empty auths rejected",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("registry"),
				"path":    []byte("repo"),
				".dockerconfigjson": []byte(`{
					"auths": {}
				}`),
			}),
			wantAllowed: false,
			wantMessage: "must contain at least one registry",
		},
		{
			name: "registry key with space rejected",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("registry"),
				"path":    []byte("repo"),
				".dockerconfigjson": []byte(`{
					"auths": {
						"bad registry": { "auth": "dXNlcjpwYXNz" }
					}
				}`),
			}),
			wantAllowed: false,
			wantMessage: "contains spaces",
		},
		{
			name: "invalid base64 auth rejected",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("registry"),
				"path":    []byte("repo"),
				".dockerconfigjson": []byte(`{
					"auths": {
						"registry.example.com": { "auth": "!!!!" }
					}
				}`),
			}),
			wantAllowed: false,
			wantMessage: "not valid base64",
		},
		{
			name: "invalid auth format rejected",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("registry"),
				"path":    []byte("repo"),
				".dockerconfigjson": []byte(`{
					"auths": {
						"registry.example.com": { "auth": "` + base64.StdEncoding.EncodeToString([]byte("onlyuser")) + `" }
					}
				}`),
			}),
			wantAllowed: false,
			wantMessage: "must be in format login:password",
		},
		{
			name: "empty login allowed",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("registry"),
				"path":    []byte("repo"),
				".dockerconfigjson": []byte(`{
					"auths": {
						"registry.example.com": { "auth": "` + base64.StdEncoding.EncodeToString([]byte(":password")) + `" }
					}
				}`),
			}),
			wantAllowed: true,
		},
		{
			name: "empty password allowed",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("registry"),
				"path":    []byte("repo"),
				".dockerconfigjson": []byte(`{
					"auths": {
						"registry.example.com": { "auth": "` + base64.StdEncoding.EncodeToString([]byte("login:")) + `" }
					}
				}`),
			}),
			wantAllowed: true,
		},
		{
			name: "empty password and login are allowed",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address": []byte("registry"),
				"path":    []byte("repo"),
				".dockerconfigjson": []byte(`{
					"auths": {
						"registry.example.com": { "auth": "` + base64.StdEncoding.EncodeToString([]byte(":")) + `" }
					}
				}`),
			}),
			wantAllowed: true,
		},
		{
			name: "valid secret passes",
			secret: createSecret("d8-system", "deckhouse-registry", map[string][]byte{
				"address":           []byte("registry.example.com"),
				"path":              []byte("repo"),
				".dockerconfigjson": validDockerCfgJSON,
			}),
			wantAllowed: true,
		},
	}

	handler := validation.RegistrySecretHandler()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ar := createAdmissionRev(tt.secret)
			body, err := json.Marshal(ar)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			var resp admissionv1.AdmissionReview
			err = json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			require.NotNil(t, resp.Response)

			if tt.wantAllowed {
				assert.True(t, resp.Response.Allowed)
			} else {
				assert.False(t, resp.Response.Allowed)
				assert.Contains(t, resp.Response.Result.Message, tt.wantMessage)
			}
		})
	}
}
