package probers

import (
	"upmeter/pkg/probe/types"
	"upmeter/pkg/probers/control-plane"
	"upmeter/pkg/probers/synthetic"
)

// Load creates instances of available Probers
func Load() []types.Prober {
	res := make([]types.Prober, 0)
	res = append(res, control_plane.LoadGroup()...)
	res = append(res, synthetic.LoadGroup()...)
	return res
}
