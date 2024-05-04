package state

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/dhctl/pkg/ui/validate"
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

type ClusterState struct {
	K8sVersion           string
	ClusterDomain        string
	PodSubnetCIDR        string
	ServiceSubnetCIDR    string
	SubnetNodeCIDRPrefix string
}

type CNIState struct {
	Type        string
	FlannelMode string
}

type DeckhouseState struct {
	ReleaseChannel       string
	PublicDomainTemplate string
	EnablePublishK8sAPI  bool
}

type State struct {
	ClusterType    string
	Provider       string
	Prefix         string
	ProviderData   map[string]interface{}
	StaticState    StaticState
	RegistryState  RegistryState
	schema         *Schema
	ClusterState   ClusterState
	CNIState       CNIState
	DeckhouseState DeckhouseState
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

func (b *State) SetSSHUser(s string) error {
	if s != "" {
		b.StaticState.SSHUser = s
		return nil
	}

	return fmt.Errorf("SSH user cannot be empty")
}

func (b *State) SetSSHHost(s string) error {
	if s != "" {
		b.StaticState.SSHHost = s
		return nil
	}

	return fmt.Errorf("SSH host cannot be empty")
}

func (b *State) SetInternalNetworkCIDR(s string) error {
	if err := validate.CIDR(s); err != nil {
		return err
	}

	b.StaticState.InternalNetworkCIDR = s
	return nil
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

func (b *State) SetRegistrySchema(s string) error {
	if s == RegistryHTTPS || s == RegistryHTTP {
		b.RegistryState.Schema = s
		return nil
	}

	return fmt.Errorf("unknown registry schema: %v", s)
}

func (b *State) SetRegistryCA(c string) {
	b.RegistryState.CA = c
}

func (b *State) SetK8sVersion(v string) error {
	found := false
	for _, vv := range b.schema.K8sVersions() {
		if vv == v {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("Incorrect k8s version")
	}

	b.ClusterState.K8sVersion = v

	return nil
}

func (b *State) SetClusterDomain(v string) error {
	if v == "" {
		return fmt.Errorf("Cluster domain should not be empty")
	}

	b.ClusterState.ClusterDomain = v
	return nil
}

func (b *State) SetPodSubnetCIDR(v string) error {
	if err := validate.CIDR(v); err != nil {
		return err
	}

	b.ClusterState.PodSubnetCIDR = v
	return nil
}
func (b *State) SetServiceSubnetCIDR(v string) error {
	if err := validate.CIDR(v); err != nil {
		return err
	}

	b.ClusterState.ServiceSubnetCIDR = v
	return nil
}

func (b *State) PodSubnetNodeCIDRPrefix(v string) error {
	suf, err := strconv.Atoi(v)
	if err != nil {
		return errors.Wrap(err, "Pod node suffix should be an integer")
	}

	if suf > 0 && suf <= 32 {
		b.ClusterState.SubnetNodeCIDRPrefix = v
		return nil
	}

	return fmt.Errorf("Pod node suffix should be > 0 and <= 32")
}

func (b *State) SetFlannelMode(t string) error {
	for _, tt := range []string{FlannelVxLAN, FlannelHostGW} {
		if tt != t {
			b.CNIState.FlannelMode = t
			return nil
		}
	}

	return fmt.Errorf("Unknown CNI type %s", t)
}

func (b *State) SetCNIType(t string) error {
	for _, tt := range []string{CNICilium, CNISimpleBridge, CNIFlannel} {
		if tt != t {
			b.CNIState.Type = t
			return nil
		}
	}

	return fmt.Errorf("Unknown CNI type %s", t)
}

func (b *State) SetReleaseChannel(ch string) error {
	channels := b.schema.ReleaseChannels()
	for _, c := range channels {
		if c == ch {
			b.DeckhouseState.ReleaseChannel = ch
			return nil
		}
	}

	return fmt.Errorf("Unknown release channel %s", ch)
}

func (b *State) SetPublicDomainTemplate(p string) error {
	if err := b.schema.ValidatePublicDomainTemplate(p); err != nil {
		return fmt.Errorf("Public domain template invalid", err)
	}

	b.DeckhouseState.PublicDomainTemplate = p
	return nil
}

func (b *State) EnablePublishK8sAPI(f bool) {
	b.DeckhouseState.EnablePublishK8sAPI = f
}
