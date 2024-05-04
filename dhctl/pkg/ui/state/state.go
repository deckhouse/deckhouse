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

type RegistryState struct {
	Repo     string
	User     string
	Password string
	Schema   string
	CA       string
}

type State struct {
	ClusterType   string
	Provider      string
	Prefix        string
	K8sVersion    string
	ProviderData  map[string]interface{}
	StaticState   StaticState
	RegistryState RegistryState
	schema        *Schema
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

func (b *State) SetRegistryRepo(r string) error {
	err := b.schema.ValidateImagesRepo(r)
	if err != nil {
		return err
	}

	b.RegistryState.Repo = r
	return nil
}

func (b *State) SetRegistryUser(u string) {
	b.RegistryState.User = u
}

func (b *State) SetRegistryPassword(p string) {
	b.RegistryState.Password = p
}

func (b *State) SetRegistrySchema(s string) {
	if s == RegistryHTTPS || s == RegistryHTTP {
		b.RegistryState.Schema = s
		return
	}

	panic(fmt.Sprintf("unknown registry schema: %v", s))
}

func (b *State) SetRegistryCA(c string) {
	b.RegistryState.CA = c
}
