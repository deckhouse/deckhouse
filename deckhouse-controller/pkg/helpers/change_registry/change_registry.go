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
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"

	authchallenge "github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kclient "github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// TODO (alex123012): Use new methods in transport package for getting bearer token:
// in the PR https://github.com/google/go-containerregistry/pull/1709
// some methods became public and we can use them except of copying the source code.
// (waiting for new "go-containerregistry" version)

const (
	d8SystemNS = "d8-system"
)

func ChangeRegistry(newRegistry, username, password, caFile, newDeckhouseImageTag string, insecure bool) error {
	ctx := context.TODO()
	logEntry := log.WithField("operator.component", "ChangeRegistry")

	nameOpts := newNameOptions(insecure)
	newRepo, err := name.NewRepository(newRegistry, nameOpts...)
	if err != nil {
		return err
	}

	if err := checkBearerSupport(ctx, newRepo.Registry); err != nil {
		return err
	}

	authConfig := newAuthConfig(username, password)

	remoteOpts, err := newRemoteOptions(ctx, newRepo, authConfig)
	if err != nil {
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

	// Check that all images for deckhouse deploy exist in the new repo before
	// updating image pull secret and prepare deployment with updated images.
	if err := updateDeployContainersImagesToNewRepo(deckhouseDeploy, newRepo, nameOpts, remoteOpts, newDeckhouseImageTag); err != nil {
		return err
	}

	imagePullSecretData, err := newImagePullSecretData(newRepo, authConfig, caFile)
	if err != nil {
		return err
	}

	logEntry.Println("Updating deckhouse image pull secret...")
	if err := updateImagePullSecret(ctx, kubeCl, imagePullSecretData); err != nil {
		return err
	}

	logEntry.Println("Updating deckhouse deployment...")
	if err := updateDeployment(ctx, kubeCl, deckhouseDeploy); err != nil {
		return err
	}

	logEntry.Println("Done")
	return nil
}

func newAuthConfig(username, password string) authn.AuthConfig {
	var cfg authn.AuthConfig
	if username != "" && password != "" {
		cfg = authn.AuthConfig{
			Username: username,
			Password: password,
			Auth:     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
		}
	}
	return cfg
}

func newRemoteOptions(ctx context.Context, repo name.Repository, authConfig authn.AuthConfig) ([]remote.Option, error) {
	var opts []remote.Option

	transportOpt, err := newTransportOption(ctx, repo, authConfig)
	if err != nil {
		return nil, err
	}

	opts = append(opts, transportOpt)
	return opts, nil
}

func newNameOptions(insecure bool) []name.Option {
	opts := []name.Option{name.StrictValidation}
	if insecure {
		opts = append(opts, name.Insecure)
	}
	return opts
}

func newTransportOption(ctx context.Context, repo name.Repository, authConfig authn.AuthConfig) (remote.Option, error) {
	authorizer := authn.FromConfig(authConfig)

	scopes := []string{repo.Scope(transport.PullScope)}
	t, err := transport.NewWithContext(ctx, repo.Registry, authorizer, http.DefaultTransport, scopes)
	if err != nil {
		return nil, err
	}

	return remote.WithTransport(t), nil
}

func newKubeClient() (kclient.KubeClient, error) {
	kubeCl := kclient.NewKubernetesClient()
	if err := kubeCl.Init(kclient.AppKubernetesInitParams()); err != nil {
		return nil, err
	}
	return kubeCl.KubeClient, nil
}

func updateImagePullSecret(ctx context.Context, kubeCl kclient.KubeClient, newSecretData map[string]string) error {
	secretClient := kubeCl.CoreV1().Secrets(d8SystemNS)
	deckhouseRegSecret, err := secretClient.Get(ctx, "deckhouse-registry", metav1.GetOptions{})
	if err != nil {
		return err
	}
	deckhouseRegSecret.StringData = newSecretData

	updateOpts := metav1.UpdateOptions{FieldValidation: metav1.FieldValidationStrict}
	if _, err := secretClient.Update(ctx, deckhouseRegSecret, updateOpts); err != nil {
		return err
	}
	return nil
}

func newImagePullSecretData(newRepo name.Repository, authConfig authn.AuthConfig, caFile string) (map[string]string, error) {
	authConfBytes, err := json.Marshal(
		map[string]map[string]authn.AuthConfig{
			"auths": {
				newRepo.RegistryStr(): authn.AuthConfig{Auth: authConfig.Auth},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	newSecretData := map[string]string{
		".dockerconfigjson": string(authConfBytes),
		"address":           newRepo.RegistryStr(),
		"path":              path.Join("/", newRepo.RepositoryStr()),
		"scheme":            newRepo.Scheme(),
	}

	if caFile != "" {
		ca, err := getCAContent(caFile)
		if err != nil {
			return nil, err
		}
		newSecretData["ca"] = ca
	}
	return newSecretData, nil
}

func getCAContent(caFile string) (string, error) {
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
	return strings.Trim(string(caBytes), "\n "), nil
}

func deckhouseDeployment(ctx context.Context, kubeCl kclient.KubeClient) (*appsv1.Deployment, error) {
	deployClient := kubeCl.AppsV1().Deployments(d8SystemNS)
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

func updateDeployment(ctx context.Context, kubeCl kclient.KubeClient, deploy *appsv1.Deployment) error {
	deployClient := kubeCl.AppsV1().Deployments(deploy.Namespace)
	updateOpts := metav1.UpdateOptions{FieldValidation: metav1.FieldValidationStrict}
	if _, err := deployClient.Update(ctx, deploy, updateOpts); err != nil {
		return err
	}
	return nil
}

func updateImageRepoForContainers(containers []v1.Container, newRepository string, nameOpts []name.Option, remoteOpts []remote.Option, newTagsForContainer map[string]string /* map[<container name>]>newTag> */) ([]v1.Container, error) {
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
func checkBearerSupport(ctx context.Context, reg name.Registry) error {
	const (
		bearer        = "bearer"
		wwwAuthHeader = "WWW-Authenticate"
	)

	client := http.Client{Transport: http.DefaultTransport}

	// This first attempts to use "https" for every request, falling back to http
	// if the registry matches our localhost heuristic or if it is intentionally
	// set to insecure via name.NewInsecureRegistry.
	schemes := []string{"https"}
	if reg.Scheme() == "http" {
		schemes = append(schemes, "http")
	}

	var errs []string
	for _, scheme := range schemes {
		url := fmt.Sprintf("%s://%s/v2/", scheme, reg.Name())
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			errs = append(errs, err.Error())
			// Potentially retry with http.
			continue
		}
		defer func() {
			// By draining the body, make sure to reuse the connection made by
			// the ping for the following access to the registry
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body)
		}()

		if resp.StatusCode != http.StatusUnauthorized {
			return transport.CheckError(resp, http.StatusUnauthorized)
		}

		if challenges := authchallenge.ResponseChallenges(resp); len(challenges) != 0 {
			// If we hit more than one, I'm not even sure what to do.
			wac := challenges[0]
			if toChallenge(wac.Scheme) == bearer {
				return nil
			}
		}
		if toChallenge(resp.Header.Get(wwwAuthHeader)) == bearer {
			return nil
		}
		return fmt.Errorf("can't use bearer token auth with registry %s", reg.Name())
	}

	return errors.New(strings.Join(errs, "; "))
}

func toChallenge(s string) string {
	return strings.ToLower(s)
}
