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
	StageProcessFirst  = "Process stage 1: apply new configs with existing ones"
	StageProcessSecond = "Process stage 2: apply new configs only, remove old if exist"
	StageCleanupFirst  = "Cleanup stage 1: apply Unmanaged configs with existing ones"
	StageCleanupSecond = "Cleanup stage 2: cleanup old configs and remove registry-bashible-config secret"
)

type Inputs struct {
	IsSecretExist  bool
	MasterNodesIPs []string
	NodeStatus     map[string]InputsNodeVersion
}

type InputsNodeVersion = string

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
	Mode registry_const.ModeType `json:"mode" yaml:"mode"`

	ProxyLocal *ProxyLocalModeParams `json:"proxy_local,omitempty" yaml:"proxy_local,omitempty"`
	Unmanaged  *UnmanagedModeParams  `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
	Direct     *DirectModeParams     `json:"direct,omitempty" yaml:"direct,omitempty"`
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

// ProcessFirstStage applies the new configuration alongside the existing one.
// Should be used when registry mode or its parameters change (transition phase).
func (state *State) ProcessFirstStage(params Params, inputs Inputs) (Result, error) {
	return state.process(params, inputs, true, StageProcessFirst)
}

// ProcessSecondStage replaces the existing config with the new one.
// Should be called after successful first stage.
func (state *State) ProcessSecondStage(params Params, inputs Inputs) (Result, error) {
	return state.process(params, inputs, false, StageProcessSecond)
}

// CleanupFirstStage applies the unmanaged configuration in combination with the existing one.
// Useful to transition from managed to unmanaged mode.
func (state *State) CleanupFirstStage(registrySecret deckhouse_registry.Config, inputs Inputs) (UnmanagedModeParams, Result, error) {
	if state.UnmanagedParams == nil {
		return UnmanagedModeParams{}, failedResult(StageCleanupFirst), fmt.Errorf("unmanaged parameters are not initialized")
	}

	params := Params{
		RegistrySecret: registrySecret,
		ModeParams: ModeParams{
			Mode:      registry_const.ModeUnmanaged,
			Unmanaged: state.UnmanagedParams,
		},
	}

	result, err := state.process(params, inputs, true, StageCleanupFirst)
	return *state.UnmanagedParams, result, err
}

// CleanupSecondStage resets the internal state and removes any configuration secrets.
// It is the final cleanup stage when switching away from Managed configurations.
func (state *State) CleanupSecondStage(inputs Inputs) Result {
	*state = State{}
	return buildResult(inputs, true, registry_const.UnknownVersion, StageCleanupSecond)
}

// IsRunning returns true if there is an active configuration managed by this state.
func (state *State) IsRunning() bool {
	return state != nil && state.Config != nil
}

// process applies the Bashible configuration, based on the provided
// mode parameters and node input state. It operates in two possible stages:
//
// First Stage (isFirstStage == true):
//   - Used when switching the registry mode (e.g., from proxy to direct).
//   - Supports dual-configuration:
//     1. The current configuration (loaded from state or secret).
//     2. The new configuration (provided via params).
//   - Ensures safe transition by keeping current config in place while preparing the new one.
//   - After successful execution, the system is ready to continue to the second stage.
//
// Second Stage (isFirstStage == false):
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
func (state *State) process(params Params, inputs Inputs, isFirstStage bool, stageInfo string) (Result, error) {
	switch {
	case params.ModeParams.Mode == registry_const.ModeUnmanaged &&
		params.ModeParams.Unmanaged == nil:
		return failedResult(stageInfo), fmt.Errorf("missing unmanaged parameters for mode %s", params.ModeParams.Mode)
	case (params.ModeParams.Mode == registry_const.ModeLocal ||
		params.ModeParams.Mode == registry_const.ModeProxy) &&
		params.ModeParams.ProxyLocal == nil:
		return failedResult(stageInfo), fmt.Errorf("missing proxy/local parameters for mode %s", params.ModeParams.Mode)
	case params.ModeParams.Mode == registry_const.ModeDirect &&
		params.ModeParams.Direct == nil:
		return failedResult(stageInfo), fmt.Errorf("missing direct parameters for mode %s", params.ModeParams.Mode)
	}

	imagesBase := registry_const.HostWithPath
	if params.ModeParams.Mode == registry_const.ModeUnmanaged {
		imagesBase = params.ModeParams.Unmanaged.ImagesRepo
	}

	// First stage + params already contained -> skip (stage already done)
	if isFirstStage &&
		state.ActualParams != nil &&
		state.ActualParams.isEqual(params.ModeParams) {
		return successResult(stageInfo), nil
	}

	// Init actual params from secret, if empty
	if state.ActualParams == nil ||
		state.ActualParams.isEmpty() {
		state.ActualParams = &ModeParams{}
		if err := state.ActualParams.fromRegistrySecret(params.RegistrySecret); err != nil {
			return failedResult(stageInfo), fmt.Errorf("failed to initialize actual params from secret: %w", err)
		}
	}

	config := Config{
		Mode:           params.ModeParams.Mode,
		ImagesBase:     imagesBase,
		Version:        "",                          // by processHash
		ProxyEndpoints: []string{},                  // by processEndpoints
		Hosts:          map[string]bashible.Hosts{}, // by processHosts
		PrepullHosts:   map[string]bashible.Hosts{}, // by processHosts
	}
	if isFirstStage {
		// Current
		config.processHosts(inputs.MasterNodesIPs, *state.ActualParams)
		// New
		config.processHosts(inputs.MasterNodesIPs, params.ModeParams)
		// Endpoints
		config.processEndpoints(inputs.MasterNodesIPs, state.ActualParams.Mode, params.ModeParams.Mode)
	} else {
		// Replace Current by new and process only new hosts
		state.ActualParams = &params.ModeParams
		config.processHosts(inputs.MasterNodesIPs, *state.ActualParams)
		// Endpoints
		config.processEndpoints(inputs.MasterNodesIPs, state.ActualParams.Mode)
	}

	if err := config.processHash(); err != nil {
		return failedResult(stageInfo), err
	}
	state.Config = &config
	return buildResult(inputs, false, state.Config.Version, stageInfo), nil
}

func (cfg *Config) processEndpoints(masterNodesIPs []string, modes ...registry_const.ModeType) {
	cfg.ProxyEndpoints = []string{}

	for _, mode := range modes {
		if mode == registry_const.ModeLocal || mode == registry_const.ModeProxy {
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

func (cfg *Config) processHosts(masterNodesIPs []string, modeParams ModeParams) {
	switch {
	case modeParams.ProxyLocal != nil:
		hosts, prepull := processProxyLocal(*modeParams.ProxyLocal, masterNodesIPs)
		cfg.mergeHosts(hosts, prepull)
	case modeParams.Unmanaged != nil:
		h := processUnmanaged(*modeParams.Unmanaged)
		cfg.mergeHosts(h, h)
	case modeParams.Direct != nil:
		h := processDirect(*modeParams.Direct)
		cfg.mergeHosts(h, h)
	}
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
		Mode: registry_const.ModeUnmanaged,
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

func (p *ModeParams) isEmpty() bool {
	return p == nil || p.Mode == ""
}

func (p *ModeParams) isEqual(other ModeParams) bool {
	if p == nil {
		return false
	}
	if p.Mode != other.Mode {
		return false
	}
	if !p.ProxyLocal.isEqual(other.ProxyLocal) {
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

func processProxyLocal(params ProxyLocalModeParams, masterNodesIPs []string) (map[string]bashible.Hosts, map[string]bashible.Hosts) {
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

	ca := singleCA(params.CA)
	hosts := map[string]bashible.Hosts{
		registry_const.Host: {
			CA:      ca,
			Mirrors: makeMirrors([]string{registry_const.ProxyHost}),
		},
	}
	prepullHosts := map[string]bashible.Hosts{
		registry_const.Host: {
			CA: ca,
			Mirrors: makeMirrors(append([]string{registry_const.ProxyHost},
				registry_const.GenerateProxyEndpoints(masterNodesIPs)...)),
		},
	}
	return hosts, prepullHosts
}

func failedResult(stageInfo string) Result {
	return Result{
		Ready:   false,
		Message: fmt.Sprintf("%s\nFailed to process Bashible configuration.", stageInfo),
	}
}

func successResult(stageInfo string) Result {
	return Result{
		Ready:   true,
		Message: fmt.Sprintf("%s\nBashible already processed.", stageInfo),
	}
}

func buildResult(inputs Inputs, isStop bool, version, stageInfo string) Result {
	builder := new(strings.Builder)
	fmt.Fprint(builder, stageInfo+"\n")

	if isStop && inputs.IsSecretExist {
		fmt.Fprint(builder, "Bashible Secret exists. Deleting now...")
		return Result{
			Ready:   false,
			Message: builder.String(),
		}
	}
	if !isStop && !inputs.IsSecretExist {
		fmt.Fprint(builder, "Creating Bashible Secret...")
		return Result{
			Ready:   false,
			Message: builder.String(),
		}
	}

	unreadyNodes := []string{}
	for nodeName, nodeVersion := range inputs.NodeStatus {
		if nodeVersion != version {
			unreadyNodes = append(unreadyNodes, nodeName)
		}
	}

	if len(unreadyNodes) == 0 {
		if isStop {
			fmt.Fprintf(builder, "All %d node(s) have been updated with Unmanaged configuration.",
				len(inputs.NodeStatus),
			)
		} else {
			fmt.Fprintf(builder, "All %d node(s) have been updated to registry version: %s.",
				len(inputs.NodeStatus), trimWithEllipsis(version),
			)
		}
		return Result{
			Ready:   true,
			Message: builder.String(),
		}
	}

	slices.Sort(unreadyNodes)

	if isStop {
		fmt.Fprintf(builder, "%d/%d node(s) are ready with Unmanaged configuration.\nWaiting for the following node(s):\n",
			len(inputs.NodeStatus)-len(unreadyNodes), len(inputs.NodeStatus),
		)
	} else {
		fmt.Fprintf(builder, "%d/%d node(s) have been updated to registry version \"%s\".\nWaiting for the following node(s):\n",
			len(inputs.NodeStatus)-len(unreadyNodes), len(inputs.NodeStatus), trimWithEllipsis(version),
		)
	}

	const maxShown = 10
	for i, name := range unreadyNodes {
		if i == maxShown {
			fmt.Fprintf(builder, "\t...and %d more\n", len(unreadyNodes)-maxShown)
			break
		}
		version := inputs.NodeStatus[name]
		fmt.Fprintf(builder, "\t%d. %q (currently running version \"%s\")\n", i+1, name, trimWithEllipsis(version))
	}

	return Result{
		Ready:   false,
		Message: builder.String(),
	}
}

func singleCA(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func trimWithEllipsis(s string) string {
	const limit = 15
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(slices.Clone(runes[:limit])) + "…"
}
