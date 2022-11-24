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

package hooks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const namespace = "d8-delivery"

// werfSource is a DTO for the WerfSource CRD, used to pass the data to ArgoCD repos and ArgoCD
// Image Updater registries.
type werfSource struct {
	// object name that will be shared with argocd repo and image updater registry
	name string

	// container image repository: cr.example.com/path/to(/image)
	repo string

	// container image registry API URL if the hostname is not the same as repository first segment
	apiURL string

	// name of creadentials secret in d8-delivery namespace, the secret is expected to have
	// dockerconfigjson format
	pullSecretName string

	// ArgoCD repository settings; skipped if the value is nil
	argocdRepo *argocdRepoConfig
}

// argocdRepoConfig is the set of options for ArgoCD repository configuration.
type argocdRepoConfig struct {
	project string
}

// imageUpdaterRegistry reflects container registries that the ArgoCD Image Updater will track, the
// JSON mapping is taken from the upstream:
// https://argocd-image-updater.readthedocs.io/en/v0.6.2/configuration/registries/#configuring-a-custom-container-registry.
type imageUpdaterRegistry struct {
	Name        string `json:"name"`
	Prefix      string `json:"prefix"`
	APIURL      string `json:"api_url"`
	Credentials string `json:"credentials,omitempty"`
	Default     bool   `json:"default"` // TODO (shvgn) accept this flag from the CRD

	// TODO (shvgn) consider 'insecure' and 'ping' fields
}

// argocdHelmOCIRepository reflects OCI Helm repos to be used as ArgoCD repository for werf bundles,
// type=helm and enableOCI=true are enforced.
//
// Doc examples https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#helm-chart-repositories
// OCI-related examples https://github.com/argoproj/argo-cd/issues/7121
type argocdHelmOCIRepository struct {
	Name     string `json:"name"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Project  string `json:"project"`

	// actually, a container repo in the form "cr.example.com/path/to(/image)"
	URL string `json:"url"`

	// TODO (shvgn) consider 'tlsClientCertData' and 'tlsClientCertKey' fields
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/deckhouse/werf_sources",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "werf_sources",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "WerfSource",
			FilterFunc: filterWerfSource,
		},
		{
			Name:       "credentials_secrets",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{namespace},
				},
			},
			FilterFunc: filterDockerConfigJSON,
		},
	},
}, dependency.WithExternalDependencies(applyWerfSources))

type internalValues struct {
	ArgoCD             internalArgoCDValues  `json:"argocd"`
	ArgoCDImageUpdater internalUpdaterValues `json:"argocdImageUpdater"`
}

type internalArgoCDValues struct {
	Repositories []argocdHelmOCIRepository `json:"repositories"`
}

type internalUpdaterValues struct {
	Registries []imageUpdaterRegistry `json:"registries"`
}

func applyWerfSources(input *go_hook.HookInput, dc dependency.Container) error {
	// Input
	werfSources, err := parseWerfSources(input.Snapshots["werf_sources"])
	if err != nil {
		return fmt.Errorf("cannot parse WerfSources: %v", err)
	}
	if len(werfSources) == 0 {
		return nil
	}
	credsBySecretName, err := parseDockerConfigsBySecretName(input.Snapshots["credentials_secrets"])
	if err != nil {
		return fmt.Errorf("cannot parse credentials secrets: %v", err)
	}

	// Convert to values
	vals, err := mapWerfSources(werfSources, credsBySecretName)
	if err != nil {
		return err
	}

	// Output
	input.Values.Set("delivery.internal", vals)

	return nil
}

func mapWerfSources(werfSources []werfSource, credsBySecret map[string]dockerFileConfig) (vals internalValues, err error) {
	argoRepos, err := convArgoCDRepositories(werfSources, credsBySecret)
	if err != nil {
		return vals, fmt.Errorf("cannot convert ArgoCD repositories: %v", err)
	}

	imageUpdaterRegistries := convImageUpdaterRegistries(werfSources)

	vals = internalValues{
		ArgoCD:             internalArgoCDValues{Repositories: argoRepos},
		ArgoCDImageUpdater: internalUpdaterValues{Registries: imageUpdaterRegistries},
	}

	return vals, nil
}

func convImageUpdaterRegistries(werfSources []werfSource) []imageUpdaterRegistry {
	var registries []imageUpdaterRegistry
	for _, ws := range werfSources {

		url := ws.apiURL
		if url == "" {
			url = "https://" + firstSegment(ws.repo)
		}

		var pullCreds string
		if ws.pullSecretName != "" {
			pullCreds = "pullsecret:d8-delivery/" + ws.pullSecretName
		}

		registries = append(registries, imageUpdaterRegistry{
			Name:        ws.name,
			Prefix:      firstSegment(ws.repo),
			APIURL:      url,
			Credentials: pullCreds,
			Default:     false,
		})
	}
	return registries
}

func convArgoCDRepositories(werfSources []werfSource, credentialsBySecretName map[string]dockerFileConfig) ([]argocdHelmOCIRepository, error) {
	var argoRepos []argocdHelmOCIRepository
	for _, ws := range werfSources {
		if ws.argocdRepo == nil {
			continue
		}
		username, password := "", ""
		config, ok := credentialsBySecretName[ws.pullSecretName]
		if ok {
			registry := firstSegment(ws.repo)
			creds, err := parseDockerConfigJSONCredentials(config, registry)
			if err != nil {
				return nil, err
			}
			username, password = creds.username, creds.password
		}

		argoRepos = append(argoRepos, argocdHelmOCIRepository{
			Name:     ws.name,
			Username: username,
			Password: password,
			Project:  ws.argocdRepo.project,
			URL:      ws.repo,
		})
	}
	return argoRepos, nil
}

// cr.example.com/path/to/image -> cr.example.com
func firstSegment(s string) string {
	for i, c := range s {
		if c == '/' {
			return s[:i]
		}
	}
	return s
}

func filterWerfSource(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		ws  werfSource
		err error
		ok  bool
	)

	ws.name = obj.GetName()

	ws.repo, ok, err = unstructured.NestedString(obj.Object, "spec", "imageRepo")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("spec.imageRepo field expected")
	}

	ws.apiURL, ok, err = unstructured.NestedString(obj.Object, "spec", "apiUrl")
	if err != nil {
		return nil, err
	}
	if !ok {
		ws.apiURL = "https://" + firstSegment(ws.repo)
	}

	ws.pullSecretName, _, err = unstructured.NestedString(obj.Object, "spec", "pullSecretName")
	if err != nil {
		return nil, err
	}

	// By default, Argo CD is desired, but the OCI repo can be disabled to use purely Image
	// Updater functionality along with another repository type.
	repoEnabled, ok, err := unstructured.NestedBool(obj.Object, "spec", "argocdRepoEnabled")
	if err != nil {
		return nil, err
	}
	if !ok {
		repoEnabled = true
	}

	// By default, Argo CD repo belongs to the "default" project.
	arepo, ok, err := unstructured.NestedStringMap(obj.Object, "spec", "argocdRepo")
	if err != nil {
		return nil, err
	}
	project := "default"
	if repoEnabled && ok {
		specifiedProject, projectSpecified := arepo["project"]
		if projectSpecified && specifiedProject != "" {
			project = specifiedProject
		}
	}
	if repoEnabled {
		ws.argocdRepo = &argocdRepoConfig{
			project: project,
		}
	}

	return ws, nil
}

func parseWerfSources(snapshots []go_hook.FilterResult) ([]werfSource, error) {
	var res []werfSource
	for _, snap := range snapshots {
		r, ok := snap.(werfSource)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", snap)
		}
		res = append(res, r)
	}
	return res, nil
}

type registryCredentials struct {
	username string
	password string
}

func parseDockerConfigJSONCredentials(config dockerFileConfig, registry string) (registryCredentials, error) {
	creds := registryCredentials{}

	cfg, ok := config.Auths[registry]
	if !ok {
		return creds, fmt.Errorf("no credentials")
	}

	if cfg.Auth != "" {
		auth, err := base64.StdEncoding.DecodeString(cfg.Auth)
		if err != nil {
			return creds, fmt.Errorf(`cannot decode base64 "auth" field`)
		}
		parts := strings.Split(string(auth), ":")
		if len(parts) != 2 {
			return creds, fmt.Errorf(`unexpected format of "auth" field, expected "username:password"`)
		}
		creds.username, creds.password = parts[0], parts[1]
		return creds, nil
	}

	creds.username, creds.password = cfg.Username, cfg.Password
	return creds, nil
}

/*
	{ "auths":{
	        "cr.example.com":{
			"username":"...",
			"password":"...",
			"auth":"base64([username]:[password])",
			"email":"...@example.com"
		}
	}}
*/
type dockerFileConfig struct {
	Auths map[string]authn.AuthConfig `json:"auths"`
}

type credSecret struct {
	Name   string
	Config dockerFileConfig
}

func filterDockerConfigJSON(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	if secret.Type != corev1.SecretTypeDockerConfigJson {
		return nil, nil
	}

	if secret.Data == nil {
		return nil, nil
	}

	rawCreds, ok := secret.Data[corev1.DockerConfigJsonKey]
	if !ok {
		return nil, nil
	}

	var config dockerFileConfig
	if err := json.Unmarshal(rawCreds, &config); err != nil {
		return nil, fmt.Errorf("cannot decode docker config JSON: %v", err)
	}

	creds := credSecret{
		Name:   secret.GetName(),
		Config: config,
	}
	return creds, nil
}

func parseDockerConfigsBySecretName(snapshots []go_hook.FilterResult) (map[string]dockerFileConfig, error) {
	var res map[string]dockerFileConfig
	for _, snap := range snapshots {
		creds, ok := snap.(credSecret)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", snap)
		}
		res[creds.Name] = creds.Config
	}
	return res, nil
}
