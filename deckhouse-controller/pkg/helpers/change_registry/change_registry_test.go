// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package changeregistry

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	klient "github.com/flant/kube-client/client"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/envconfig"
	kclient "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func TestUpdateDeployContainersImagesToNewRepo(t *testing.T) {
	type args struct {
		newRepo         string
		nameOpts        []name.Option
		remoteOpts      []remote.Option
		newDeckhouseTag string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Deckhouse CE registry (need internet access)",
			args: args{
				newRepo: "registry.deckhouse.io/deckhouse/ce",
			},
		},
		{
			name: "Deckhouse CE registry with new deckhouse tag (need internet access)",
			args: args{
				newRepo:         "registry.deckhouse.io/deckhouse/ce",
				newDeckhouseTag: "v1.45.7",
			},
		},
		{
			name: "non-existed repo",
			args: args{
				newRepo: "registry.test.com",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := name.NewRepository(tt.args.newRepo)
			if err != nil {
				t.Fatal(err)
			}

			deploy := testDeckhouseDeploy()
			if err := updateDeployContainersImagesToNewRepo(deploy, repo, tt.args.nameOpts, tt.args.remoteOpts, tt.args.newDeckhouseTag); (err != nil) != tt.wantErr {
				t.Fatalf("updateDeployContainersImagesToNewRepo() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := checkContainersHost(deploy.Spec.Template.Spec.Containers, tt.args.newRepo, tt.args.newDeckhouseTag); (err != nil) != tt.wantErr {
				t.Error(err)
			}

			if err := checkContainersHost(deploy.Spec.Template.Spec.Containers, tt.args.newRepo, tt.args.newDeckhouseTag); (err != nil) != tt.wantErr {
				t.Error(err)
			}
		})
	}
}

func checkContainersHost(containers []v1.Container, host string, deckhouseTag string) error {
	for _, container := range containers {
		ref, err := name.ParseReference(container.Image)
		if err != nil {
			return err
		}
		if ref.Context().Name() != host {
			return fmt.Errorf("image not changed '%s' to host '%s'", ref.Name(), host)
		}

		if deckhouseTag != "" && ref.Identifier() != deckhouseTag {
			return fmt.Errorf("deckhouse tag not changed for image '%s': %s", ref.Name(), deckhouseTag)
		}
	}
	return nil
}

func TestNewImagePullSecretData(t *testing.T) {
	type args struct {
		authConfig authn.AuthConfig
	}
	tests := []struct {
		name      string
		newRepo   string
		caContent string
		scheme    string
		args      args
		want      map[string]string
		wantErr   bool
	}{
		{
			name:    "http anonymous registry",
			args:    args{},
			newRepo: "registry.example.com/deckhouse",
			scheme:  "http",
			want: map[string]string{
				".dockerconfigjson": `{"auths":{"registry.example.com":{}}}`,
				"address":           "registry.example.com",
				"path":              "/deckhouse",
				"scheme":            "http",
			},
		},
		{
			name:    "https anonymous registry",
			args:    args{},
			newRepo: "registry.example.com/deckhouse",
			scheme:  "https",
			want: map[string]string{
				".dockerconfigjson": `{"auths":{"registry.example.com":{}}}`,
				"address":           "registry.example.com",
				"path":              "/deckhouse",
				"scheme":            "https",
			},
		},
		{
			name:    "http local registry",
			args:    args{},
			newRepo: "my.registry.local/deckhouse",
			want: map[string]string{
				".dockerconfigjson": `{"auths":{"my.registry.local":{}}}`,
				"address":           "my.registry.local",
				"path":              "/deckhouse",
				"scheme":            "http",
			},
		},
		{
			name:    "https local registry",
			args:    args{},
			newRepo: "my.registry.local/deckhouse",
			scheme:  "https",
			want: map[string]string{
				".dockerconfigjson": `{"auths":{"my.registry.local":{}}}`,
				"address":           "my.registry.local",
				"path":              "/deckhouse",
				"scheme":            "https",
			},
		},
		{
			name: "https user+password registry and CA",
			args: args{
				authConfig: authn.AuthConfig{
					Username: "test",
					Password: "test",
				},
			},
			newRepo:   "registry.example.com/deckhouse",
			caContent: testCaContent,
			want: map[string]string{
				".dockerconfigjson": `{"auths":{"registry.example.com":{"username":"test","password":"test","auth":"dGVzdDp0ZXN0"}}}`,
				"address":           "registry.example.com",
				"path":              "/deckhouse",
				"scheme":            "https",
				"ca":                testCaContent,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRepo, err := name.NewRepository(tt.newRepo)
			if err != nil {
				t.Fatal(err)
			}

			got, err := newImagePullSecretData(newRepo, tt.args.authConfig, tt.caContent, tt.scheme)
			if (err != nil) != tt.wantErr {
				t.Errorf("newImagePullSecretData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newImagePullSecretData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckResponseForBearerSupport(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool

		headerValue string
		scheme      string
		host        string
	}{
		{
			name:        "normal response",
			headerValue: `Bearer realm="https://auth.example.com/token",service="registry.example.com",other=fun,slashed="he\"\l\lo"`,
			scheme:      "https",
			host:        "registry.example.com",
		},
		{
			name:        "anonymous response",
			headerValue: "anonymous",
			scheme:      "https",
			host:        "registry.example.com",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(fmt.Sprintf("%s://%s/v2/", tt.scheme, tt.host))
			if err != nil {
				t.Fatal(err)
			}

			reader, _ := io.Pipe()
			resp := &http.Response{
				Request:    &http.Request{},
				Header:     make(http.Header),
				StatusCode: http.StatusUnauthorized,
				Body:       reader,
			}
			resp.Header.Add("WWW-Authenticate", tt.headerValue)
			resp.Request.URL = u

			if err := checkResponseForAuthSupport(resp, tt.host); (err != nil) != tt.wantErr {
				t.Errorf("checkResponseForBearerSupport() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAuthHeaderWithBearer(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		wantFalse   bool
	}{
		{
			name:        "full header",
			headerValue: `Bearer realm="https://auth.example.com/token",service="registry.example.com",other=fun,slashed="he\"\l\lo"`,
		},
		{
			name:        "short header",
			headerValue: "Bearer;",
		},
		{
			name:        "no bearer",
			headerValue: "anonymous;",
			wantFalse:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("WWW-Authenticate", tt.headerValue)
			if !authHeader(header) && !tt.wantFalse {
				t.Error("Unexpected scheme in auth header: must be 'bearer'")
			}
		})
	}
}

func TestAuthHeaderWithBasic(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		wantFalse   bool
	}{
		{
			name:        "full header",
			headerValue: `BASIC realm="Sonatype Nexus Repository Manager"`,
		},
		{
			name:        "short header",
			headerValue: "BASIC;",
		},
		{
			name:        "no auth",
			headerValue: "anonymous;",
			wantFalse:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("WWW-Authenticate", tt.headerValue)
			if !authHeader(header) && !tt.wantFalse {
				t.Error("Unexpected scheme in auth header: must be 'basic'")
			}
		})
	}
}

func testDeckhouseDeploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					InitContainers: []v1.Container{
						{
							Name:  "init-downloaded-modules",
							Image: "registry.example.com@sha256:79ed551f4d0ec60799a9bd67f35441df6d86443515dd8337284fb68d97a01b3d",
						},
					},
					Containers: []v1.Container{
						{
							Name:  "deckhouse",
							Image: "registry.example.com:v1.46.8",
						},
					},
				},
			},
		},
	}
}

func Test_checkBearerSupport(t *testing.T) {
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name         string
		registryHost string
		scheme       string
		args         args
		wantErr      bool
	}{
		{
			name:         "check deckhouse CE (need internet access)",
			registryHost: "registry.deckhouse.io",
			args: args{
				ctx: context.Background(),
			},
		},

		{
			name:         "check non existed registry",
			registryHost: "registry.example.com",
			args: args{
				ctx: context.Background(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := name.NewRegistry(tt.registryHost)
			if err != nil {
				t.Fatal(err)
			}

			if err := checkAuthSupport(tt.args.ctx, reg, http.DefaultTransport); (err != nil) != tt.wantErr {
				t.Errorf("checkBearerSupport() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getCAContent(t *testing.T) {
	type args struct {
		caFile string
	}
	tests := []struct {
		name      string
		args      args
		want      string
		wantErr   bool
		caContent string
	}{
		{
			name: "read ca file",
			args: args{
				caFile: filepath.Join(t.TempDir(), "ca.crt"),
			},
			want:      strings.TrimSpace(testCaContent),
			caContent: testCaContent,
		},
		{
			name: "no ca file",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.caFile != "" && len(tt.caContent) > 0 {
				if err := os.WriteFile(tt.args.caFile, []byte(tt.caContent), 0755); err != nil {
					t.Fatal(err)
				}
			}

			got, err := getCAContent(tt.args.caFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("getCAContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getCAContent() = %v, want %v", got, tt.want)
				return
			}
		})
	}
}

func Test_checkPullPathNotManaged(t *testing.T) {
	t.Helper()

	toUnstructured := func(resourceYAML string) *unstructured.Unstructured {
		obj := &unstructured.Unstructured{}
		err := yaml.Unmarshal([]byte(resourceYAML), obj)
		require.NoError(t, err)
		return obj
	}

	enabledModule := toUnstructured(fmt.Sprintf(`
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  name: registry
status:
  conditions:
  - type: %q
    status: "True"
`, deckhousev1alpha1.ModuleConditionEnabledByModuleManager))

	registryConfig := func(mode string) *unstructured.Unstructured {
		return toUnstructured(fmt.Sprintf(`
apiVersion: deckhouse.io/v1alpha1
kind: RegistryConfig
metadata:
  name: registry
spec:
  mode: %q
`, mode))
	}

	imageAddress := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      registry_const.ImageAddressConfigMapName,
			Namespace: "d8-system",
		},
		Data: map[string]string{registry_const.ImageAddressConfigMapKey: registry_const.HostWithPath},
	}

	tests := []struct {
		name           string
		module         *unstructured.Unstructured
		registryConfig *unstructured.Unstructured
		imageAddress   *v1.ConfigMap
		wantRefused    bool
	}{
		{
			name: "Module disabled (managed Kubernetes)",
		},
		{
			name:   "Module enabled, Unmanaged: nothing rendered",
			module: enabledModule,
		},
		{
			name:           "RegistryConfig left in the Unmanaged mode still overrides the secret",
			module:         enabledModule,
			registryConfig: registryConfig("Unmanaged"),
			wantRefused:    true,
		},
		{
			name:           "Managed",
			module:         enabledModule,
			registryConfig: registryConfig("Managed"),
			imageAddress:   imageAddress,
			wantRefused:    true,
		},
		{
			name:           "Managed, the agents have not applied the layout yet",
			module:         enabledModule,
			registryConfig: registryConfig("Managed"),
			wantRefused:    true,
		},
		{
			name:           "RegistryConfig without a mode",
			module:         enabledModule,
			registryConfig: registryConfig(""),
			wantRefused:    true,
		},
		{
			name:         "Image address published without a RegistryConfig (installer)",
			module:       enabledModule,
			imageAddress: imageAddress,
			wantRefused:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			kubeCl := kclient.NewFakeKubernetesClient()

			if tt.module != nil {
				_, err := kubeCl.
					Dynamic().
					Resource(deckhousev1alpha1.ModuleGVR).
					Namespace("").
					Create(ctx, tt.module, metav1.CreateOptions{})
				require.NoError(t, err, "failed to create test module")
			}

			if tt.registryConfig != nil {
				_, err := kubeCl.
					Dynamic().
					Resource(registryConfigGVR).
					Create(ctx, tt.registryConfig, metav1.CreateOptions{})
				require.NoError(t, err, "failed to create test registryconfig")
			}

			if tt.imageAddress != nil {
				_, err := kubeCl.KubeClient.CoreV1().
					ConfigMaps(tt.imageAddress.Namespace).
					Create(ctx, tt.imageAddress, metav1.CreateOptions{})
				require.NoError(t, err, "failed to create test configmap")
			}

			err := checkPullPathNotManaged(ctx, kubeCl)
			if !tt.wantRefused {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), "moduleConfig/registry")
			assert.Contains(t, err.Error(), "rollout restart deploy/deckhouse")
			assert.Contains(t, err.Error(), "storage.source")
			assert.Contains(t, err.Error(), "'registryconfig/registry' is gone")
			assert.NotContains(t, err.Error(), "moduleConfig/deckhouse")
			// 'd8 k exec' rewrites a quoted d8 command on stderr into a 'd8 k ...' one that does not exist.
			assert.Contains(t, err.Error(), "mirror push command of the Deckhouse CLI")
			assert.NotContains(t, err.Error(), "'d8 ")
		})
	}
}

// Test_checkPullPathNotManaged_APIError checks that the helper fails closed: when the API server
// does not answer whether the module manages the pull path, the helper refuses rather than edits the
// secret. Only a missing object or a missing kind means the module is not there.
func Test_checkPullPathNotManaged_APIError(t *testing.T) {
	tests := []struct {
		name            string
		configMapErr    error
		registryConfErr error
		wantRefused     bool
	}{
		{
			name:         "ConfigMap get times out",
			configMapErr: k8serrors.NewTimeoutError("the server was unable to return a response", 1),
			wantRefused:  true,
		},
		{
			name:         "ConfigMap get is forbidden",
			configMapErr: k8serrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, registry_const.ImageAddressConfigMapName, errors.New("denied")),
			wantRefused:  true,
		},
		{
			name:            "RegistryConfig get fails with an internal error",
			registryConfErr: k8serrors.NewInternalError(errors.New("etcdserver: request timed out")),
			wantRefused:     true,
		},
		{
			name:            "RegistryConfig get is throttled",
			registryConfErr: k8serrors.NewTooManyRequests("slow down", 1),
			wantRefused:     true,
		},
		{
			name:            "RegistryConfig not found",
			registryConfErr: k8serrors.NewNotFound(registryConfigGVR.GroupResource(), registryConfigName),
		},
		{
			name:            "No RegistryConfig kind on a cluster without the module",
			registryConfErr: &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: registryConfigGVR.Group, Kind: "RegistryConfig"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kubeCl := kclient.NewFakeKubernetesClient()

			if tt.configMapErr != nil {
				clientset, ok := kubeCl.KubeClient.(*klient.Client).Interface.(*k8sfake.Clientset)
				require.True(t, ok, "fake kube client is not backed by a fake clientset")
				clientset.PrependReactor("get", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, tt.configMapErr
				})
			}

			if tt.registryConfErr != nil {
				dyn, ok := kubeCl.Dynamic().(*dynamicfake.FakeDynamicClient)
				require.True(t, ok, "fake kube client is not backed by a fake dynamic client")
				dyn.PrependReactor("get", registryConfigGVR.Resource, func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, tt.registryConfErr
				})
			}

			err := checkPullPathNotManaged(context.Background(), kubeCl)
			if !tt.wantRefused {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to check if the \"registry\" module manages the image pull path")
		})
	}
}

func Test_kubeInitParams(t *testing.T) {
	t.Run("in-cluster by default", func(t *testing.T) {
		t.Setenv(envconfig.EnvKubeConfigInCluster, "")

		params := kubeInitParams()
		assert.True(t, params.KubeConfigInCluster)
		assert.Empty(t, params.KubeConfig)
		assert.Nil(t, params.RestConfig)
	})

	t.Run("in-cluster turned off", func(t *testing.T) {
		t.Setenv(envconfig.EnvKubeConfigInCluster, "false")

		assert.False(t, kubeInitParams().KubeConfigInCluster)
	})
}

// Test_newKubeClient checks that the client goes to the API server the environment names, over TLS
// with its token, and not to http://localhost:6445 as it did with empty kube options. The pod's
// service account files sit at a fixed path, so KUBECONFIG stands in for them here: the kube client
// tries it first and falls back to the service account when it is absent.
func Test_newKubeClient(t *testing.T) {
	const token = "dh-test-secret-0123456789abcdef"

	var gotPath, gotAuth string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"` +
			registry_const.ImageAddressConfigMapName + `","namespace":"d8-system"}}`))
	}))
	defer srv.Close()

	caData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	kubeconfig := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: %s
    certificate-authority-data: %s
users:
- name: test
  user:
    token: %s
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
`, srv.URL, base64.StdEncoding.EncodeToString(caData), token)

	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0o600))

	t.Setenv("KUBECONFIG", kubeconfigPath)
	t.Setenv(envconfig.EnvKubeConfigInCluster, "")
	t.Setenv("FLANT_KUBE_CLIENT_IN_MEMORY_DISCOVERY_CACHE", "true")

	kubeCl, err := newKubeClient()
	require.NoError(t, err)

	managed, err := pullPathManaged(context.Background(), kubeCl)
	require.NoError(t, err)
	assert.True(t, managed)
	assert.Equal(t, "/api/v1/namespaces/d8-system/configmaps/"+registry_const.ImageAddressConfigMapName, gotPath)
	assert.Equal(t, "Bearer "+token, gotAuth)
}

const (
	testCaContent = `
-----BEGIN CERTIFICATE-----
MIIDqjCCApICCQC5+/3MLrlWRzANBgkqhkiG9w0BAQsFADCBljELMAkGA1UEBhMC
TUgxEjAQBgNVBAgMCURlY2tob3VzZTESMBAGA1UEBwwJRGVja2hvdXNlMRIwEAYD
VQQKDAlEZWNraG91c2UxEjAQBgNVBAsMCURlY2tob3VzZTESMBAGA1UEAwwJRGVj
a2hvdXNlMSMwIQYJKoZIhvcNAQkBFhRjb250YWN0QGRlY2tob3VzZS5pbzAeFw0y
MzA2MTYxMjQwMjNaFw0yODA2MTQxMjQwMjNaMIGWMQswCQYDVQQGEwJNSDESMBAG
A1UECAwJRGVja2hvdXNlMRIwEAYDVQQHDAlEZWNraG91c2UxEjAQBgNVBAoMCURl
Y2tob3VzZTESMBAGA1UECwwJRGVja2hvdXNlMRIwEAYDVQQDDAlEZWNraG91c2Ux
IzAhBgkqhkiG9w0BCQEWFGNvbnRhY3RAZGVja2hvdXNlLmlvMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvelLA8XRx8LzSPFcqvoV/M68fnhrcykASl/a
MaRVoKn9Ms/vNvPzkW8X2ushD3tuWJw1JHwb3vUw2eK2K+LdSu/swLodwOf/tMp7
JX7NuhI0XLFMEpoSfAhbGdyYUaSHEbJkOCF9ZXlp7dtW2dEYefJn4a8+ZXCSQJYP
TMj08IrztzU9jnWi6nwupB+ItYvMhjNT8tlnCXaZebQMXKpBuH7F0acnojCnOQmT
qZCvh7bl2EbK4zM9Q0iUSx4MYnC+mQ3x0l525toetmGMqwV1GlTLGD9/t5Cc4q3l
6mDbkytuKsZeZpkEgmOtdjoESkdAepDiBej1eQvS0i0AiyAFBwIDAQABMA0GCSqG
SIb3DQEBCwUAA4IBAQAo8oAwmz5wyxljxXqOoWLMlit7MVU/jfUwFCFFCK+pqI2V
/kQBBH5ZJRZ0AF3k4cuA+vJc+Cwlu25c5KJrl+CgDQQ+pdrHqbw+hLnkRsA6a9kn
5UBpLuOj2ALuYvxsGVp2DvxVkpKGU2fcbtPbQFY3n7yK1SW64nTCk5dS30gU61pU
SdpwTLz+GMV14jWRh+TQWO135tFZSuuUwPWzx6k68raQVxPi2fFu949BT5gl1L2Y
e4f/w8EYFkBiGlZo2RguL3fFMouOo65CPxYj2jA1Y9D2AUG0L/3+CIe+RKCn4HML
oFmKNcjIX7fCuW8fGd+DwdUQ9cR8JV9si/gjdZzu
-----END CERTIFICATE-----
`
)
