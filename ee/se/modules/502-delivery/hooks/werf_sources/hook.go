/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const namespace = "d8-delivery"

// werfSourceConfig is a DTO for the WerfSource CRD, used to pass the data from the custom resource
// to the hook
type werfSourceConfig struct {
	// object Name that will be shared with argocd repo and image updater registry
	Name string

	// container image repository: cr.example.com/path/to(/image)
	Repo string

	// container image registry API URL if the hostname is not the same as repository first segment
	APIURL string

	// name of creadentials secret in d8-delivery namespace, the secret is expected to have
	// dockerconfigjson format
	PullSecretName string

	// Whether the Argo CD repository should be created for the source
	ArgocdRepoEnabled *bool

	// Argo CD repository settings; skipped if the value is nil
	ArgocdRepo *argocdRepoConfig
}

// werfSource is an inner represenataion of the WerfSource CRD, used to pass the data to Argo CD
// repos and Argo CD Image Updater registries.
type werfSource struct {
	// object Name that will be shared with argocd repo and image updater registry
	Name string

	// container image repository: cr.example.com/path/to(/image)
	Repo string

	// container image registry API URL if the hostname is not the same as repository first segment
	APIURL string

	// name of creadentials secret in d8-delivery namespace, the secret is expected to have
	// dockerconfigjson format
	PullSecretName string

	// Argo CD repository settings; skipped if the value is nil
	ArgocdRepo *argocdRepoConfig
}

// argocdRepoConfig is the set of options for Argo CD repository configuration.
type argocdRepoConfig struct {
	Project string
}

// imageUpdaterRegistry reflects container registries that the Argo CD Image Updater will track, the
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

// argocdHelmOCIRepository reflects OCI Helm repos to be used as Argo CD repository for werf bundles,
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
			FilterFunc: filterWerfSourceConfig,
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
			// We use field selector in the subscription to reduce memory footprint. Additionally, we
			// duplicate the check because currently test framework cannot handle field selector.
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{{
					Field:    "type",
					Operator: "Equals",
					Value:    string(corev1.SecretTypeDockerConfigJson),
				}},
			},
			FilterFunc: filterDockerConfigJSON,
		},
	},
}, applyWerfSources)

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

func applyWerfSources(input *go_hook.HookInput) error {
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

func mapWerfSources(werfSources []werfSource, credsBySecret map[string]dockerFileConfig) (internalValues, error) {
	argoRepos, err := convArgoCDRepositories(werfSources, credsBySecret)
	if err != nil {
		return internalValues{}, err
	}
	imageUpdaterRegistries := convImageUpdaterRegistries(werfSources)

	vals := internalValues{
		ArgoCD:             internalArgoCDValues{Repositories: argoRepos},
		ArgoCDImageUpdater: internalUpdaterValues{Registries: imageUpdaterRegistries},
	}

	return vals, nil
}

func convImageUpdaterRegistries(werfSources []werfSource) []imageUpdaterRegistry {
	var registries []imageUpdaterRegistry
	for _, ws := range werfSources {
		url := ws.APIURL
		if url == "" {
			url = "https://" + firstSegment(ws.Repo)
		}

		var pullCreds string
		if ws.PullSecretName != "" {
			pullCreds = "pullsecret:d8-delivery/" + ws.PullSecretName
		}

		registries = append(registries, imageUpdaterRegistry{
			Name:        ws.Name,
			Prefix:      firstSegment(ws.Repo),
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
		if ws.ArgocdRepo == nil {
			continue
		}
		registry := firstSegment(ws.Repo)
		creds, err := extractCredentials(credentialsBySecretName, ws.PullSecretName, registry)
		if err != nil {
			return nil, fmt.Errorf("extracting credentials for registry %q in secret %q: %v", registry, ws.PullSecretName, err)
		}

		argoRepos = append(argoRepos, argocdHelmOCIRepository{
			Name:     ws.Name,
			Username: creds.username,
			Password: creds.password,
			Project:  ws.ArgocdRepo.Project,
			URL:      ws.Repo,
		})
	}
	return argoRepos, nil
}

func extractCredentials(credentialsBySecretName map[string]dockerFileConfig, pullSecretName, registry string) (registryCredentials, error) {
	creds := registryCredentials{}
	if pullSecretName == "" {
		// No credentials is OK for public registries, there can be unspecified pullsecret
		return creds, nil
	}

	config, ok := credentialsBySecretName[pullSecretName]
	if !ok {
		return creds, fmt.Errorf("unknown pull secret %q", pullSecretName)
	}

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

// cr.example.com/path/to/image -> cr.example.com
func firstSegment(s string) string {
	for i, c := range s {
		if c == '/' {
			return s[:i]
		}
	}
	return s
}

func filterWerfSourceConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var (
		wsc werfSourceConfig
		err error
		ok  bool
	)

	wsc.Name = obj.GetName()

	wsc.Repo, _, err = unstructured.NestedString(obj.Object, "spec", "imageRepo")
	if err != nil {
		return nil, err
	}

	wsc.APIURL, _, err = unstructured.NestedString(obj.Object, "spec", "apiURL")
	if err != nil {
		return nil, err
	}

	wsc.PullSecretName, _, err = unstructured.NestedString(obj.Object, "spec", "pullSecretName")
	if err != nil {
		return nil, err
	}

	// By default, Argo CD repository is desired, but the OCI repo can be disabled to use purely
	// Argo CD Image Updater along with another repository type (git or helm chart museum).
	repoEnabled, ok, err := unstructured.NestedBool(obj.Object, "spec", "argocdRepoEnabled")
	if err != nil {
		return nil, err
	}
	if ok {
		wsc.ArgocdRepoEnabled = &repoEnabled
	}

	// By default, Argo CD repo belongs to the "default" project.
	arepo, _, err := unstructured.NestedStringMap(obj.Object, "spec", "argocdRepo")
	if err != nil {
		return nil, err
	}
	wsc.ArgocdRepo = &argocdRepoConfig{
		Project: arepo["project"],
	}

	return wsc, nil
}

func parseWerfSources(snapshots []go_hook.FilterResult) ([]werfSource, error) {
	var wss []werfSource
	for _, snap := range snapshots {
		wsConfig, ok := snap.(werfSourceConfig)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", snap)
		}
		ws, err := parseWefSource(wsConfig)
		if err != nil {
			return nil, fmt.Errorf("parsing WerfSource %q: %v", ws.Name, err)
		}

		wss = append(wss, ws)
	}
	return wss, nil
}

func parseWefSource(wsConfig werfSourceConfig) (werfSource, error) {
	var ws werfSource

	ws.Name = wsConfig.Name
	ws.PullSecretName = wsConfig.PullSecretName

	if wsConfig.Repo == "" {
		return ws, fmt.Errorf("missing spec.imageRepo field")
	}
	ws.Repo = wsConfig.Repo

	if wsConfig.APIURL == "" {
		ws.APIURL = "https://" + firstSegment(ws.Repo)
	} else {
		ws.APIURL = wsConfig.APIURL
	}

	// By default, Argo CD is desired, but the OCI repo can be disabled to use purely Image
	// Updater functionality along with another repository type.
	repoEnabled := wsConfig.ArgocdRepoEnabled == nil || *wsConfig.ArgocdRepoEnabled
	if repoEnabled {
		ws.ArgocdRepo = &argocdRepoConfig{
			Project: parseProject(wsConfig.ArgocdRepo),
		}
	}

	return ws, nil
}

// parseProject returns "default" project if not specified.
func parseProject(arc *argocdRepoConfig) string {
	if arc == nil || arc.Project == "" {
		return "default"
	}
	return arc.Project
}

type registryCredentials struct {
	username string
	password string
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

	// We use field selector in the subscription to reduce memory footprint. Additionally, we
	// duplicate the check because currently test framework cannot handle field selector.
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
	res := map[string]dockerFileConfig{}
	for _, snap := range snapshots {
		if snap == nil {
			continue
		}
		creds, ok := snap.(credSecret)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", snap)
		}
		res[creds.Name] = creds.Config
	}
	return res, nil
}
