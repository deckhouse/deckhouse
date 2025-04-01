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

package registry

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type RegistryStateController = registryStateController

var _ reconcile.Reconciler = &registryStateController{}

type registryStateController struct {
	Namespace      string
	client         client.Client
	eventRecorder  record.EventRecorder
	registryDataCh chan RegistryDataWithHash
}

func (sc *registryStateController) SetupWithManager(ctx context.Context, mgr ctrl.Manager) (chan RegistryDataWithHash, error) {
	controllerName := "registry-state-controller"
	sc.client = mgr.GetClient()
	sc.eventRecorder = mgr.GetEventRecorderFor(controllerName)
	sc.registryDataCh = make(chan RegistryDataWithHash)

	secretsPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != sc.Namespace {
			return false
		}
		name := obj.GetName()
		return name == DeckhouseRegistrySecretName ||
			name == RegistryBashibleConfigSecretName
	})

	err := ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&corev1.Secret{}, builder.WithPredicates(secretsPredicate)).
		Complete(sc)
	if err != nil {
		return sc.registryDataCh, fmt.Errorf("cannot build controller: %w", err)
	}
	return sc.registryDataCh, nil
}

func (sc *registryStateController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	deckhouseRegistrySecret, err := sc.loadDeckhouseRegistrySecrets(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot load deckhouse registry secrets: %w", err)
	}

	registryBashibleConfigSecret, err := sc.loadRegistryBashibleConfigSecret(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot load registry bashible config secrets: %w", err)
	}

	registryData := RegistryData{}
	err = registryData.FromInputData(deckhouseRegistrySecret, registryBashibleConfigSecret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot create registry data: %w", err)
	}

	err = registryData.Validate()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("validation error for registry data: %w", err)
	}

	hashSum, err := registryData.hashSum()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("hash sum generate error for registry data: %w", err)
	}

	sc.registryDataCh <- RegistryDataWithHash{HashSum: hashSum, RegistryData: registryData}
	return ctrl.Result{}, nil
}

func (sc *registryStateController) loadDeckhouseRegistrySecrets(ctx context.Context) (deckhouseRegistry, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      DeckhouseRegistrySecretName,
		Namespace: sc.Namespace,
	}

	if err := sc.client.Get(ctx, key, &secret); err != nil {
		return deckhouseRegistry{}, fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	ret := deckhouseRegistry{}
	if err := ret.DecodeSecret(&secret); err != nil {
		return deckhouseRegistry{}, fmt.Errorf("cannot decode from secret: %w", err)
	}

	if err := ret.Validate(); err != nil {
		return deckhouseRegistry{}, fmt.Errorf("validation error: %w", err)
	}

	return ret, nil
}

func (sc *registryStateController) loadRegistryBashibleConfigSecret(ctx context.Context) (*registryBashibleConfig, error) {
	secret := corev1.Secret{}
	key := types.NamespacedName{
		Name:      RegistryBashibleConfigSecretName,
		Namespace: sc.Namespace,
	}

	if err := sc.client.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot get secret %v k8s object: %w", key.Name, err)
	}

	ret := registryBashibleConfig{}
	if err := ret.DecodeSecret(&secret); err != nil {
		return nil, fmt.Errorf("cannot decode from secret: %w", err)
	}

	if err := ret.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	return &ret, nil
}

type RegistryDataWithHash struct {
	HashSum      string
	RegistryData RegistryData
}

type RegistryData struct {
	// For backward compatibility
	Address      string `json:"address" yaml:"address"`
	Path         string `json:"path" yaml:"path"`
	Scheme       string `json:"scheme" yaml:"scheme"`
	CA           string `json:"ca" yaml:"ca"`
	DockerCFG    []byte `json:"dockerCfg" yaml:"dockerCfg"`
	Auth         string `json:"auth" yaml:"auth"`
	RegistryMode string `json:"registryMode" yaml:"registryMode"`

	Mode           string                `json:"mode" yaml:"mode"`
	ImagesBase     string                `json:"imagesBase" yaml:"imagesBase"`
	Version        string                `json:"version" yaml:"version"`
	ProxyEndpoints []string              `json:"proxyEndpoints" yaml:"proxyEndpoints"`
	Hosts          []RegistryHostsObject `json:"hosts" yaml:"hosts"`
	PrepullHosts   []RegistryHostsObject `json:"prepullHosts" yaml:"prepullHosts"`
}

type RegistryHostsObject struct {
	Host    string                     `json:"host" yaml:"host"`
	CA      []string                   `json:"ca" yaml:"ca"`
	Mirrors []RegistryMirrorHostObject `json:"mirrors" yaml:"mirrors"`
}

type RegistryMirrorHostObject struct {
	Host     string `json:"host" yaml:"host"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
	Scheme   string `json:"scheme" yaml:"scheme"`
}

// FromInputData populates RegistryData fields from given registry configuration
func (d *RegistryData) FromInputData(deckhouseRegistry deckhouseRegistry, registryBashibleConfig *registryBashibleConfig) error {
	d.Mode = "Unmanaged"
	d.ImagesBase = fmt.Sprintf(
		"%s/%s",
		deckhouseRegistry.Address,
		strings.TrimLeft(deckhouseRegistry.Path, "/"),
	)
	d.Version = "unknown"
	d.ProxyEndpoints = []string{}
	d.Hosts = []RegistryHostsObject{}
	d.PrepullHosts = []RegistryHostsObject{}

	// Replace, if registryBashibleConfig exist
	if registryBashibleConfig != nil {
		d.Mode = registryBashibleConfig.Mode
		d.ImagesBase = registryBashibleConfig.ImagesBase
		d.Version = registryBashibleConfig.Version
		d.ProxyEndpoints = slices.Clone(registryBashibleConfig.ProxyEndpoints)
		d.Hosts = slices.Clone(registryBashibleConfig.Hosts)
		d.PrepullHosts = slices.Clone(registryBashibleConfig.PrepullHosts)
	}

	deckhouseRegistryAuth, err := deckhouseRegistry.Auth()
	if err != nil {
		return err
	}

	deckhouseRegistryMirrorHost := RegistryMirrorHostObject{
		Host:   deckhouseRegistry.Address,
		Auth:   deckhouseRegistryAuth,
		Scheme: deckhouseRegistry.Scheme,
	}

	// Append Mirrors, if deckhouseRegistry.Address not exist in Mirrors list
	if !slices.ContainsFunc(d.Hosts, func(host RegistryHostsObject) bool {
		return host.Host == deckhouseRegistry.Address
	}) {
		d.Hosts = append(d.Hosts, RegistryHostsObject{
			Host:    deckhouseRegistry.Address,
			CA:      []string{deckhouseRegistry.CA},
			Mirrors: []RegistryMirrorHostObject{deckhouseRegistryMirrorHost},
		})
		d.PrepullHosts = append(d.PrepullHosts, RegistryHostsObject{
			Host:    deckhouseRegistry.Address,
			CA:      []string{deckhouseRegistry.CA},
			Mirrors: []RegistryMirrorHostObject{deckhouseRegistryMirrorHost},
		})
	}

	// For backward compatibility
	d.Address = deckhouseRegistry.Address
	d.Path = deckhouseRegistry.Path
	d.Scheme = deckhouseRegistry.Scheme
	d.CA = deckhouseRegistry.CA
	d.DockerCFG = deckhouseRegistry.DockerConfig
	d.Auth = deckhouseRegistryAuth
	d.RegistryMode = deckhouseRegistry.RegistryMode
	return nil
}

func (d *RegistryData) Validate() error {
	// Implementation of validation logic if needed
	return nil
}

func (d *RegistryData) hashSum() (string, error) {
	rawData, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("error marshalling data: %w", err)
	}

	hash := sha256.New()
	_, err = hash.Write(rawData)
	if err != nil {
		return "", fmt.Errorf("error generating hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
