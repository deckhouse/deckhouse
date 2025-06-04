/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	bashible "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/bashible"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
)

const (
	transitionMessage     = "Applying configuration to nodes"
	preflightCheckMessage = "Check containerd configuration"
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

type Config bashible.Config

type ModeParams struct {
	Proxy     *ProxyLocalModeParams `json:"proxy,omitempty" yaml:"proxy,omitempty"`
	Local     *ProxyLocalModeParams `json:"local,omitempty" yaml:"local,omitempty"`
	Direct    *DirectModeParams     `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *UnmanagedModeParams  `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
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

type Result struct {
	Ready   bool
	Message string
}

type bashibleHosts map[string]bashible.Hosts

func PreflightCheck(input Inputs) Result {
	var msg strings.Builder
	fmt.Fprintln(&msg, preflightCheckMessage)

	if input.IsSecretExist {
		fmt.Fprintln(&msg, "Bashible secret already exists.")
		return Result{Ready: true, Message: msg.String()}
	}

	var pending []string
	for nodeName, status := range input.NodeStatus {
		if status.ContainerdCfgMode != containerdCfgModeDefault {
			pending = append(pending, nodeName)
		}
	}

	total := len(input.NodeStatus)
	ready := total - len(pending)

	if len(pending) == 0 {
		fmt.Fprintf(&msg, "All %d node(s) have default containerd configuration.\n", total)
		return Result{Ready: true, Message: msg.String()}
	}

	fmt.Fprintf(&msg, "%d/%d node(s) ready. Waiting:\n", ready, total)

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
			fmt.Fprintf(&msg, "- %s: has custom containerd configuration\n", name)
		default:
			fmt.Fprintf(&msg, "- %s: unknown containerd configuration\n", name)
		}
	}
	return Result{Ready: false, Message: msg.String()}
}

// ProcessTransition applies the new configuration alongside the existing one.
// Should be used when registry mode or its parameters change (transition phase).
func (state *State) ProcessTransition(params Params, inputs Inputs) (Result, error) {
	return state.process(params, inputs, true)
}

// FinalizeTransition replaces the existing config with the new one.
// Should be called after successful first stage.
func (state *State) FinalizeTransition(params Params, inputs Inputs) (Result, error) {
	return state.process(params, inputs, false)
}

// ProcessUnmanagedTransition applies the unmanaged configuration in combination with the existing one.
// Useful to transition from managed to unmanaged mode.
func (state *State) ProcessUnmanagedTransition(registrySecret deckhouse_registry.Config, inputs Inputs) (UnmanagedModeParams, Result, error) {
	if state.UnmanagedParams == nil {
		return UnmanagedModeParams{}, failedResult, fmt.Errorf("unmanaged parameters are not initialized")
	}

	params := Params{
		RegistrySecret: registrySecret,
		ModeParams: ModeParams{
			Unmanaged: state.UnmanagedParams,
		},
	}

	result, err := state.process(params, inputs, true)
	return *state.UnmanagedParams, result, err
}

// FinalizeUnmanagedTransition resets the internal state and removes any configuration secrets.
// It is the final cleanup stage when switching away from Managed configurations.
func (state *State) FinalizeUnmanagedTransition(inputs Inputs) Result {
	*state = State{}
	return buildResult(inputs, true, registry_const.UnknownVersion)
}

// IsRunning returns true if there is an active configuration managed by this state.
func (state *State) IsRunning() bool {
	return state != nil && state.Config != nil
}

// process applies the Bashible configuration, based on the provided
// mode parameters and node input state. It operates in two possible stages:
//
// Transition Stage (isFirstStage == true):
//   - Used when switching the registry mode (e.g., from proxy to direct).
//   - Supports dual-configuration:
//     1. The current configuration (loaded from state or secret).
//     2. The new configuration (provided via params).
//   - Ensures safe transition by keeping current config in place while preparing the new one.
//   - After successful execution, the system is ready to continue to the second stage.
//
// Final Stage (isFirstStage == false):
//   - Used after the first stage has completed successfully.
//   - Replaces the current configuration with the new one entirely.
//
// Usage Rules:
//   - When the registry mode or mode-specific parameters change:
//     ==> always run First Stage.
//   - Else:
//     ==> run Second Stage.
//
// Returns:
//   - Result: status of configuration (ready or not, message for logging/display).
//   - error: encountered if parameter validation or processing fails.
func (state *State) process(params Params, inputs Inputs, isTransitionStage bool) (Result, error) {
	if params.ModeParams.isEmpty() {
		return failedResult, fmt.Errorf("mode params are empty")
	}

	mode, err := params.ModeParams.mode()
	if err != nil {
		return failedResult, fmt.Errorf("failed to resolve mode: %w", err)
	}

	imagesBase := registry_const.HostWithPath
	if params.ModeParams.Unmanaged != nil {
		imagesBase = params.ModeParams.Unmanaged.ImagesRepo
	}

	// First stage + params already contained -> skip (stage already done)
	if isTransitionStage &&
		state.ActualParams != nil &&
		state.ActualParams.isEqual(params.ModeParams) {
		return successResult, nil
	}

	// Init actual params from secret, if empty
	if state.ActualParams == nil ||
		state.ActualParams.isEmpty() {
		state.ActualParams = &ModeParams{}
		if err := state.ActualParams.fromRegistrySecret(params.RegistrySecret); err != nil {
			return failedResult, fmt.Errorf("failed to initialize actual params from secret: %w", err)
		}
	}

	config := Config{
		Mode:           mode,
		ImagesBase:     imagesBase,
		Version:        "",                          // by processHash
		ProxyEndpoints: []string{},                  // by processEndpoints
		Hosts:          map[string]bashible.Hosts{}, // by processHosts
	}
	if isTransitionStage {
		// Current
		config.processHosts(*state.ActualParams)
		// New
		config.processHosts(params.ModeParams)
		// Endpoints
		config.processEndpoints(inputs.MasterNodesIPs, *state.ActualParams, params.ModeParams)
	} else {
		// Replace Current by new and process only new hosts
		state.ActualParams = &params.ModeParams
		config.processHosts(*state.ActualParams)
		// Endpoints
		config.processEndpoints(inputs.MasterNodesIPs, *state.ActualParams)
	}

	if err := config.processHash(); err != nil {
		return failedResult, err
	}
	state.Config = &config
	return buildResult(inputs, false, state.Config.Version), nil
}

func (cfg *Config) processEndpoints(masterNodesIPs []string, params ...ModeParams) {
	cfg.ProxyEndpoints = []string{}

	for _, p := range params {
		if p.Proxy != nil || p.Local != nil {
			cfg.ProxyEndpoints = registry_const.GenerateProxyEndpoints(masterNodesIPs)
			break
		}
	}
}

func (cfg *Config) processHash() error {
	cfg.Version = ""
	hash, err := helpers.ComputeHash(cfg)
	if err != nil {
		return fmt.Errorf("failed to compute config hash: %w", err)
	}
	cfg.Version = hash
	return nil
}

func (cfg *Config) processHosts(modeParams ModeParams) {
	switch {
	case modeParams.Proxy != nil:
		h := processProxyLocal(*modeParams.Proxy)
		cfg.mergeHosts(h)
	case modeParams.Local != nil:
		h := processProxyLocal(*modeParams.Local)
		cfg.mergeHosts(h)
	case modeParams.Direct != nil:
		h := processDirect(*modeParams.Direct)
		cfg.mergeHosts(h)
	case modeParams.Unmanaged != nil:
		h := processUnmanaged(*modeParams.Unmanaged)
		cfg.mergeHosts(h)
	}
}

func (cfg *Config) mergeHosts(hosts bashibleHosts) {
	if cfg.Hosts == nil {
		cfg.Hosts = make(bashibleHosts)
	}
	for name, h := range hosts {
		old := cfg.Hosts[name]
		old.Mirrors = deduplicateMirrors(append(old.Mirrors, h.Mirrors...))
		cfg.Hosts[name] = old
	}
}

func (p *ModeParams) fromRegistrySecret(registrySecret deckhouse_registry.Config) error {
	username, password, err := helpers.CredsFromDockerCfg(
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
	case p.Proxy != nil:
		return registry_const.ModeProxy, nil
	case p.Local != nil:
		return registry_const.ModeLocal, nil
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
	return p.Proxy == nil &&
		p.Local == nil &&
		p.Direct == nil &&
		p.Unmanaged == nil
}

func (p *ModeParams) isEqual(other ModeParams) bool {
	if p == nil {
		return false
	}
	if !p.Proxy.isEqual(other.Proxy) {
		return false
	}
	if !p.Local.isEqual(other.Local) {
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

func (p *ProxyLocalModeParams) isEqual(other *ProxyLocalModeParams) bool {
	switch {
	case p == nil && other == nil:
		return true
	case p != nil && other != nil:
		return *p == *other
	}
	return false
}

func processUnmanaged(params UnmanagedModeParams) bashibleHosts {
	host, _ := getRegistryAddressAndPathFromImagesRepo(params.ImagesRepo)
	return bashibleHosts{
		host: {
			Mirrors: []bashible.MirrorHost{{
				Host:   host,
				CA:     params.CA,
				Scheme: strings.ToLower(params.Scheme),
				Auth: bashible.Auth{
					Username: params.Username,
					Password: params.Password,
				},
			}},
		},
	}
}

func processDirect(params DirectModeParams) bashibleHosts {
	host, path := getRegistryAddressAndPathFromImagesRepo(params.ImagesRepo)
	return bashibleHosts{
		registry_const.Host: {
			Mirrors: []bashible.MirrorHost{{
				Host:   host,
				CA:     params.CA,
				Scheme: strings.ToLower(params.Scheme),
				Auth: bashible.Auth{
					Username: params.Username,
					Password: params.Password,
				},
				Rewrites: []bashible.Rewrite{{
					From: registry_const.PathRegexp,
					To:   strings.TrimLeft(path, "/"),
				}},
			}},
		},
	}
}

func processProxyLocal(params ProxyLocalModeParams) bashibleHosts {
	return bashibleHosts{
		registry_const.Host: {
			Mirrors: []bashible.MirrorHost{{
				Host:   registry_const.ProxyHost,
				CA:     params.CA,
				Scheme: registry_const.Scheme,
				Auth: bashible.Auth{
					Username: params.Username,
					Password: params.Password,
				},
			}},
		},
	}
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
			fmt.Fprintf(&msg, "All %d node(s) updated to version %s.\n", total, trimWithEllipsis(version))
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
			fmt.Fprintf(&msg, "- %s: %q → Unmanaged\n", name, trimWithEllipsis(currentVersion))
		} else {
			fmt.Fprintf(&msg, "- %s: %q → %q\n", name, trimWithEllipsis(currentVersion), trimWithEllipsis(version))
		}
	}

	return Result{Ready: false, Message: msg.String()}
}

func deduplicateMirrors(values []bashible.MirrorHost) []bashible.MirrorHost {
	ret := []bashible.MirrorHost{}

	for _, newMirror := range values {
		duplicate := false
		for _, existingMirror := range ret {
			if existingMirror.IsEqual(newMirror) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			ret = append(ret, newMirror)
		}
	}
	return ret
}

func trimWithEllipsis(value string) string {
	const limit = 15
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(slices.Clone(runes[:limit])) + "…"
}
