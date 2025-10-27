/*
Copyright 2025 Flant JSC

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

package bashible

import (
	"fmt"
	"slices"
	"strings"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_docker "github.com/deckhouse/deckhouse/go_lib/registry/docker"
	bashible "github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	transitionMessage     = "Applying configuration to nodes"
	preflightCheckMessage = "Check current nodes configuration"
)

var (
	failedResult = Result{
		Ready:   false,
		Message: fmt.Sprintf("%s\nFailed to process Bashible configuration.", transitionMessage),
	}

	successResult = Result{
		Ready:   true,
		Message: fmt.Sprintf("%s\nBashible already processed.", transitionMessage),
	}
)

type Inputs struct {
	IsSecretExist  bool
	MasterNodesIPs []string
	NodeStatus     map[string]InputsNodeStatus
}

type InputsNodeStatus struct {
	Version           string
	ContainerdCfgMode string
}

type Params struct {
	ModeParams     ModeParams
	RegistrySecret deckhouse_registry.Config
}

type State struct {
	UnmanagedParams *UnmanagedModeParams `json:"unmanaged_params,omitempty" yaml:"unmanaged_params,omitempty"`
	ActualParams    *ModeParams          `json:"actual_params,omitempty" yaml:"actual_params,omitempty"`
	Config          *Config              `json:"config,omitempty" yaml:"config,omitempty"`
}

type ConfigBuilder struct {
	ModeParams     ModeParams
	ActualParams   []ModeParams
	MasterNodesIPs []string
}

type Config bashible.Config

type ModeParams struct {
	Direct    *DirectModeParams    `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *UnmanagedModeParams `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
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

type Result struct {
	Ready   bool
	Message string
}

func PreflightCheck(input Inputs) Result {
	var msg strings.Builder
	fmt.Fprintln(&msg, preflightCheckMessage)

	total := len(input.NodeStatus)
	if input.IsSecretExist {
		fmt.Fprintln(&msg, "Configuration from registry module already exists.")
		fmt.Fprintf(&msg, "All %d node(s) Ready to configure.\n", total)
		return Result{Ready: true, Message: msg.String()}
	}

	var pending []string
	for nodeName, status := range input.NodeStatus {
		if status.ContainerdCfgMode != containerdCfgModeDefault {
			pending = append(pending, nodeName)
		}
	}

	if len(pending) == 0 {
		fmt.Fprintf(&msg, "All %d node(s) Ready to configure.\n", total)
		return Result{Ready: true, Message: msg.String()}
	}

	fmt.Fprintf(&msg, "%d/%d node(s) Unready:\n", len(pending), total)

	slices.Sort(pending)
	const maxShown = 10
	for i, name := range pending {
		if i == maxShown {
			remaining := len(pending) - maxShown
			fmt.Fprintf(&msg, "\t...and %d more\n", remaining)
			break
		}

		switch input.NodeStatus[name].ContainerdCfgMode {
		case containerdCfgModeCustom:
			fmt.Fprintf(&msg, "- %s: has custom toml merge containerd configuration\n", name)
		default:
			fmt.Fprintf(&msg, "- %s: unknown containerd configuration, waiting...\n", name)
		}
	}
	return Result{Ready: false, Message: msg.String()}
}

// ProcessTransition applies the new configuration alongside the existing one.
// Should be used when registry mode or its parameters change (transition phase).
func (s *State) ProcessTransition(params Params, inputs Inputs) (Result, error) {
	return s.process(params, inputs, true)
}

// FinalizeTransition replaces the existing config with the new one.
// Should be called after successful Transition Stage.
func (s *State) FinalizeTransition(params Params, inputs Inputs) (Result, error) {
	return s.process(params, inputs, false)
}

// FinalizeUnmanagedTransition handles the transition away from managed configuration mode.
// If the registry secret is not present, the internal state is cleared and the transition is considered complete.
// If the secret is present – we preserve and support its configuration instead of using Deckhouse registry secret.
func (s *State) FinalizeUnmanagedTransition(registrySecret deckhouse_registry.Config, inputs Inputs) (Result, error) {
	if !inputs.IsSecretExist {
		*s = State{}
		return buildResult(inputs, true, registry_const.UnknownVersion), nil
	}

	modeParams := ModeParams{}
	if err := modeParams.fromRegistrySecret(registrySecret); err != nil {
		return failedResult, fmt.Errorf("failed to initialize params from secret: %w", err)
	}

	params := Params{
		RegistrySecret: registrySecret,
		ModeParams:     modeParams,
	}
	return s.process(params, inputs, false)
}

// process applies the Bashible configuration based on the given mode parameters
// and node input state. It operates in two stages:
//
// Transition Stage (isTransitionStage == true):
//   - Prepares a dual configuration: current (previous) + intended (new).
//   - Used when switching the registry mode (e.g., proxy → direct).
//   - Keeps current config active until transition succeeds.
//
// Final Stage (isTransitionStage == false):
//   - Applies the new configuration immediately, replacing the previous one.
//
// Returns:
//   - Result: status of config preparation (success or pending).
//   - error: any validation, loading, or build error that occurred.
func (s *State) process(params Params, inputs Inputs, isTransitionStage bool) (Result, error) {
	if params.ModeParams.isEmpty() {
		return failedResult, fmt.Errorf("mode params are empty")
	}

	// Transition stage + params already contained -> skip (stage already done)
	if isTransitionStage &&
		s.ActualParams != nil && s.ActualParams.isEqual(params.ModeParams) {
		return successResult, nil
	}

	// Transition stage + nodes have final version -> skip (transition stage not needed)
	// This check is required to handle cases when the cluster is already configured in its final state, for example:
	// * State was lost and restored — if the cluster is already in final state, there's no need to repeat the transition stage.
	// * Cluster bootstrap — the initial state already matches the final one, so the transition stage can be skipped.
	if isTransitionStage && s.Config.Version == "" {
		config, err := s.finalConfig(params, inputs)
		if err != nil {
			return failedResult, fmt.Errorf("failed to build config: %w", err)
		}
		isFinalVersionApplied := true
		for _, node := range inputs.NodeStatus {
			if config.Version != node.Version {
				isFinalVersionApplied = false
				break
			}
		}
		if isFinalVersionApplied {
			return successResult, nil
		}
	}

	if isTransitionStage {
		// Initialize actual params from secret if not set or empty
		if s.ActualParams == nil || s.ActualParams.isEmpty() {
			s.ActualParams = &ModeParams{}
			if err := s.ActualParams.fromRegistrySecret(params.RegistrySecret); err != nil {
				return failedResult, fmt.Errorf("cannot load actual params from registry secret: %w", err)
			}
		}

		// Build transition config using actual params
		config, err := s.transitionConfig(params, inputs, *s.ActualParams)
		if err != nil {
			return failedResult, fmt.Errorf("failed to build config: %w", err)
		}
		s.Config = config
	} else {
		// Build final config (no actual params needed at this point)
		config, err := s.finalConfig(params, inputs)
		if err != nil {
			return failedResult, fmt.Errorf("failed to build config: %w", err)
		}
		s.Config = config

		// Store current params:
		// - for potential future transition stage
		// - to check if the transition stage has already been applied
		s.ActualParams = &params.ModeParams
	}

	return buildResult(inputs, false, s.Config.Version), nil
}

func (s *State) transitionConfig(params Params, inputs Inputs, actualParams ModeParams) (*Config, error) {
	builder := ConfigBuilder{
		ModeParams:     params.ModeParams,
		MasterNodesIPs: inputs.MasterNodesIPs,
		ActualParams:   []ModeParams{actualParams},
	}
	return builder.build()
}

func (s *State) finalConfig(params Params, inputs Inputs) (*Config, error) {
	builder := ConfigBuilder{
		ModeParams:     params.ModeParams,
		MasterNodesIPs: inputs.MasterNodesIPs,
	}
	return builder.build()
}

func (b *ConfigBuilder) build() (*Config, error) {
	mode, err := b.ModeParams.mode()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve mode: %w", err)
	}

	imagesBase := registry_const.HostWithPath
	if b.ModeParams.Unmanaged != nil {
		imagesBase = b.ModeParams.Unmanaged.ImagesRepo
	}

	ret := Config{
		Mode:           mode,
		ImagesBase:     imagesBase,
		ProxyEndpoints: b.proxyEndpoints(),
		Hosts:          b.hosts(),
	}

	version, err := registry_pki.ComputeHash(&ret)
	if err != nil {
		return nil, fmt.Errorf("failed to compute config version: %w", err)
	}
	ret.Version = version
	return &ret, nil
}

func (b *ConfigBuilder) proxyEndpoints() []string {
	return []string{}
}

func (b *ConfigBuilder) hosts() map[string]bashible.ConfigHosts {
	ret := make(map[string]bashible.ConfigHosts)

	for _, params := range append(slices.Clone(b.ActualParams), b.ModeParams) {
		var (
			host    string
			mirrors []bashible.ConfigMirrorHost
		)

		switch {
		case params.Direct != nil:
			host, mirrors = params.Direct.hostMirrors()
		case params.Unmanaged != nil:
			host, mirrors = params.Unmanaged.hostMirrors()
		default:
			continue
		}

		existingHost := ret[host]
		for _, mirror := range mirrors {
			key := mirror.UniqueKey()
			found := false

			// Replace existing mirror with the same UniqueKey (if found)
			for i, existing := range existingHost.Mirrors {
				if existing.UniqueKey() == key {
					existingHost.Mirrors[i] = mirror
					found = true
					break
				}
			}
			if !found {
				existingHost.Mirrors = append(existingHost.Mirrors, mirror)
			}
		}
		ret[host] = existingHost
	}
	return ret
}

func (p *ModeParams) fromRegistrySecret(registrySecret deckhouse_registry.Config) error {
	username, password, err := registry_docker.CredsFromDockerCfg(
		registrySecret.DockerConfig,
		registrySecret.Address,
	)
	if err != nil {
		return fmt.Errorf("failed to extract credentials from Docker config: %w", err)
	}

	hostWithPath := registrySecret.Address + registrySecret.Path
	*p = ModeParams{
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

func (p *ModeParams) mode() (registry_const.ModeType, error) {
	switch {
	case p == nil:
		return "", fmt.Errorf("empty mode params")
	case p.Direct != nil:
		return registry_const.ModeDirect, nil
	case p.Unmanaged != nil:
		return registry_const.ModeUnmanaged, nil
	default:
		return "", fmt.Errorf("unknown mode")
	}
}

func (p *ModeParams) isEmpty() bool {
	if p == nil {
		return true
	}
	return p.Direct == nil &&
		p.Unmanaged == nil
}

func (p *ModeParams) isEqual(other ModeParams) bool {
	if p == nil {
		return false
	}
	if !p.Direct.isEqual(other.Direct) {
		return false
	}
	if !p.Unmanaged.isEqual(other.Unmanaged) {
		return false
	}
	return true
}

func (p *UnmanagedModeParams) isEqual(other *UnmanagedModeParams) bool {
	switch {
	case p == nil && other == nil:
		return true
	case p != nil && other != nil:
		return *p == *other
	}
	return false
}

func (p *DirectModeParams) isEqual(other *DirectModeParams) bool {
	switch {
	case p == nil && other == nil:
		return true
	case p != nil && other != nil:
		return *p == *other
	}
	return false
}

func (p *UnmanagedModeParams) hostMirrors() (string, []bashible.ConfigMirrorHost) {
	host, _ := helpers.RegistryAddressAndPathFromImagesRepo(p.ImagesRepo)
	return host, []bashible.ConfigMirrorHost{{
		Host:   host,
		CA:     p.CA,
		Scheme: strings.ToLower(p.Scheme),
		Auth: bashible.ConfigAuth{
			Username: p.Username,
			Password: p.Password,
		},
	}}
}

func (p *DirectModeParams) hostMirrors() (string, []bashible.ConfigMirrorHost) {
	host, path := helpers.RegistryAddressAndPathFromImagesRepo(p.ImagesRepo)
	return registry_const.Host, []bashible.ConfigMirrorHost{{
		Host:   host,
		CA:     p.CA,
		Scheme: strings.ToLower(p.Scheme),
		Auth: bashible.ConfigAuth{
			Username: p.Username,
			Password: p.Password,
		},
		Rewrites: []bashible.ConfigRewrite{{
			From: registry_const.PathRegexp,
			To:   strings.TrimLeft(path, "/"),
		}},
	}}
}

func buildResult(inputs Inputs, isStop bool, version string) Result {
	var msg strings.Builder
	fmt.Fprintln(&msg, transitionMessage)

	if isStop && inputs.IsSecretExist {
		fmt.Fprintln(&msg, "Cleaning Managed configuration...")
		return Result{Ready: false, Message: msg.String()}
	}
	if !isStop && !inputs.IsSecretExist {
		fmt.Fprintln(&msg, "Creating Managed configuration...")
		return Result{Ready: false, Message: msg.String()}
	}

	var pending []string
	for name, status := range inputs.NodeStatus {
		if status.Version != version {
			pending = append(pending, name)
		}
	}

	total := len(inputs.NodeStatus)
	ready := total - len(pending)

	if len(pending) == 0 {
		if isStop {
			fmt.Fprintf(&msg, "All %d node(s) use the Unmanaged config.\n", total)
		} else {
			fmt.Fprintf(&msg, "All %d node(s) updated to version %s.\n", total, helpers.TrimWithEllipsis(version))
		}
		return Result{Ready: true, Message: msg.String()}
	}

	fmt.Fprintf(&msg, "%d/%d node(s) ready. Waiting:\n", ready, total)

	slices.Sort(pending)
	const maxShown = 10
	for i, name := range pending {
		if i == maxShown {
			fmt.Fprintf(&msg, "\t...and %d more\n", len(pending)-maxShown)
			break
		}
		currentVersion := inputs.NodeStatus[name].Version
		if isStop {
			fmt.Fprintf(&msg, "- %s: %q → Unmanaged\n", name, helpers.TrimWithEllipsis(currentVersion))
		} else {
			fmt.Fprintf(&msg, "- %s: %q → %q\n", name, helpers.TrimWithEllipsis(currentVersion), helpers.TrimWithEllipsis(version))
		}
	}

	return Result{Ready: false, Message: msg.String()}
}
