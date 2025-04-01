// Copyright 2024 Flant JSC
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

package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	util_time "github.com/deckhouse/deckhouse/dhctl/pkg/util/time"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	registry_models "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/pki"
	"regexp"
	"slices"
	"strings"
)

type Registry struct {
	Data               RegistryData `json:"-"`
	ModeSpecificFields interface{}  `json:"-"`
}

type RegistryData struct {
	Address   string `json:"address"`
	Path      string `json:"path"`
	Scheme    string `json:"scheme"`
	CA        string `json:"ca"`
	DockerCfg string `json:"dockerCfg"`
}

type ProxyModeRegistryData struct {
	InternalRegistryPKI  RegistryPKI          `json:"-"`
	InternalRegistryTTL  util_time.Duration   `json:"-"`
	UpstreamRegistryData UpstreamRegistryData `json:"-"`
}

type DetachedModeRegistryData struct {
	InternalRegistryPKI RegistryPKI `json:"-"`
	ImagesBundlePath    string      `json:"-"`
	RegistryPath        string      `json:"-"`
}

type UpstreamRegistryData struct {
	Address  string `json:"-"`
	Path     string `json:"-"`
	Scheme   string `json:"-"`
	CA       string `json:"-"`
	Username string `json:"-"`
	Password string `json:"-"`
}

type RegistryPKI struct {
	CA     CertKey           `json:"-"`
	UserRW registry_pki.User `json:"-"`
	UserRO registry_pki.User `json:""`
}

type CertKey struct {
	Cert string
	Key  string
}

func NewRegistryCfg(registryClusterCfg RegistryClusterConfig) (registryCfg Registry, isDataDeviceEnable bool, err error) {
	switch registryClusterCfg.Mode {
	case registry_const.ModeDirect:
		properties := registryClusterCfg.DirectModeProperties
		if properties == nil {
			err = fmt.Errorf("unable to get the properties of the direct registry mode")
			return
		}
		address, path := getRegistryAddressAndPathFromImagesRepo(properties.ImagesRepo)

		// for iniConfig deckhouse.io/v1
		dockerCfg := properties.DockerCfg
		if dockerCfg == "" {
			dockerCfg, err = generateDockerCfgBase64(properties.Username, properties.Password, address)
			if err != nil {
				err = fmt.Errorf("failed to generate docker config from username and password")
				return
			}
		}

		registryCfg = Registry{
			Data: RegistryData{
				Address:   address,
				Path:      path,
				Scheme:    strings.ToLower(properties.Scheme),
				CA:        properties.CA,
				DockerCfg: dockerCfg,
			},
		}
	case registry_const.ModeProxy:
		isDataDeviceEnable = true
		properties := registryClusterCfg.ProxyModeProperties
		if properties == nil {
			err = fmt.Errorf("unable to get the properties of the proxy registry mode")
			return
		}
		address, path := getRegistryAddressAndPathFromImagesRepo(properties.ImagesRepo)

		registryCfg = Registry{
			Data: RegistryData{
				Address: registry_const.Host,
				Path:    registry_const.Path,
				// These parameters are filled in the method `PrepareAfterGlobalCacheInit`:
				// Scheme:       "",
				// DockerCfg:    "",
				// CA:           "",
			},
			ModeSpecificFields: ProxyModeRegistryData{
				UpstreamRegistryData: UpstreamRegistryData{
					Address:  address,
					Path:     path,
					Scheme:   strings.ToLower(properties.Scheme),
					CA:       properties.CA,
					Username: properties.Username,
					Password: properties.Password,
				},
				InternalRegistryTTL: properties.TTL,
				// These parameters are filled in the method `PrepareAfterGlobalCacheInit`:
				// internalRegistryPKI: "",
			},
		}
	case registry_const.ModeDetached:
		isDataDeviceEnable = true
		properties := registryClusterCfg.DetachedModeProperties
		if properties == nil {
			err = fmt.Errorf("unable to get the properties of the detached registry mode")
			return
		}

		registryCfg = Registry{
			Data: RegistryData{
				Address: registry_const.Host,
				Path:    registry_const.Path,
				// These parameters are filled in the method `PrepareAfterGlobalCacheInit`:
				// Scheme:       "",
				// DockerCfg:    "",
				// CA:           "",
			},
			ModeSpecificFields: DetachedModeRegistryData{
				RegistryPath:     registry_const.Path,
				ImagesBundlePath: properties.ImagesBundlePath,
				// These parameters are filled in the method `PrepareAfterGlobalCacheInit`:
				// internalRegistryPKI: "",
			},
		}
	default:
		err = fmt.Errorf("unknown registry mode: %s", registryClusterCfg.Mode)
		return
	}
	return
}

func (r *Registry) PrepareAfterGlobalCacheInit() error {
	if !r.IsDirect() {
		internalRegistryPKI, err := getRegistryPKI()
		if err != nil {
			return fmt.Errorf("unable to get internal registry pki: %v", err)
		}

		switch modeSpecificFields := r.ModeSpecificFields.(type) {
		case ProxyModeRegistryData:
			modeSpecificFields.InternalRegistryPKI = *internalRegistryPKI
			r.ModeSpecificFields = modeSpecificFields
		case DetachedModeRegistryData:
			modeSpecificFields.InternalRegistryPKI = *internalRegistryPKI
			r.ModeSpecificFields = modeSpecificFields
		}

		r.Data.DockerCfg, err = generateDockerCfgBase64(
			internalRegistryPKI.UserRO.Name,
			internalRegistryPKI.UserRO.Password,
			r.Data.Address,
		)
		if err != nil {
			return fmt.Errorf("unable to generate dockerCfg: %v", err)
		}
		r.Data.Scheme = registry_const.Scheme
		r.Data.CA = internalRegistryPKI.CA.Cert
	}
	return nil
}

func (r *Registry) Mode() registry_const.ModeType {
	switch r.ModeSpecificFields.(type) {
	case ProxyModeRegistryData:
		return registry_const.ModeProxy
	case DetachedModeRegistryData:
		return registry_const.ModeDetached
	}
	return registry_const.ModeDirect
}

func (r *Registry) IsDirect() bool {
	return r.Mode() == registry_const.ModeDirect
}

func (r *Registry) IsProxy() (*ProxyModeRegistryData, bool) {
	data, ok := r.ModeSpecificFields.(ProxyModeRegistryData)
	if ok {
		return &data, true
	}
	return nil, false
}

func (r *Registry) IsDetached() (*DetachedModeRegistryData, bool) {
	data, ok := r.ModeSpecificFields.(DetachedModeRegistryData)
	if ok {
		return &data, true
	}
	return nil, false
}

func (r Registry) DeepCopy() Registry {
	var modeSpecificFieldsCopy interface{}
	switch modeSpecificFields := r.ModeSpecificFields.(type) {
	case ProxyModeRegistryData:
		modeSpecificFieldsCopy = modeSpecificFields.DeepCopy()
	case DetachedModeRegistryData:
		modeSpecificFieldsCopy = modeSpecificFields.DeepCopy()
	}
	return Registry{
		Data:               r.Data,
		ModeSpecificFields: modeSpecificFieldsCopy,
	}
}

func (r Registry) KubeadmTemplatesContext() (map[string]interface{}, error) {
	return r.Data.ConvertToMap()
}

func (r Registry) BashibleBundleTemplateContext() (map[string]interface{}, error) {
	// prepare common data
	imagesBase := fmt.Sprintf("%s/%s", r.Data.Address, strings.TrimLeft(r.Data.Path, "/"))
	auth, err := r.Data.Auth()
	if err != nil {
		return nil, err
	}

	// prepare mirrrors and proxy endpoints
	mirrors := []registry_models.MirrorHostObject{}
	prepullMirrors := []registry_models.MirrorHostObject{}
	proxyEndpoints := []string{}
	if registry_const.ShouldRunStaticPodRegistry(r.Mode()) {
		// If static pod registry
		for _, host := range []string{registry_const.ProxyHost} {
			mirrors = append(mirrors, registry_models.MirrorHostObject{
				Host:     host,
				Username: "",
				Password: "",
				Auth:     auth,
				Scheme:   r.Data.Scheme,
			})
		}
		// ${discovered_node_ip} - bashible will use this as a placeholder on envsubst call
		// address will be discovered in one of bashible steps
		proxyEndpoints = registry_const.GenerateProxyEndpoints([]string{"${discovered_node_ip}"})
		for _, host := range append([]string{registry_const.ProxyHost}, proxyEndpoints...) {
			prepullMirrors = append(prepullMirrors, registry_models.MirrorHostObject{
				Host:     host,
				Username: "",
				Password: "",
				Auth:     auth,
				Scheme:   r.Data.Scheme,
			})
		}
	} else {
		// if not static pod registry
		for _, host := range []string{r.Data.Address} {
			mirrors = append(mirrors, registry_models.MirrorHostObject{
				Host:     host,
				Username: "",
				Password: "",
				Auth:     auth,
				Scheme:   r.Data.Scheme,
			})
		}
		prepullMirrors = slices.Clone(mirrors)
	}

	cfg := registry_models.BashibleContext{
		Mode:           r.Mode(),
		Version:        registry_const.DefaultVersion,
		ImagesBase:     imagesBase,
		ProxyEndpoints: proxyEndpoints,
		Hosts:          []registry_models.HostsObject{{Host: r.Data.Address, CA: []string{r.Data.CA}, Mirrors: mirrors}},
		PrepullHosts:   []registry_models.HostsObject{{Host: r.Data.Address, CA: []string{r.Data.CA}, Mirrors: prepullMirrors}},
	}

	mapData, err := registry_models.ToMap(cfg)
	if err != nil {
		return nil, err
	}

	// Added boostrap data
	bootstrapMapData := map[string]interface{}{}
	switch modeSpecificFields := r.ModeSpecificFields.(type) {
	case ProxyModeRegistryData:
		proxyMapData := map[string]interface{}{}
		var err error

		proxyMapData["upstreamRegistryData"], err = modeSpecificFields.UpstreamRegistryData.ConvertToMap()
		if err != nil {
			return nil, err
		}
		proxyMapData["internalRegistryTTL"] = modeSpecificFields.InternalRegistryTTL.String()
		proxyMapData["internalRegistryPKI"], err = modeSpecificFields.InternalRegistryPKI.convertToMap()
		if err != nil {
			return nil, err
		}
		proxyMapData["internalRegistryData"], err = r.Data.ConvertToMap()
		if err != nil {
			return nil, err
		}
		bootstrapMapData = proxyMapData
	case DetachedModeRegistryData:
		detachedMapData := map[string]interface{}{}
		var err error

		detachedMapData["internalRegistryPKI"], err = modeSpecificFields.InternalRegistryPKI.convertToMap()
		if err != nil {
			return nil, err
		}
		detachedMapData["internalRegistryData"], err = r.Data.ConvertToMap()
		if err != nil {
			return nil, err
		}
		bootstrapMapData = detachedMapData
	}
	if len(bootstrapMapData) > 0 {
		mapData["bootstrap"] = bootstrapMapData
	}
	return mapData, nil
}

func (rData *RegistryData) ConvertToMap() (map[string]interface{}, error) {
	ret := map[string]interface{}{
		"address":   rData.Address,
		"path":      rData.Path,
		"scheme":    rData.Scheme,
		"ca":        rData.CA,
		"dockerCfg": rData.DockerCfg,
	}
	if rData.DockerCfg != "" {
		auth, err := rData.Auth()
		if err != nil {
			return nil, err
		}

		ret["auth"] = auth
	}
	return ret, nil
}

func (r *RegistryData) Auth() (string, error) {
	type dockerCfg struct {
		Auths map[string]struct {
			Auth     string `json:"auth"`
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}

	var (
		registryAuth string
		dc           dockerCfg
	)

	bytes, err := base64.StdEncoding.DecodeString(r.DockerCfg)
	if err != nil {
		return "", fmt.Errorf("cannot base64 decode docker cfg: %v", err)
	}

	log.DebugF("parse registry data: dockerCfg after base64 decode = %s\n", bytes)
	err = json.Unmarshal(bytes, &dc)
	if err != nil {
		return "", fmt.Errorf("cannot unmarshal docker cfg: %v", err)
	}

	if registry, ok := dc.Auths[r.Address]; ok {
		switch {
		case registry.Auth != "":
			registryAuth = registry.Auth
		case registry.Username != "" && registry.Password != "":
			auth := fmt.Sprintf("%s:%s", registry.Username, registry.Password)
			registryAuth = base64.StdEncoding.EncodeToString([]byte(auth))
		default:
			log.DebugF("auth or username with password not found in dockerCfg %s for %s. Use empty string", bytes, r.Address)
		}
	}

	return registryAuth, nil
}

func (r *UpstreamRegistryData) ToRegistryData() (registryData RegistryData, err error) {
	var dockerCfg string
	dockerCfg, err = generateDockerCfgBase64(r.Username, r.Password, r.Address)
	if err != nil {
		err = fmt.Errorf("failed to convert upstream registry to registry data struct: %w", err)
		return
	}
	registryData = RegistryData{
		Address:   r.Address,
		Path:      r.Path,
		Scheme:    r.Scheme,
		CA:        r.CA,
		DockerCfg: dockerCfg,
	}
	return
}

func (rData *UpstreamRegistryData) ConvertToMap() (map[string]interface{}, error) {
	data := map[string]interface{}{
		"address":  rData.Address,
		"path":     rData.Path,
		"scheme":   rData.Scheme,
		"ca":       rData.CA,
		"username": rData.Username,
		"password": rData.Password,
	}
	return data, nil
}

func (rData ProxyModeRegistryData) DeepCopy() ProxyModeRegistryData {
	return ProxyModeRegistryData{
		UpstreamRegistryData: rData.UpstreamRegistryData,
		InternalRegistryTTL:  rData.InternalRegistryTTL,
		InternalRegistryPKI:  rData.InternalRegistryPKI.DeepCopy(),
	}
}

func (rData DetachedModeRegistryData) DeepCopy() DetachedModeRegistryData {
	return DetachedModeRegistryData{
		RegistryPath:        rData.RegistryPath,
		ImagesBundlePath:    rData.ImagesBundlePath,
		InternalRegistryPKI: rData.InternalRegistryPKI.DeepCopy(),
	}
}

func (d RegistryPKI) DeepCopy() RegistryPKI {
	return RegistryPKI{
		CA:     d.CA,
		UserRW: d.UserRW,
		UserRO: d.UserRO,
	}
}

func (d *RegistryPKI) convertToMap() (map[string]interface{}, error) {
	return map[string]interface{}{
		"ca": map[string]interface{}{
			"cert": d.CA.Cert,
			"key":  d.CA.Key,
		},
		"userRW": map[string]interface{}{
			"name":         d.UserRW.Name,
			"password":     d.UserRW.Password,
			"passwordHash": d.UserRW.PasswordHash,
		},
		"userRO": map[string]interface{}{
			"name":         d.UserRO.Name,
			"password":     d.UserRO.Password,
			"passwordHash": d.UserRO.PasswordHash,
		},
	}, nil
}

func getRegistryPKI() (*RegistryPKI, error) {
	registryPKICacheName := "registry-pki"
	inCache, err := cache.Global().InCache(registryPKICacheName)
	if err != nil {
		return nil, err
	}
	if inCache {
		var registryPKI RegistryPKI
		err := cache.Global().LoadStruct(registryPKICacheName, &registryPKI)
		return &registryPKI, err
	} else {
		certKey, err := registry_pki.GenerateCACertificate(registry_pki.CACommonName)
		if err != nil {
			return nil, err
		}
		cert, key, err := registry_pki.EncodeCertKey(certKey)
		if err != nil {
			return nil, err
		}

		userRW, err := registry_pki.NewUser(registry_pki.UserRW)
		if err != nil {
			return nil, err
		}

		userRO, err := registry_pki.NewUser(registry_pki.UserRO)
		if err != nil {
			return nil, err
		}

		registryPKI := RegistryPKI{
			CA:     CertKey{Cert: string(cert), Key: string(key)},
			UserRW: userRW,
			UserRO: userRO,
		}
		err = cache.Global().SaveStruct(registryPKICacheName, registryPKI)
		return &registryPKI, err
	}
}

// getRegistryAddressAndPathFromImagesRepo returns the registry address and path from the given image repository.
func getRegistryAddressAndPathFromImagesRepo(imgRepo string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(strings.TrimRight(imgRepo, "/")), "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], "/" + parts[1]
}

// generateDockerCfgBase64 creates a base64-encoded Docker config.json with credentials for a given registry.
func generateDockerCfgBase64(username, password, registryAddress string) (string, error) {
	// Create the "auth" field by base64-encoding "username:password"
	authString := fmt.Sprintf("%s:%s", username, password)
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(authString))

	// Build Docker config JSON structure
	type authEntry struct {
		Auth     string `json:"auth"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	type dockerCfg struct {
		Auths map[string]authEntry `json:"auths"`
	}

	cfg := dockerCfg{
		Auths: map[string]authEntry{
			registryAddress: {
				Auth:     encodedAuth,
				Username: username,
				Password: password,
			},
		},
	}

	// Convert the config structure to JSON
	jsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal DockerCfg JSON: %w", err)
	}

	// Encode the JSON to a base64 string
	return base64.StdEncoding.EncodeToString(jsonBytes), nil
}

func validateRegistryDockerCfg(cfg string, repo string) error {
	if cfg == "" {
		return fmt.Errorf("can't be empty")
	}

	regcrd, err := base64.StdEncoding.DecodeString(cfg)
	if err != nil {
		return fmt.Errorf("unable to decode registryDockerCfg: %w", err)
	}

	var creds struct {
		Auths map[string]interface{} `json:"auths"`
	}

	if err = json.Unmarshal(regcrd, &creds); err != nil {
		return fmt.Errorf("unable to unmarshal docker credentials: %w", err)
	}

	// The regexp match string with this pattern:
	// ^([a-z]|\d)+ - string starts with a [a-z] letter or a number
	// (\.?|\-?) - next symbol might be '.' or '-' and repeated zero or one times
	// (([a-z]|\d)+(\.|\-|))* - middle part of string might have [a-z] letters, numbers, '.' or ':',
	// and moreover '.' or ':' symbols can't be doubled or goes next to each other
	// ([a-z]|\d+|([a-z]|\d)\:\d+)$ - string might be ended by [a-z] letter or number (if we have single host) or
	// [a-z] letter or number with ':' symbol, and moreover there might be only numbers after ':' symbol
	regx, err := regexp.Compile(`^([a-z]|\d)+(\.?|\-?)(([a-z]|\d)+(\.|\-|))*([a-z]|\d+|([a-z]|\d)\:\d+)$`)
	if err != nil {
		return fmt.Errorf("unable to compile regexp by pattern: %w", err)
	}

	for k := range creds.Auths {
		if !regx.MatchString(k) {
			return fmt.Errorf("invalid registryDockerCfg. Your auths host \"%s\" should be similar to \"your.private.registry.example.com\"", k)
		}
	}

	for k := range creds.Auths {
		if k == repo {
			return nil
		}
	}
	return fmt.Errorf("incorrect registryDockerCfg. It must contain auths host {\"auths\": { \"%s\": {}}}", repo)
}
