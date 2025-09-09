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
	"log/slog"
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
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/yaml"

	deckhousev1alpha1 "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	kclient "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/go_lib/dependency/cr"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// TODO (alex123012): Use new methods in transport package for getting bearer token:
// in the PR https://github.com/google/go-containerregistry/pull/1709
// some methods became public and we can use them except of copying the source code.
// (waiting for new "go-containerregistry" version)

const (
	d8SystemNS         = "d8-system"
	caKey              = "ca"
	registryModuleName = "registry"
)

func ChangeRegistry(newRegistry, username, password, caFile, newDeckhouseImageTag, scheme string, dryRun bool, logger *log.Logger) error {
	ctx := context.Background()
	logEntry := logger.With("operator.component", "ChangeRegistry")

	kubeCl, err := newKubeClient()
	if err != nil {
		return err
	}

	logEntry.Info("Checking registry module")

	enabled, err := moduleEnabled(ctx, kubeCl, registryModuleName)
	if err != nil {
		return fmt.Errorf("failed to check if %q module is enabled: %w", registryModuleName, err)
	}

	if enabled {
		return fmt.Errorf("the %q module is enabled; please configure the registry using 'moduleConfig/deckhouse'", registryModuleName)
	}

	authConfig := newAuthConfig(username, password)

	caContent, err := getCAContent(caFile)
	if err != nil {
		return err
	}

	// !! Convert scheme to lowercase to avoid case-sensitive issues
	scheme = strings.ToLower(scheme)
	nameOpts := newNameOptions(scheme)
	newRepo, err := name.NewRepository(strings.TrimRight(newRegistry, "/"), nameOpts...)
	if err != nil {
		return err
	}

	caTransport := cr.GetHTTPTransport(caContent)

	if err := checkAuthSupport(ctx, newRepo.Registry, caTransport); err != nil {
		return err
	}

	logEntry.Info("Retrieving deckhouse deployment...")
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
		logEntry.Info("Dry-run enabled")
		secretYaml, _ := yaml.Marshal(deckhouseSecret)
		deploymentYaml, _ := yaml.Marshal(deckhouseDeploy)
		logEntry.Info("------------------------------")
		logEntry.Info(fmt.Sprintf("New Secret will be applied:\n%s\n", secretYaml))
		logEntry.Info("------------------------------")
		logEntry.Info(fmt.Sprintf("New Deployment will be applied:\n%s\n", deploymentYaml))
	} else {
		logEntry.Info("Updating deckhouse image pull secret...")
		if err := updateImagePullSecret(ctx, kubeCl, deckhouseSecret); err != nil {
			return err
		}

		logEntry.Info("Updating deckhouse deployment...")
		if err := updateDeployment(ctx, kubeCl, deckhouseDeploy); err != nil {
			return err
		}
	}

	logEntry.Info("Done")
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
func checkAuthSupport(ctx context.Context, reg name.Registry, roundTripper http.RoundTripper) error {
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

		err = checkResponseForAuthSupport(resp, reg.Name())
		if err == nil {
			return nil
		}

		errs = multierror.Append(errs, fmt.Errorf("check auth support with %q scheme failed: %w", scheme, err))
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

func checkResponseForAuthSupport(resp *http.Response, registryHost string) error {
	if resp.StatusCode != http.StatusUnauthorized {
		return transport.CheckError(resp, http.StatusUnauthorized)
	}

	if authHeader(resp.Header) {
		return nil
	}

	return fmt.Errorf("can't use bearer or basic auth with registry %s", registryHost)
}

func authHeader(headers http.Header) bool {
	authSchemes := []string{"bearer", "basic"}
	authHeader := headers.Get("WWW-Authenticate")
	if authHeader == "" {
		log.Info("Empty WWW-Authenticate header")
		return false
	}

	lowerHeader := strings.ToLower(authHeader)
	for _, scheme := range authSchemes {
		if strings.HasPrefix(lowerHeader, scheme) {
			return true
		}
	}
	log.Info("WWW-Authenticate header has an incorrect value", slog.String("value", authHeader))
	return false
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

func moduleEnabled(ctx context.Context, kubeCl *kclient.KubernetesClient, moduleName string) (bool, error) {
	moduleUnstructured, err := kubeCl.
		Dynamic().
		Resource(deckhousev1alpha1.ModuleGVR).
		Namespace("").
		Get(ctx, moduleName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get module: %w", err)
	}

	moduleJSON, err := moduleUnstructured.MarshalJSON()
	if err != nil {
		return false, fmt.Errorf("failed to marshal unstructured module: %w", err)
	}

	var module deckhousev1alpha1.Module
	decoder := serializer.
		NewCodecFactory(runtime.NewScheme()).
		UniversalDeserializer()
	if _, _, err := decoder.Decode(moduleJSON, nil, &module); err != nil {
		return false, fmt.Errorf("failed to decode module JSON: %w", err)
	}

	enabled := module.IsCondition(deckhousev1alpha1.ModuleConditionEnabledByModuleManager, v1.ConditionTrue)
	return enabled, nil
}
