/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	bashible "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
)

type Inputs struct {
	IsSecretExist  bool
	MasterNodesIPs []string
	NodeStatus     map[string]InputsNodeVersion
}

type InputsNodeVersion = string

type Params struct {
	Mode           registry_const.ModeType
	RegistrySecret deckhouse_registry.Config

	ProxyLocal *ProxyLocalModeParams
	Unmanaged  *UnmanagedModeParams
	Direct     *DirectModeParams
}

type ProxyLocalModeParams struct {
	CA       string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

type UnmanagedModeParams struct {
	ImagesRepo string `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     string `json:"scheme" yaml:"scheme"`
	CA         string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string `json:"username" yaml:"username"`
	Password   string `json:"password" yaml:"password"`
}

type DirectModeParams struct {
	ImagesRepo string `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     string `json:"scheme" yaml:"scheme"`
	CA         string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string `json:"username" yaml:"username"`
	Password   string `json:"password" yaml:"password"`
}

type State struct {
	Config *StateConfig `json:"config,omitempty" yaml:"config,omitempty"`
}

type StateConfig struct {
	ActualParams ActualParams `json:"actual_params,omitempty" yaml:"actual_params,omitempty"`
	Config       Config       `json:"config,omitempty" yaml:"config,omitempty"`
}

type Config bashible.Config

type ActualParams struct {
	Mode           registry_const.ModeType `json:"mode" yaml:"mode"`
	ImagesBase     string                  `json:"imagesBase" yaml:"imagesBase"`
	MasterNodesIPs []string                `json:"masterNodesIPs,omitempty" yaml:"masterNodesIPs,omitempty"`

	// ProxyLocal contains parameters used in Proxy and Local modes.
	//
	// These values remain unchanged when switching between Proxy ↔ Local,
	// unless regenerated due to external events, such as PKI re-issuance.
	//
	// For example, if PKI is regenerated:
	//   1. A new password is generated for the RO user;
	//   2. A new CA certificate is issued;
	//   3. The static Pod (registry) is restarted;
	//   4. Bashible is restarted.
	//
	// Since the registry uses the new PKI, old values are no longer usable.
	// Therefore, maintaining an array of mirror (contained's fallback) (e.g. [old, new]) is unnecessary.
	// Especially when the first mirror is derived from outdated credentials — it will simply fail.
	ProxyLocal *ProxyLocalModeParams `json:"proxyLocal,omitempty" yaml:"proxyLocal,omitempty"`

	// Unmanaged contains parameters used in unmanaged registry mode.
	//
	// This field is populated only in two cases:
	//   1. Switching from Unmanaged to another mode — values come from the deckhouse-registry Secret;
	//   2. Switching from another mode to Unmanaged — values come from previously used ActualParams.
	//
	// Since switching from Unmanaged → Unmanaged is not supported, there's no need to maintain a list of values.
	//
	// We do not support the case where users manually modify the deckhouse-registry Secret
	// during transitions between Unmanaged and other modes.
	Unmanaged *UnmanagedModeParams `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`

	// Direct keeps a list of parameter sets used in Direct mode.
	//
	// Direct→Direct transitions are allowed and can involve significant changes.
	// To support mirrors (contained's fallback), we store multiple configurations (old and new).
	//
	// Each DirectModeParams entry represents a full, immutable configuration snapshot.
	// Merging partial changes (e.g. only updating path or credentials) is not supported.
	// For example, if only the path changes but the user also changes,
	// that must result in a completely new mirror (contained's fallback).
	Direct []DirectModeParams `json:"direct,omitempty" yaml:"direct,omitempty"`
}

type Result struct {
	Ready   bool
	Message string
}

func (state *State) IsStopped() bool {
	return state == nil || state.Config == nil
}

func (state *State) Stop(inputs Inputs) Result {
	state.Config = nil

	if inputs.IsSecretExist {
		return Result{
			Ready:   false,
			Message: "Bashible secret exists. Deleting...",
		}
	}

	var msg strings.Builder
	msg.WriteString("Status:\n")
	ready := true
	for nodeName, nodeVersion := range inputs.NodeStatus {
		nodeReady := nodeVersion == registry_const.UnknownVersion
		msg.WriteString(fmt.Sprintf(" - node: %q, ready: %t\n", nodeName, nodeReady))
		ready = ready && nodeReady
	}

	return Result{
		Ready:   ready,
		Message: msg.String(),
	}
}

func (state *State) Process(params Params, inputs Inputs, withActual bool) (Result, error) {
	if state.Config == nil {
		state.Config = &StateConfig{}
	}

	if err := state.Config.process(params, inputs, withActual); err != nil {
		return Result{
			Ready:   false,
			Message: "Configuration processing for Bashible failed.",
		}, fmt.Errorf("cannot process config: %w", err)
	}

	if !inputs.IsSecretExist {
		return Result{
			Ready:   false,
			Message: "Bashible secret is not deployed. Proceeding...",
		}, nil
	}

	var msg strings.Builder
	msg.WriteString("Status:\n")
	ready := true
	for nodeName, nodeVersion := range inputs.NodeStatus {
		nodeReady := nodeVersion == state.Config.Config.Version
		msg.WriteString(fmt.Sprintf(" - node: %q, ready: %t\n", nodeName, nodeReady))
		ready = ready && nodeReady
	}

	return Result{
		Ready:   ready,
		Message: msg.String(),
	}, nil
}

// process builds the Bashible configuration based on the current and new registry parameters.
//
// Bashible uses a two-phase rollout process for safe configuration transition:
//  1. First phase uses withActual = true, to keep old and new registry parameters together.
//  2. Second phase uses withActual = false, to fully switch to the new configuration.
//
// If ActualParams is empty, it is initialized from the deckhouse-registry Secret before processing.
func (cfg *StateConfig) process(params Params, inputs Inputs, withActual bool) error {
	switch {
	case params.Mode == registry_const.ModeUnmanaged &&
		params.Unmanaged == nil:
		return fmt.Errorf("missing unmanaged parameters for mode %s", params.Mode)

	case (params.Mode == registry_const.ModeLocal ||
		params.Mode == registry_const.ModeProxy) &&
		params.ProxyLocal == nil:
		return fmt.Errorf("missing proxy/local parameters for mode %s", params.Mode)

	case params.Mode == registry_const.ModeDirect &&
		params.Direct == nil:
		return fmt.Errorf("missing direct parameters for mode %s", params.Mode)
	}

	if cfg.ActualParams.isEmpty() {
		if err := cfg.ActualParams.fromRegistrySecret(params.RegistrySecret, inputs.MasterNodesIPs); err != nil {
			return err
		}
	}

	newParams := ActualParams{
		Mode:           params.Mode,
		ImagesBase:     registry_const.HostWithPath,
		MasterNodesIPs: inputs.MasterNodesIPs,
		ProxyLocal:     params.ProxyLocal,
		Unmanaged:      params.Unmanaged,
	}

	if params.Mode == registry_const.ModeUnmanaged {
		newParams.ImagesBase = params.Unmanaged.ImagesRepo
	}

	if params.Direct != nil {
		newParams.Direct = append(newParams.Direct, *params.Direct)
	}

	if withActual {
		cfg.ActualParams.merge(newParams)
	} else {
		cfg.ActualParams.set(newParams)
	}

	newCfg, err := cfg.ActualParams.process()
	if err != nil {
		return fmt.Errorf("failed to process bashible config: %w", err)
	}

	cfg.Config = newCfg
	return nil
}

func (p *ActualParams) process() (Config, error) {
	cfg := Config{
		ImagesBase:   p.ImagesBase,
		Mode:         p.Mode,
		Hosts:        make(map[string]bashible.Hosts),
		PrepullHosts: make(map[string]bashible.Hosts),
	}

	if p.ProxyLocal != nil {
		endpoints, hosts, prepull := processProxyLocal(*p.ProxyLocal, p.MasterNodesIPs)
		cfg.ProxyEndpoints = endpoints
		cfg.mergeHosts(hosts, prepull)
	} else {
		cfg.ProxyEndpoints = []string{}
	}

	if p.Unmanaged != nil {
		h := processUnmanaged(*p.Unmanaged)
		cfg.mergeHosts(h, h)
	}

	for _, d := range p.Direct {
		h := processDirect(d)
		cfg.mergeHosts(h, h)
	}

	hash, err := helpers.ComputeHash(cfg)
	if err != nil {
		return cfg, fmt.Errorf("failed to compute config hash: %w", err)
	}

	cfg.Version = hash
	return cfg, nil
}

func (p ActualParams) isEmpty() bool {
	return p.ProxyLocal == nil && p.Unmanaged == nil && len(p.Direct) == 0
}

func (p *ActualParams) set(newParams ActualParams) {
	*p = newParams
}

func (p *ActualParams) merge(newParams ActualParams) {
	p.MasterNodesIPs = newParams.MasterNodesIPs
	p.ImagesBase = newParams.ImagesBase
	p.Mode = newParams.Mode

	if newParams.ProxyLocal != nil {
		p.ProxyLocal = &ProxyLocalModeParams{}
		*p.ProxyLocal = *newParams.ProxyLocal
	}

	if newParams.Unmanaged != nil {
		p.Unmanaged = &UnmanagedModeParams{}
		*p.Unmanaged = *newParams.Unmanaged
	}

	if p.Direct == nil {
		p.Direct = []DirectModeParams{}
	}
	p.Direct = helpers.DeduplicateSlice(append(p.Direct, newParams.Direct...))
}

func (p *ActualParams) fromRegistrySecret(registrySecret deckhouse_registry.Config, masterNodesIPs []string) error {
	username, password, err := helpers.CredsFromDockerCfg(
		registrySecret.DockerConfig,
		registrySecret.Address,
	)
	if err != nil {
		return fmt.Errorf("failed to extract credentials from Docker config: %w", err)
	}

	hostWithPath := registrySecret.Address + registrySecret.Path
	*p = ActualParams{
		Mode:           registry_const.ModeUnmanaged,
		ImagesBase:     hostWithPath,
		MasterNodesIPs: masterNodesIPs,
		Unmanaged: &UnmanagedModeParams{
			ImagesRepo: hostWithPath,
			Scheme:     strings.ToLower(registrySecret.Scheme),
			CA:         registrySecret.CA,
			Username:   username,
			Password:   password,
		},
	}
	return nil
}

func (cfg *Config) mergeHosts(hosts, prepull map[string]bashible.Hosts) {
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]bashible.Hosts)
	}
	if cfg.PrepullHosts == nil {
		cfg.PrepullHosts = make(map[string]bashible.Hosts)
	}

	for name, h := range hosts {
		old := cfg.Hosts[name]
		old.CA = helpers.DeduplicateAndSortSlice(append(old.CA, h.CA...))
		old.Mirrors = append(old.Mirrors, h.Mirrors...)
		cfg.Hosts[name] = old
	}

	for name, h := range prepull {
		old := cfg.PrepullHosts[name]
		old.CA = helpers.DeduplicateAndSortSlice(append(old.CA, h.CA...))
		old.Mirrors = append(old.Mirrors, h.Mirrors...)
		cfg.PrepullHosts[name] = old
	}
}

func processUnmanaged(params UnmanagedModeParams) map[string]bashible.Hosts {
	host, _ := getRegistryAddressAndPathFromImagesRepo(params.ImagesRepo)
	return map[string]bashible.Hosts{
		host: {
			CA: singleCA(params.CA),
			Mirrors: []bashible.MirrorHost{
				{
					Host:   host,
					Scheme: strings.ToLower(params.Scheme),
					Auth: bashible.Auth{
						Username: params.Username,
						Password: params.Password,
					},
				},
			},
		},
	}
}

func processDirect(params DirectModeParams) map[string]bashible.Hosts {
	host, path := getRegistryAddressAndPathFromImagesRepo(params.ImagesRepo)
	return map[string]bashible.Hosts{
		registry_const.Host: {
			CA: singleCA(params.CA),
			Mirrors: []bashible.MirrorHost{
				{
					Host:   host,
					Scheme: strings.ToLower(params.Scheme),
					Auth: bashible.Auth{
						Username: params.Username,
						Password: params.Password,
					},
					Rewrites: []bashible.Rewrite{{
						From: registry_const.PathRegexp,
						To:   strings.TrimLeft(path, "/"),
					}},
				},
			},
		},
	}
}

func processProxyLocal(params ProxyLocalModeParams, masterNodesIPs []string) ([]string, map[string]bashible.Hosts, map[string]bashible.Hosts) {
	makeMirrors := func(hosts []string) []bashible.MirrorHost {
		mirrors := make([]bashible.MirrorHost, 0, len(hosts))
		for _, h := range hosts {
			mirrors = append(mirrors, bashible.MirrorHost{
				Host:   h,
				Scheme: registry_const.Scheme,
				Auth: bashible.Auth{
					Username: params.Username,
					Password: params.Password,
				},
			})
		}
		return mirrors
	}

	endpoints := registry_const.GenerateProxyEndpoints(masterNodesIPs)
	ca := singleCA(params.CA)
	hosts := map[string]bashible.Hosts{
		registry_const.Host: {
			CA:      ca,
			Mirrors: makeMirrors([]string{registry_const.ProxyHost}),
		},
	}
	prepullHosts := map[string]bashible.Hosts{
		registry_const.Host: {
			CA:      ca,
			Mirrors: makeMirrors(append([]string{registry_const.ProxyHost}, endpoints...)),
		},
	}
	return endpoints, hosts, prepullHosts
}

func singleCA(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}
