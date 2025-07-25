/*
Copyright 2023 Flant JSC

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

package template

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"sort"

	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

type ContextBuilder struct {
	ctx context.Context

	stepsStorage *StepsStorage

	registryData     map[string]interface{}
	clusterInputData inputData
	versionMap       map[string]interface{}
	imagesDigests    map[string]map[string]string // module: { image_name: tag }

	// debug function injection
	emitStepsOutput func(string, map[string]string)

	moduleSourcesCA        map[string]string
	nodeUserConfigurations map[string][]*UserConfiguration
}

func NewContextBuilder(ctx context.Context, stepsStorage *StepsStorage) *ContextBuilder {
	cb := &ContextBuilder{
		ctx:                    ctx,
		stepsStorage:           stepsStorage,
		nodeUserConfigurations: make(map[string][]*UserConfiguration),
		moduleSourcesCA:        make(map[string]string),
	}

	return cb
}

type bashibleContexts map[string]interface{}

type BashibleContextData struct {
	bashibleContexts // rendered contexts

	Images     map[string]map[string]string `json:"images" yaml:"images"`
	VersionMap map[string]interface{}       `json:"versionMap" yaml:"versionMap"`
	Registry   map[string]interface{}       `json:"registry" yaml:"registry"`
	Proxy      map[string]interface{}       `json:"proxy" yaml:"proxy"`
}

// Map returns context data as map[string]interface{} for proper yaml marshalling
func (bd BashibleContextData) Map() map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range bd.bashibleContexts {
		result[key] = value
	}

	result["images"] = bd.Images
	result["registry"] = bd.Registry
	result["versionMap"] = bd.VersionMap
	result["proxy"] = bd.Proxy

	return result
}

func (cb *ContextBuilder) SetRegistryData(rd map[string]interface{}) {
	cb.registryData = rd
}

func (cb *ContextBuilder) SetImagesData(data map[string]map[string]string) {
	cb.imagesDigests = data
}

func (cb *ContextBuilder) SetVersionMapData(data map[string]interface{}) {
	cb.versionMap = data
}

func (cb *ContextBuilder) SetInputData(data inputData) {
	sort.Strings(data.AllowedBundles)
	sort.Slice(data.NodeGroups, func(i, j int) bool {
		ngi := data.NodeGroups[i]
		ngj := data.NodeGroups[j]

		return ngi.Name() < ngj.Name()
	})
	cb.clusterInputData = data
}

func (cb *ContextBuilder) setStepsOutput(f func(string, map[string]string)) {
	cb.emitStepsOutput = f
}

func (cb *ContextBuilder) getCloudProvider() string {
	if cb.clusterInputData.CloudProvider == nil {
		// absent cloud provider means static nodes
		return ""
	}

	cloudProvider, ok := cb.clusterInputData.CloudProvider.(map[string]interface{})
	if !ok {
		return ""
	}
	providerType, ok := cloudProvider["type"].(string)
	if !ok {
		return ""
	}
	return providerType
}

func (cb *ContextBuilder) Build() (BashibleContextData, map[string][]byte, map[string]error) {
	bb := BashibleContextData{
		bashibleContexts: make(map[string]interface{}),
		VersionMap:       cb.versionMap,
		Images:           cb.imagesDigests,
		Registry:         cb.registryData,
		Proxy:            cb.clusterInputData.Proxy,
	}

	ngMap := make(map[string][]byte)
	errorsMap := make(map[string]error)
	hashMap := make(map[string]hash.Hash, len(cb.clusterInputData.NodeGroups))

	commonContext := &tplContextCommon{
		versionMapWrapper: versionMapFromMap(cb.versionMap),
		RunType:           "Normal",
		Normal: normal{
			ClusterDomain:      cb.clusterInputData.ClusterDomain,
			ClusterDNSAddress:  cb.clusterInputData.ClusterDNSAddress,
			BootstrapTokens:    cb.clusterInputData.BootstrapTokens,
			ApiserverEndpoints: cb.clusterInputData.APIServerEndpoints,
			KubernetesCA:       cb.clusterInputData.KubernetesCA,
			ModuleSourcesCA:    cb.moduleSourcesCA,
		},
		Registry:      cb.registryData,
		Images:        cb.imagesDigests,
		Proxy:         cb.clusterInputData.Proxy,
		PackagesProxy: cb.clusterInputData.PackagesProxy,
	}

	for _, ng := range cb.clusterInputData.NodeGroups {
		checksumCollector, ok := hashMap[ng.Name()]
		if !ok {
			checksumCollector = sha256.New()
			hashMap[ng.Name()] = checksumCollector
		}
		bundleContextName := fmt.Sprintf("bundle-%s", ng.Name())
		bashibleContextName := fmt.Sprintf("bashible-%s", ng.Name())
		bundleNgContext := cb.newBundleNGContext(ng, cb.clusterInputData.Freq, cb.clusterInputData.CloudProvider, commonContext)
		bb.bashibleContexts[bundleContextName] = bundleNgContext

		bashibleContext, err := cb.newBashibleContext(checksumCollector, ng, cb.versionMap, &bundleNgContext)
		if err != nil {
			errorsMap[bundleContextName] = err
		}
		// bashibleContext always exists. Err is only for checksum generation
		bb.bashibleContexts[bashibleContextName] = bashibleContext

	}

	for _, ng := range cb.clusterInputData.NodeGroups {
		bashibleContextName := fmt.Sprintf("bashible-%s", ng.Name())
		checksumCollector := hashMap[ng.Name()]
		checksum := fmt.Sprintf("%x", checksumCollector.Sum(nil))
		bcr := bb.bashibleContexts[bashibleContextName]
		bc := bcr.(bashibleContext)
		bc.ConfigurationChecksum = checksum
		bb.bashibleContexts[bashibleContextName] = bc
		ngMap[ng.Name()] = []byte(checksum)
	}

	return bb, ngMap, errorsMap
}

func (cb *ContextBuilder) newBashibleContext(checksumCollector hash.Hash, ng nodeGroup, versionMap map[string]interface{}, bundleNgContext *bundleNGContext) (bashibleContext, error) {
	bc := bashibleContext{
		KubernetesVersion: ng.KubernetesVersion(),
		Normal: map[string]interface{}{
			"apiserverEndpoints": bundleNgContext.tplContextCommon.Normal.ApiserverEndpoints,
		},
		NodeGroup: ng,
		RunType:   "Normal",

		Images:            cb.imagesDigests,
		Registry:          cb.registryData,
		Proxy:             cb.clusterInputData.Proxy,
		CloudProviderType: cb.getCloudProvider(),
		PackagesProxy:     cb.clusterInputData.PackagesProxy,
	}

	err := cb.generateBashibleChecksum(checksumCollector, bc, bundleNgContext, versionMap)
	if err != nil {
		return bc, errors.Wrap(err, "checksum calc failed")
	}

	return bc, nil
}

func (cb *ContextBuilder) generateBashibleChecksum(checksumCollector hash.Hash, bc bashibleContext, bundleNgContext *bundleNGContext, versionMap map[string]interface{}) error {
	err := bc.AddToChecksum(checksumCollector)
	if err != nil {
		return err
	}

	bcData, err := yaml.Marshal(bc)
	if err != nil {
		return errors.Wrap(err, "marshal bashibleContext failed")
	}

	var res map[string]interface{}

	err = yaml.Unmarshal(bcData, &res)
	if err != nil {
		return errors.Wrap(err, "unmarshal bashibleContext failed")
	}

	// enrich context map with version-map data
	for k, v := range versionMap {
		res[k] = v
	}

	var providerType string
	cloudProvider, ok := bundleNgContext.CloudProvider.(map[string]interface{})
	if ok {
		providerType, ok = cloudProvider["type"].(string)
		if !ok {
			return fmt.Errorf("cloudProvider.type is not a string")
		}
	}

	// render steps
	steps, err := cb.stepsStorage.Render("all", providerType, res)
	if err != nil {
		return errors.Wrap(err, "steps render failed")
	}

	if cb.emitStepsOutput != nil {
		cb.emitStepsOutput(bc.NodeGroup.Name(), steps)
	}

	stepsData, err := yaml.Marshal(steps)
	if err != nil {
		return errors.Wrap(err, "marshal steps failed")
	}

	checksumCollector.Write(stepsData)

	var bundleRes map[string]interface{}

	bundleData, err := yaml.Marshal(bundleNgContext)
	if err != nil {
		return errors.Wrap(err, "marshal bundleNGContext failed")
	}

	err = yaml.Unmarshal(bundleData, &bundleRes)
	if err != nil {
		return errors.Wrap(err, "unmarshal bundleNGContext failed")
	}

	// render ng steps
	ngSteps, err := cb.stepsStorage.Render("all", providerType, bundleRes, bc.NodeGroup.Name())
	if err != nil {
		return errors.Wrap(err, "NG steps render failed")
	}

	if cb.emitStepsOutput != nil {
		cb.emitStepsOutput(bc.NodeGroup.Name(), ngSteps)
	}

	ngStepsData, err := yaml.Marshal(ngSteps)
	if err != nil {
		return errors.Wrap(err, "marshal NG steps failed")
	}

	checksumCollector.Write(ngStepsData)

	return nil
}

func (cb *ContextBuilder) newBundleNGContext(ng nodeGroup, freq interface{}, cloudProvider interface{}, contextCommon *tplContextCommon) bundleNGContext {
	return bundleNGContext{
		tplContextCommon:          contextCommon,
		KubernetesVersion:         ng.KubernetesVersion(),
		CRI:                       ng.CRIType(),
		NodeGroup:                 ng,
		CloudProvider:             cloudProvider,
		NodeStatusUpdateFrequency: freq,
		NodeUsers:                 cb.getNodeUserConfigurations(ng.Name()),
	}
}

func (cb *ContextBuilder) getNodeUserConfigurations(nodeGroup string) []*UserConfiguration {

	users := make([]*UserConfiguration, 0)
	wildcardBundle := fmt.Sprintf("*:%s", nodeGroup)
	totalWildcard := "*:*"

	users = append(users, cb.nodeUserConfigurations[wildcardBundle]...)
	users = append(users, cb.nodeUserConfigurations[totalWildcard]...)

	sort.SliceStable(users, func(i, j int) bool {
		return users[i].Name < users[j].Name
	})

	return users
}

func versionMapFromMap(m map[string]interface{}) versionMapWrapper {
	var res versionMapWrapper

	if v, ok := m["k8s"]; ok {
		res.K8s = v.(map[string]interface{})
	}

	return res
}

type nodeGroup map[string]interface{}

func (ng nodeGroup) Name() string {
	if name, ok := ng["name"]; ok {
		return name.(string)
	}

	return ""
}

func (ng nodeGroup) KubernetesVersion() string {
	if kv, ok := ng["kubernetesVersion"]; ok {
		return kv.(string)
	}

	return ""
}

func (ng nodeGroup) CRIType() string {
	if cri, ok := ng["cri"]; ok {
		if typ, ok := cri.(map[string]interface{})["type"]; ok {
			return typ.(string)
		}
	}

	return ""
}

type bashibleContext struct {
	ConfigurationChecksum string      `json:"configurationChecksum" yaml:"configurationChecksum"`
	KubernetesVersion     string      `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	Normal                interface{} `json:"normal" yaml:"normal"`
	NodeGroup             nodeGroup   `json:"nodeGroup" yaml:"nodeGroup"`
	RunType               string      `json:"runType" yaml:"runType"` // Normal

	// Enrich with images and registry
	Images            map[string]map[string]string `json:"images" yaml:"images"`
	Registry          map[string]interface{}       `json:"registry" yaml:"registry"`
	Proxy             map[string]interface{}       `json:"proxy" yaml:"proxy"`
	CloudProviderType string                       `json:"cloudProviderType" yaml:"cloudProviderType"`
	PackagesProxy     map[string]interface{}       `json:"packagesProxy" yaml:"packagesProxy"`
}

func (bc *bashibleContext) AddToChecksum(checksumCollector hash.Hash) error {
	cpy := deepcopy.Copy(bc)
	bcCopy := cpy.(*bashibleContext)

	cloudInstancesRaw, ok := bcCopy.NodeGroup["cloudInstances"]
	if ok {
		cloudInstances, ok := cloudInstancesRaw.(map[string]interface{})
		if ok {
			// remove counter for prevent updating nodes while upscale
			delete(cloudInstances, "maxPerZone")
			delete(cloudInstances, "maxSurgePerZone")
			delete(cloudInstances, "maxUnavailablePerZone")
			delete(cloudInstances, "minPerZone")
			delete(cloudInstances, "zones")
		}
	}

	bcData, err := yaml.Marshal(bcCopy)
	if err != nil {
		return errors.Wrap(err, "marshal bashibleContext for checksum collect failed")
	}

	checksumCollector.Write(bcData)

	return nil
}

// for appropriate marshalling
type versionMapWrapper struct {
	K8s map[string]interface{} `json:"k8s" yaml:"k8s"`
}

type tplContextCommon struct {
	versionMapWrapper

	RunType string `json:"runType" yaml:"runType"`
	Normal  normal `json:"normal" yaml:"normal"`

	Images   map[string]map[string]string `json:"images" yaml:"images"`
	Registry map[string]interface{}       `json:"registry" yaml:"registry"`

	Proxy         map[string]interface{} `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	PackagesProxy map[string]interface{} `json:"packagesProxy,omitempty" yaml:"packagesProxy,omitempty"`
}

type bundleNGContext struct {
	*tplContextCommon

	KubernetesVersion string      `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CRI               string      `json:"cri" yaml:"cri"`
	NodeGroup         nodeGroup   `json:"nodeGroup" yaml:"nodeGroup"`
	CloudProvider     interface{} `json:"cloudProvider,omitempty" yaml:"cloudProvider,omitempty"`

	NodeStatusUpdateFrequency interface{} `json:"nodeStatusUpdateFrequency,omitempty" yaml:"nodeStatusUpdateFrequency,omitempty"`

	NodeUsers []*UserConfiguration `json:"nodeUsers" yaml:"nodeUsers"`
}

type bundleK8sVersionContext struct {
	*tplContextCommon

	KubernetesVersion string `json:"kubernetesVersion" yaml:"kubernetesVersion"`

	CloudProvider interface{} `json:"cloudProvider,omitempty" yaml:"cloudProvider,omitempty"`
}

type normal struct {
	ClusterDomain      string            `json:"clusterDomain" yaml:"clusterDomain"`
	ClusterDNSAddress  string            `json:"clusterDNSAddress" yaml:"clusterDNSAddress"`
	BootstrapTokens    map[string]string `json:"bootstrapTokens" yaml:"bootstrapTokens"`
	ApiserverEndpoints []string          `json:"apiserverEndpoints" yaml:"apiserverEndpoints"`
	KubernetesCA       string            `json:"kubernetesCA" yaml:"kubernetesCA"`
	ModuleSourcesCA    map[string]string `json:"moduleSourcesCA" yaml:"moduleSourcesCA"`
}

type inputData struct {
	ClusterDomain      string                 `json:"clusterDomain" yaml:"clusterDomain"`
	ClusterDNSAddress  string                 `json:"clusterDNSAddress" yaml:"clusterDNSAddress"`
	CloudProvider      interface{}            `json:"cloudProvider,omitempty" yaml:"cloudProvider,omitempty"`
	Proxy              map[string]interface{} `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	BootstrapTokens    map[string]string      `json:"bootstrapTokens,omitempty" yaml:"bootstrapTokens,omitempty"`
	PackagesProxy      map[string]interface{} `json:"packagesProxy,omitempty" yaml:"packagesProxy,omitempty"`
	APIServerEndpoints []string               `json:"apiserverEndpoints" yaml:"apiserverEndpoints"`
	KubernetesCA       string                 `json:"kubernetesCA" yaml:"kubernetesCA"`
	AllowedBundles     []string               `json:"allowedBundles" yaml:"allowedBundles"`
	NodeGroups         []nodeGroup            `json:"nodeGroups" yaml:"nodeGroups"`
	Freq               interface{}            `json:"NodeStatusUpdateFrequency,omitempty" yaml:"NodeStatusUpdateFrequency,omitempty"`
}
