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
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/hashicorp/go-multierror"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	kclient "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
)

// TODO (alex123012): Use new methods in transport package for getting bearer token:
// in the PR https://github.com/google/go-containerregistry/pull/1709
// some methods became public and we can use them except of copying the source code.
// (waiting for new "go-containerregistry" version)

const (
	d8SystemNS = "d8-system"
	caKey      = "ca"
)

func ChangeRegistry(newRegistry, username, password, caFile, newDeckhouseImageTag, scheme string, dryRun bool) error {
	ctx := context.Background()
	logEntry := log.WithField("operator.component", "ChangeRegistry")

	authConfig := newAuthConfig(username, password)

	caContent, err := getCAContent(caFile)
	if err != nil {
		return err
	}

	nameOpts := newNameOptions(scheme)
	newRepo, err := name.NewRepository(newRegistry, nameOpts...)
	if err != nil {
		return err
	}

	caTransport := cr.GetHTTPTransport(caContent)

	if err := checkBearerSupport(ctx, newRepo.Registry, caTransport); err != nil {
		return err
	}

	kubeCl, err := newKubeClient()
	if err != nil {
		return err
	}

	logEntry.Println("Retrieving deckhouse deployment...")
	deckhouseDeploy, err := deckhouseDeployment(ctx, kubeCl)
	if err != nil {
		return err
	}

	remoteOpts, err := newRemoteOptions(ctx, newRepo, authConfig, caTransport)
	if err != nil {
		return err
	}

	// Check that all images for deckhouse deploy exist in the new repo before
	// updating image pull secret and prepare deployment with updated images.
	if err := updateDeployContainersImagesToNewRepo(deckhouseDeploy, newRepo, nameOpts, remoteOpts, newDeckhouseImageTag); err != nil {
		return err
	}

	imagePullSecretData, err := newImagePullSecretData(newRepo, authConfig, caContent, scheme)
	if err != nil {
		return err
	}

	deckhouseSecret, err := modifyPullSecret(ctx, kubeCl, imagePullSecretData)
	if err != nil {
		return err
	}

	if dryRun {
		logEntry.Println("Dry-run enabled")
		secretYaml, _ := yaml.Marshal(deckhouseSecret)
		deploymentYaml, _ := yaml.Marshal(deckhouseDeploy)
		logEntry.Println("------------------------------")
		logEntry.Printf("New Secret will be applied:\n%s\n", secretYaml)
		logEntry.Println("------------------------------")
		logEntry.Printf("New Deployment will be applied:\n%s\n", deploymentYaml)
	} else {
		logEntry.Println("Updating deckhouse image pull secret...")
		if err := updateImagePullSecret(ctx, kubeCl, deckhouseSecret); err != nil {
			return err
		}

		logEntry.Println("Updating deckhouse deployment...")
		if err := updateDeployment(ctx, kubeCl, deckhouseDeploy); err != nil {
			return err
		}
	}

	logEntry.Println("Done")
	return nil
}

func newAuthConfig(username, password string) authn.AuthConfig {
	return authn.AuthConfig{
		Username: username,
		Password: password,
	}
}

func newRemoteOptions(ctx context.Context, repo name.Repository, authConfig authn.AuthConfig, caTransport http.RoundTripper) ([]remote.Option, error) {
	t, err := newTransport(ctx, repo, authConfig, caTransport)
	if err != nil {
		return nil, err
	}

	return []remote.Option{
		remote.WithTransport(t),
	}, nil
}

func newNameOptions(scheme string) []name.Option {
	opts := []name.Option{name.StrictValidation}
	if scheme == "http" {
		opts = append(opts, name.Insecure)
	}
	return opts
}

func newTransport(ctx context.Context, repo name.Repository, authConfig authn.AuthConfig, caTransport http.RoundTripper) (http.RoundTripper, error) {
	authorizer := authn.FromConfig(authConfig)

	scopes := []string{repo.Scope(transport.PullScope)}
	return transport.NewWithContext(ctx, repo.Registry, authorizer, caTransport, scopes)
}

func newKubeClient() (*kclient.KubernetesClient, error) {
	kubeCl := kclient.NewKubernetesClient()
	if err := kubeCl.Init(kclient.AppKubernetesInitParams()); err != nil {
		return nil, err
	}
	return kubeCl, nil
}

func modifyPullSecret(ctx context.Context, kubeCl *kclient.KubernetesClient, newSecretData map[string]string) (*v1.Secret, error) {
	secretClient := kubeCl.KubeClient.CoreV1().Secrets(d8SystemNS)
	deckhouseRegSecret, err := secretClient.Get(ctx, "deckhouse-registry", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	deckhouseRegSecret.StringData = newSecretData

	delete(deckhouseRegSecret.Data, caKey)

	return deckhouseRegSecret, nil
}

func updateImagePullSecret(ctx context.Context, kubeCl *kclient.KubernetesClient, newSecret *v1.Secret) error {
	secretClient := kubeCl.KubeClient.CoreV1().Secrets(d8SystemNS)

	updateOpts := metav1.UpdateOptions{FieldValidation: metav1.FieldValidationStrict}
	if _, err := secretClient.Update(ctx, newSecret, updateOpts); err != nil {
		return err
	}
	return nil
}

func newImagePullSecretData(newRepo name.Repository, authConfig authn.AuthConfig, caContent, specScheme string) (map[string]string, error) {
	authConfBytes, err := json.Marshal(
		map[string]map[string]*dockerCfgAuthEntry{
			"auths": {
				newRepo.RegistryStr(): encodeDockerCfgAuthEntryFromAuthConfig(authConfig),
			},
		},
	)
	if err != nil {
		return nil, err
	}

	scheme := specScheme
	if scheme != "http" && scheme != "https" {
		scheme = newRepo.Scheme()
	}

	newSecretData := map[string]string{
		".dockerconfigjson": string(authConfBytes),
		"address":           newRepo.RegistryStr(),
		"path":              path.Join("/", newRepo.RepositoryStr()),
		"scheme":            scheme,
	}

	if caContent != "" {
		newSecretData[caKey] = caContent
	}
	return newSecretData, nil
}

func getCAContent(caFile string) (string, error) {
	if caFile == "" {
		return "", nil
	}

	caBytes, err := os.ReadFile(caFile)
	if err != nil {
		return "", err
	}

	keyBlock, _ := pem.Decode(caBytes)
	if keyBlock == nil {
		return "", errors.New("can't read CA file content as pem file")
	}

	// Check that file content is a certificate
	if _, err := x509.ParseCertificate(keyBlock.Bytes); err != nil {
		return "", err
	}
	return strings.TrimSpace(string(caBytes)), nil
}

func deckhouseDeployment(ctx context.Context, kubeCl *kclient.KubernetesClient) (*appsv1.Deployment, error) {
	deployClient := kubeCl.KubeClient.AppsV1().Deployments(d8SystemNS)
	return deployClient.Get(ctx, "deckhouse", metav1.GetOptions{})
}

func updateDeployContainersImagesToNewRepo(deploy *appsv1.Deployment, newRepo name.Repository, nameOpts []name.Option, remoteOpts []remote.Option, newDeckhouseTag string) error {
	newInitContainers, err := updateImageRepoForContainers(deploy.Spec.Template.Spec.InitContainers, newRepo.Name(), nameOpts, remoteOpts, nil)
	if err != nil {
		return err
	}
	deploy.Spec.Template.Spec.InitContainers = newInitContainers

	newImageForDeckhouseContainer := make(map[string]string)
	if newDeckhouseTag != "" {
		newImageForDeckhouseContainer["deckhouse"] = newDeckhouseTag
	}

	newContainers, err := updateImageRepoForContainers(deploy.Spec.Template.Spec.Containers, newRepo.Name(), nameOpts, remoteOpts, newImageForDeckhouseContainer)
	if err != nil {
		return err
	}
	deploy.Spec.Template.Spec.Containers = newContainers

	return nil
}

func updateDeployment(ctx context.Context, kubeCl *kclient.KubernetesClient, deploy *appsv1.Deployment) error {
	deployClient := kubeCl.KubeClient.AppsV1().Deployments(deploy.Namespace)
	updateOpts := metav1.UpdateOptions{FieldValidation: metav1.FieldValidationStrict}
	if _, err := deployClient.Update(ctx, deploy, updateOpts); err != nil {
		return err
	}
	return nil
}

func updateImageRepoForContainers(containers []v1.Container, newRepository string, nameOpts []name.Option, remoteOpts []remote.Option, newTagsForContainer map[string]string /* map[<container name>]<newTag> */) ([]v1.Container, error) {
	for i, container := range containers {
		oldImage, err := name.ParseReference(container.Image)
		if err != nil {
			return nil, err
		}

		tagOrDigestref := oldImage.Identifier()
		if newTagsForContainer != nil {
			if v, f := newTagsForContainer[container.Name]; f && v != "" {
				tagOrDigestref = v
			}
		}

		var delim string
		switch oldImage.(type) {
		case name.Digest:
			delim = "@"
		case name.Tag:
			delim = ":"
		}

		newRef, err := name.ParseReference(newRepository+delim+tagOrDigestref, nameOpts...)
		if err != nil {
			return nil, err
		}

		// Check that new reference exists in the new repo before applying changes.
		if err := checkImageExists(newRef, remoteOpts); err != nil {
			return nil, fmt.Errorf("\nimage '%s' doesn't exist: %w", newRef.Name(), err)
		}

		container.Image = newRef.Name()
		containers[i] = container
	}

	return containers, nil
}

func checkImageExists(imageRef name.Reference, opts []remote.Option) error {
	img, err := remote.Image(imageRef, opts...)
	if err != nil {
		return err
	}

	if _, err := img.Digest(); err != nil {
		return err
	}
	return nil
}

// checkBearerSupport func checks that registry accepts bearer token authentification.
// This is modified "ping" func from
// https://github.com/google/go-containerregistry/blob/v0.5.1/pkg/v1/remote/transport/ping.go
func checkBearerSupport(ctx context.Context, reg name.Registry, roundTripper http.RoundTripper) error {
	client := &http.Client{Transport: roundTripper}

	// This first attempts to use "https" for every request, falling back to http
	// if the registry matches our localhost heuristic or if it is intentionally
	// set to insecure via name.NewInsecureRegistry.
	schemes := []string{"https"}
	if reg.Scheme() == "http" {
		schemes = append(schemes, "http")
	}

	var errs *multierror.Error
	for _, scheme := range schemes {
		resp, err := makeRequestWithScheme(ctx, client, scheme, reg.Name())
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("making request with %q scheme failed: %w", scheme, err))
			continue
		}

		err = checkResponseForBearerSupport(resp, reg.Name())
		if err == nil {
			return nil
		}

		errs = multierror.Append(errs, fmt.Errorf("check bearer support with %q scheme failed: %w", scheme, err))
	}

	return errs.ErrorOrNil()
}

func makeRequestWithScheme(ctx context.Context, client *http.Client, scheme, registryName string) (*http.Response, error) {
	u, err := url.Parse(fmt.Sprintf("%s://%s/v2/", scheme, registryName))
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Close body, because we only need headers
	return resp, resp.Body.Close()
}

func checkResponseForBearerSupport(resp *http.Response, registryHost string) error {
	if resp.StatusCode != http.StatusUnauthorized {
		return transport.CheckError(resp, http.StatusUnauthorized)
	}

	if authHeaderWithBearer(resp.Header) {
		return nil
	}

	return fmt.Errorf("can't use bearer token auth with registry %s", registryHost)
}

func authHeaderWithBearer(header http.Header) bool {
	const (
		wwwAuthHeader = "WWW-Authenticate"
		bearer        = "bearer"
	)

	for _, h := range header[http.CanonicalHeaderKey(wwwAuthHeader)] {
		if strings.HasPrefix(strings.ToLower(h), bearer) {
			return true
		}
	}

	return strings.ToLower(header.Get(wwwAuthHeader)) == bearer
}

type dockerCfgAuthEntry struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

func encodeDockerCfgAuthEntryFromAuthConfig(authConfig authn.AuthConfig) *dockerCfgAuthEntry {
	if authConfig.Username == "" && authConfig.Password == "" && authConfig.Auth == "" {
		return &dockerCfgAuthEntry{}
	}

	return &dockerCfgAuthEntry{
		Username: authConfig.Username,
		Password: authConfig.Password,
		Auth:     base64.StdEncoding.EncodeToString([]byte(authConfig.Username + ":" + authConfig.Password)),
	}
}
