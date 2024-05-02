package state

import (
	"fmt"
)

type StaticState struct {
	SSHHost         string
	SSHUser         string
	AskSudoPassword bool

	InternalNetworkCIDR string

	BastionHost string
	BastionUser string
}

type State struct {
	ClusterType  string
	Provider     string
	Prefix       string
	K8sVersion   string
	ProviderData map[string]interface{}
	StaticState  StaticState
	schema       *Schema
}

func (b *State) SetSSHUser(s string) {
	b.StaticState.SSHUser = s
}

func (b *State) SetSSHHost(s string) {
	b.StaticState.SSHHost = s
}

func (b *State) SetInternalNetworkCIDR(s string) {
	b.StaticState.InternalNetworkCIDR = s
}

func (b *State) SetUsePasswordForSudo(b2 bool) {
	b.StaticState.AskSudoPassword = b2
}

func (b *State) SetBastionSSHUser(s string) {
	b.StaticState.BastionUser = s
}

func (b *State) SetBastionSSHHost(s string) {
	b.StaticState.BastionHost = s
}

func NewState(s *Schema) *State {
	return &State{
		schema: s,
	}
}

func (b *State) build() []string {
	return []string{
		b.ClusterType,
	}
}

func (b *State) SetClusterType(t string) {
	if t == CloudCluster || t == StaticCluster {
		b.ClusterType = t
		return
	}

	panic(fmt.Sprintf("unknown cluster type: %v", t))
}

func (b *State) SetProvider(p string) {
	b.Provider = p
}

func (b *State) SetClusterPrefix(p string) {
	b.Prefix = p
}

func (b *State) SetK8sVersion(v string) {
	b.K8sVersion = v
}

func (b *State) SetProviderData(d map[string]interface{}) {
	b.ProviderData = d
}

func (b *State) GetProvider() string {
	return b.Provider
}
