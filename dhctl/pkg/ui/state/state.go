package state

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

type StaticState struct {
	AskSudoPassword     bool   `json:"-"`
	InternalNetworkCIDR string `json:"internal_network_cidr"`
}

type RegistryState struct {
	Repo     string `json:"repo"`
	User     string `json:"user"`
	Password string `json:"password"`
	Schema   string `json:"schema"`
	CA       string `json:"ca"`
}

type ClusterState struct {
	K8sVersion           string `json:"k8s_version"`
	ClusterDomain        string `json:"cluster_domain"`
	PodSubnetCIDR        string `json:"pod_subnet_cidr"`
	ServiceSubnetCIDR    string `json:"service_subnet_cidr"`
	SubnetNodeCIDRPrefix string `json:"subnet_node_cidr_prefix"`
}

type CNIState struct {
	Type        string `json:"cni_type"`
	FlannelMode string `json:"flannel_mode"`
}

type DeckhouseState struct {
	ReleaseChannel       string `json:"release_channel"`
	PublicDomainTemplate string `json:"public_domain_template"`
	EnablePublishK8sAPI  bool   `json:"enable_publish_k8s_api"`
}

type SSHState struct {
	Public      string `json:"public"`
	Private     string `json:"-"`
	User        string `json:"-"`
	Host        string `json:"-"`
	BastionUser string `json:"-"`
	BastionHost string `json:"-"`
}

type State struct {
	ClusterType    string                 `json:"cluster_type"`
	Provider       string                 `json:"provider"`
	Prefix         string                 `json:"prefix"`
	ProviderData   map[string]interface{} `json:"provider_data"`
	StaticState    StaticState            `json:"static_state"`
	RegistryState  RegistryState          `json:"registry_state"`
	schema         *Schema                `json:"-"`
	ConfigYAML     string                 `json:"-"`
	ClusterState   ClusterState           `json:"cluster_state"`
	CNIState       CNIState               `json:"cni_state"`
	DeckhouseState DeckhouseState         `json:"deckhouse_state"`
	SSHState       SSHState               `json:"ssh_state"`
}

func NewState(s *Schema) *State {
	public, private, err := generateEd25519Keys()
	if err != nil {
		panic(err)
	}

	return &State{
		SSHState: SSHState{
			Public:      public,
			Private:     private,
			User:        "ubuntu",
			BastionUser: "ubuntu",
		},
		RegistryState: RegistryState{
			Repo: s.DefaultRegistryRepo(),
			User: s.DefaultRegistryUser(),
		},
		ClusterState: ClusterState{
			K8sVersion:           s.K8sVersions()[len(s.K8sVersions())-1],
			ClusterDomain:        "cluster.local",
			PodSubnetCIDR:        "10.111.0.0/16",
			ServiceSubnetCIDR:    "10.222.0.0/16",
			SubnetNodeCIDRPrefix: "24",
		},
		DeckhouseState: DeckhouseState{
			PublicDomainTemplate: "%s.example.com",
			EnablePublishK8sAPI:  true,
			ReleaseChannel:       s.ReleaseChannels()[len(s.ReleaseChannels())-2],
		},
		StaticState: StaticState{
			AskSudoPassword:     false,
			InternalNetworkCIDR: "",
		},
		schema: s,
	}
}

func (b *State) SetConfigYAML(data string) error {
	// validate config.yaml
	if _, err := config.ParseConfigFromData(data); err != nil {
		return err
	}

	b.ConfigYAML = data
	return nil
}

func (b *State) GetRegistryState() RegistryState {
	return b.RegistryState
}

func (b *State) ConvertToMap() (map[string]interface{}, error) {
	raw, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(raw, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (b *State) GetClusterType() string {
	return b.ClusterType
}

func (b *State) GetSSHUser() string {
	return b.SSHState.User
}

func (b *State) GetSSHHost() string {
	return b.SSHState.Host
}

func (b *State) GetInternalNetworkCIDR() string {
	return b.StaticState.InternalNetworkCIDR
}
func (b *State) IsUsePasswordForSudo() bool {
	return b.StaticState.AskSudoPassword
}

func (b *State) GetBastionSSHUser() string {
	return b.SSHState.BastionUser
}

func (b *State) GetBastionSSHHost() string {
	return b.SSHState.BastionHost
}

func (b *State) SetSSHUser(s string) error {
	if s != "" {
		b.SSHState.User = s
		return nil
	}

	return fmt.Errorf("SSH user cannot be empty")
}

func (b *State) SetSSHHost(s string) error {
	if s != "" {
		b.SSHState.Host = s
		return nil
	}

	return fmt.Errorf("SSH host cannot be empty")
}

func (b *State) SetInternalNetworkCIDR(s string) error {
	if err := validateCIDR(s); err != nil {
		return err
	}

	b.StaticState.InternalNetworkCIDR = s
	return nil
}

func (b *State) SetUsePasswordForSudo(b2 bool) {
	b.StaticState.AskSudoPassword = b2
}

func (b *State) SetBastionSSHUser(s string) {
	b.SSHState.BastionUser = s
}

func (b *State) SetBastionSSHHost(s string) {
	b.SSHState.BastionHost = s
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

func (b *State) GetClusterPrefix() string {
	return b.Prefix
}

func (b *State) GetRegistryRepo() string {
	return b.RegistryState.Repo
}

func (b *State) GetRegistryUser() string {
	return b.RegistryState.User
}

func (b *State) GetRegistryPassword() string {
	return b.RegistryState.Password
}

func (b *State) GetRegistrySchema() string {
	return b.RegistryState.Schema
}

func (b *State) GetRegistryCA() string {
	return b.RegistryState.CA
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

func (b *State) GetK8sVersion() string {
	return b.ClusterState.K8sVersion
}

func (b *State) GetClusterDomain() string {
	return b.ClusterState.ClusterDomain
}

func (b *State) GetPodSubnetCIDR() string {
	return b.ClusterState.PodSubnetCIDR
}

func (b *State) GetServiceSubnetCIDR() string {
	return b.ClusterState.ServiceSubnetCIDR
}

func (b *State) GetPodSubnetNodeCIDRPrefix() string {
	return b.ClusterState.SubnetNodeCIDRPrefix
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
	if err := validateCIDR(v); err != nil {
		return err
	}

	b.ClusterState.PodSubnetCIDR = v
	return nil
}
func (b *State) SetServiceSubnetCIDR(v string) error {
	if err := validateCIDR(v); err != nil {
		return err
	}

	b.ClusterState.ServiceSubnetCIDR = v
	return nil
}

func (b *State) SetPodSubnetNodeCIDRPrefix(v string) error {
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

func (b *State) GetCNIType() string {
	return b.CNIState.Type
}

func (b *State) GetFlannelMode() string {
	return b.CNIState.FlannelMode
}

func (b *State) SetFlannelMode(t string) error {
	for _, tt := range []string{FlannelVxLAN, FlannelHostGW} {
		if tt == t {
			b.CNIState.FlannelMode = t
			return nil
		}
	}

	return fmt.Errorf("Unknown CNI type %s", t)
}

func (b *State) SetCNIType(t string) error {
	for _, tt := range []string{CNICilium, CNISimpleBridge, CNIFlannel} {
		if tt == t {
			b.CNIState.Type = t
			return nil
		}
	}

	return fmt.Errorf("Unknown CNI type %s", t)
}

func (b *State) GetReleaseChannel() string {
	return b.DeckhouseState.ReleaseChannel
}
func (b *State) GetPublicDomainTemplate() string {
	return b.DeckhouseState.PublicDomainTemplate
}

func (b *State) IsEnablePublishK8sAPI() bool {
	return b.DeckhouseState.EnablePublishK8sAPI
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
	if err := b.schema.validatePublicDomainTemplate(p); err != nil {
		return fmt.Errorf("Public domain template invalid", err)
	}

	b.DeckhouseState.PublicDomainTemplate = p
	return nil
}

func (b *State) EnablePublishK8sAPI(f bool) {
	b.DeckhouseState.EnablePublishK8sAPI = f
}

func (b *State) GetUser() string {
	return b.SSHState.User
}

func (b *State) PublicSSHKey() string {
	return b.SSHState.Public
}

func (b *State) PrivateSSHKey() string {
	return b.SSHState.Private
}

func (b *State) GetSSHState() SSHState {
	return b.SSHState
}
