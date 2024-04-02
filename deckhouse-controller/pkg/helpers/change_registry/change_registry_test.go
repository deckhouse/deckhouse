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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
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
		insecure  bool
		args      args
		want      map[string]string
		wantErr   bool
	}{
		{
			name:     "http anonymous registry",
			args:     args{},
			newRepo:  "registry.example.com/deckhouse",
			insecure: true,
			want: map[string]string{
				".dockerconfigjson": `{"auths":{"registry.example.com":{}}}`,
				"address":           "registry.example.com",
				"path":              "/deckhouse",
				"scheme":            "http",
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
			var opts []name.Option
			if tt.insecure {
				opts = append(opts, name.Insecure)
			}

			newRepo, err := name.NewRepository(tt.newRepo, opts...)
			if err != nil {
				t.Fatal(err)
			}

			got, err := newImagePullSecretData(newRepo, tt.args.authConfig, tt.caContent)
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

			if err := checkResponseForBearerSupport(resp, tt.host); (err != nil) != tt.wantErr {
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
			if !authHeaderWithBearer(header) && !tt.wantFalse {
				t.Error("Unexpected scheme in auth header: must be 'bearer'")
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
							Name:  "init-external-modules",
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
		insecure     bool
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
			insecure:     true,
			args: args{
				ctx: context.Background(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []name.Option
			if tt.insecure {
				opts = append(opts, name.Insecure)
			}

			reg, err := name.NewRegistry(tt.registryHost, opts...)
			if err != nil {
				t.Fatal(err)
			}

			if err := checkBearerSupport(tt.args.ctx, reg, http.DefaultTransport); (err != nil) != tt.wantErr {
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
