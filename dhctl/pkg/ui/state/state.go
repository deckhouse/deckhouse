package state

import (
	"fmt"
)

type State struct {
	ClusterType string
	Provider    string
	Prefix      string
	K8sVersion  string
	schema      *Schema
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
