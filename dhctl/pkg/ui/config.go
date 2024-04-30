package ui

import (
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

const (
	cloudCluster  = config.CloudClusterType
	staticCluster = config.StaticClusterType
)

type configBuilder struct {
	clusterType string
}

func newConfigBuilder() *configBuilder {
	return &configBuilder{}
}

func (b *configBuilder) build() []string {
	return []string{
		b.clusterType,
	}
}

func (b *configBuilder) setClusterType(t string) {
	if t == cloudCluster || t == staticCluster {
		b.clusterType = t
		return
	}

	panic(fmt.Sprintf("unknown cluster type: %v", t))
}
